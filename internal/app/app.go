package app

import (
	"io"
	"path/filepath"

	"github.com/kindbrave/claude-knowledger/internal/adapters/cli"
	mcpadapter "github.com/kindbrave/claude-knowledger/internal/adapters/mcp"
	"github.com/kindbrave/claude-knowledger/internal/backends/sqlite"
	"github.com/kindbrave/claude-knowledger/internal/backends/text"
	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/indexing/chroma"
	"github.com/kindbrave/claude-knowledger/internal/indexing/chunking"
	"github.com/kindbrave/claude-knowledger/internal/indexing/semantic"
	"github.com/kindbrave/claude-knowledger/internal/install/claude"
	"github.com/kindbrave/claude-knowledger/internal/install/opencode"
	"github.com/kindbrave/claude-knowledger/internal/projectroot"
	"github.com/kindbrave/claude-knowledger/internal/registry"
	"github.com/kindbrave/claude-knowledger/internal/service"
	"github.com/kindbrave/claude-knowledger/internal/spec"
	kbprovider "github.com/kindbrave/claude-knowledger/internal/spec/kb"
	extprovider "github.com/kindbrave/claude-knowledger/internal/spec/external"
	scriptprovider "github.com/kindbrave/claude-knowledger/internal/spec/script"
	"github.com/kindbrave/claude-knowledger/internal/specregistry"
)

// Version is set at build time via -ldflags "-X github.com/kindbrave/claude-knowledger/internal/app.Version=..."
var Version = "dev"

type MCPRunner func(*service.Service) error

type ClaudeInstallRunner func(out, errOut io.Writer) error
type OpenCodeInstallRunner func(out, errOut io.Writer) error

var runMCPServer MCPRunner = func(svc *service.Service) error {
	return mcpadapter.NewServer(svc).ServeStdio()
}

var runClaudeInstall ClaudeInstallRunner = func(out, errOut io.Writer) error {
	return claude.NewInstaller().Install(out, errOut)
}

var runOpenCodeInstall OpenCodeInstallRunner = func(out, errOut io.Writer) error {
	return opencode.NewInstaller().Install(out, errOut)
}

func SetMCPRunnerForTest(runner MCPRunner) func() {
	previous := runMCPServer
	runMCPServer = runner
	return func() { runMCPServer = previous }
}

func SetClaudeInstallRunnerForTest(runner ClaudeInstallRunner) func() {
	previous := runClaudeInstall
	runClaudeInstall = runner
	return func() { runClaudeInstall = previous }
}

func SetOpenCodeInstallRunnerForTest(runner OpenCodeInstallRunner) func() {
	previous := runOpenCodeInstall
	runOpenCodeInstall = runner
	return func() { runOpenCodeInstall = previous }
}

func Run(configPath string, args []string) error {
	if isNoConfigCommand(args) {
		return runService(nil, config.DefaultServerAddress, args)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	projectRoot, _, _ := projectroot.Discover()
	return RunWithConfig(cfg, projectRoot, args)
}

func RunDefault(args []string) error {
	if isNoConfigCommand(args) {
		return runService(nil, config.DefaultServerAddress, args)
	}
	cfg, err := config.Default()
	if err != nil {
		return err
	}
	projectRoot, _, _ := projectroot.Discover()
	return RunWithConfig(cfg, projectRoot, args)
}

func RunWithConfig(cfg config.Config, projectRoot string, args []string) error {
	if isNoConfigCommand(args) {
		return runService(nil, config.DefaultServerAddress, args)
	}
	if err := config.ApplyDefaults(&cfg); err != nil {
		return err
	}
	svc, err := BuildServiceFromConfig(cfg, projectRoot)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	return runService(svc, cfg.Server.Address, args)
}

func BuildService(configPath, projectRoot string) (*service.Service, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	return BuildServiceFromConfig(cfg, projectRoot)
}

func BuildDefaultService(projectRoot string) (*service.Service, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	return BuildServiceFromConfig(cfg, projectRoot)
}

func BuildServiceFromConfig(cfg config.Config, projectRoot string) (*service.Service, error) {
	if err := config.ApplyDefaults(&cfg); err != nil {
		return nil, err
	}
	globalStore := registry.NewFileStore(cfg.RuntimeRegistryPath)
	var projectStore registry.Store
	if projectRoot != "" {
		projectStore = registry.NewFileStore(filepath.Join(projectRoot, projectroot.MarkerDirName, "registry.json"))
	}
	r := registry.New(cfg.KnowledgeBases, globalStore, projectStore, projectRoot)
	svc, err := service.NewManaged(r, buildBackends)
	if err != nil {
		return nil, err
	}
	globalSpecStore := specregistry.NewFileStore(cfg.SpecRegistryPath)
	var projectSpecStore specregistry.Store
	if projectRoot != "" {
		projectSpecStore = specregistry.NewFileStore(filepath.Join(projectRoot, projectroot.MarkerDirName, "specs.json"))
	}
	specReg := specregistry.New(cfg.Specifications, globalSpecStore, projectSpecStore, projectRoot)
	svc.SetSpecRegistry(specReg)
	scope := core.ScopeGlobal
	if projectRoot != "" {
		scope = core.ScopeProject
	}
	svc.SetSpecEngineBuilder(func(specs []config.SpecificationConfig) service.SpecEngine {
		return buildSpecEngine(specs, svc, scope)
	})
	mergedSpecs, err := specReg.List()
	if err != nil {
		return nil, err
	}
	if len(mergedSpecs) > 0 {
		svc.SetSpecs(mergedSpecs)
		svc.SetSpecEngine(buildSpecEngine(mergedSpecs, svc, scope))
	}
	return svc, nil
}

func buildSpecEngine(specs []config.SpecificationConfig, svc *service.Service, scope string) *spec.Engine {
	engine := spec.NewEngine(specs)
	for _, s := range specs {
		if !s.Enabled {
			continue
		}
		switch s.Type {
		case "kb":
			engine.RegisterProvider(s.ID, kbprovider.New(svc, s.Source, scope))
		case "external":
			engine.RegisterProvider(s.ID, extprovider.New(s.Source))
		case "script":
			engine.RegisterProvider(s.ID, scriptprovider.New(s.Source))
		}
	}
	return engine
}

func buildBackends(kbs []core.KnowledgeBase) (map[string]core.StoreBackend, error) {
	indexer := semantic.NewIndexer(chroma.NewClient, chunking.Default())
	// text backend owns the shared indexer's Close lifecycle
	backends := map[string]core.StoreBackend{
		"text": text.New(text.WithIndexer(indexer)),
	}
	if hasStoreType(kbs, "sqlite") {
		sqliteBackend, err := sqlite.NewMulti(kbs, sqlite.WithIndexer(indexer))
		if err != nil {
			_ = indexer.Close()
			return nil, err
		}
		backends["sqlite"] = sqliteBackend
	}
	return backends, nil
}

func runService(svc *service.Service, address string, args []string) error {
	claudeInstall := func(out, errOut io.Writer) error {
		return runClaudeInstall(out, errOut)
	}
	cmd := cli.NewRootCommandWithAddressAndRunners(svc, address, Version, func() error {
		return runMCPServer(svc)
	}, claudeInstall, func(out, errOut io.Writer) error {
		return runOpenCodeInstall(out, errOut)
	}, func(version string, install cli.ClaudeInstallRunner, out, errOut io.Writer) error {
		return cli.DefaultUpdateRunner(version, install, out, errOut)
	})
	cmd.SetArgs(args)
	return cmd.Execute()
}

func isNoConfigCommand(args []string) bool {
	return len(args) > 0 && (args[0] == "install" || args[0] == "update" || args[0] == "version")
}

func hasStoreType(kbs []core.KnowledgeBase, storeType string) bool {
	for _, kb := range kbs {
		if kb.StoreType == storeType {
			return true
		}
	}
	return false
}
