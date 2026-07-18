package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pluginbundle "github.com/kindbrave/claude-knowledger/plugins/knowledger"
)

const (
	pluginName            = "knowledger"
	defaultMarketplace    = "personal"
	marketplaceSourcePath = "./plugins/knowledger"
	pluginRepository      = "https://github.com/sunshine0523/claude-knowledger"
)

var bundleFiles = []string{
	".codex-plugin/plugin.json",
	"README.md",
	"skills/knowledger/SKILL.md",
	"skills/git-knowledge/SKILL.md",
	"skills/update-knowledger/SKILL.md",
	"skills/kb-code-review/SKILL.md",
	"skills/create-knowledge-base/SKILL.md",
}

type CommandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

type CommandResult struct {
	Stdout string
	Stderr string
}

type Installer struct {
	runner         CommandRunner
	executablePath func() (string, error)
	homeDir        func() (string, error)
}

type Option func(*Installer)

func WithRunner(runner CommandRunner) Option {
	return func(i *Installer) {
		if runner != nil {
			i.runner = runner
		}
	}
}

func WithExecutablePath(executablePath func() (string, error)) Option {
	return func(i *Installer) {
		if executablePath != nil {
			i.executablePath = executablePath
		}
	}
}

func WithHomeDir(homeDir func() (string, error)) Option {
	return func(i *Installer) {
		if homeDir != nil {
			i.homeDir = homeDir
		}
	}
}

func NewInstaller(opts ...Option) *Installer {
	installer := &Installer{
		runner:         execRunner{},
		executablePath: os.Executable,
		homeDir:        os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(installer)
	}
	return installer
}

func (i *Installer) Install(out, errOut io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	runner := i.runner
	if runner == nil {
		runner = execRunner{}
	}

	executable, err := i.resolveExecutablePath(runner)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Checking Codex...")
	if err := preflight(runner); err != nil {
		return err
	}

	home, err := i.homeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory: %w", err)
	}
	marketplace, err := loadMarketplace(home)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Installing Knowledger Codex plugin...")
	if err := materializeBundle(home, executable); err != nil {
		return fmt.Errorf("failed to materialize Knowledger Codex plugin bundle: %w", err)
	}
	if marketplace.changed {
		if err := writeFileAtomic(marketplace.path, marketplace.data, 0o644); err != nil {
			return fmt.Errorf("failed to update Codex personal marketplace at %s: %w", marketplace.path, err)
		}
	}

	pluginID := pluginName + "@" + marketplace.name
	result, err := runner.Run(context.Background(), "codex", "plugin", "add", pluginID, "--json")
	if err != nil {
		return commandFailedError("codex plugin add "+pluginID+" --json", result, err)
	}

	fmt.Fprintln(out, "Knowledger is installed for Codex.")
	fmt.Fprintln(out, "Verify with:")
	fmt.Fprintln(out, "  codex plugin list")
	fmt.Fprintln(out, "Start a new Codex thread to load the plugin.")
	_ = errOut
	return nil
}

func (i *Installer) resolveExecutablePath(runner CommandRunner) (string, error) {
	path, err := i.executablePath()
	if err != nil || path == "" {
		path, err = runner.LookPath("knowledger")
		if err != nil {
			return "", fmt.Errorf("could not resolve Knowledger executable path: %w", err)
		}
	}
	if abs, absErr := filepath.Abs(path); absErr == nil {
		path = abs
	}
	if evaluated, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
		path = evaluated
	}
	return filepath.Clean(path), nil
}

func preflight(runner CommandRunner) error {
	if _, err := runner.LookPath("codex"); err != nil {
		return codexUpdateError("codex", CommandResult{}, err)
	}
	for _, args := range [][]string{{"plugin", "add", "--help"}, {"plugin", "list", "--help"}} {
		result, err := runner.Run(context.Background(), "codex", args...)
		if err != nil {
			return codexUpdateError(strings.Join(append([]string{"codex"}, args...), " "), result, err)
		}
	}
	return nil
}

type marketplaceState struct {
	path    string
	name    string
	data    []byte
	changed bool
}

func loadMarketplace(home string) (marketplaceState, error) {
	path := filepath.Join(home, ".agents", "plugins", "marketplace.json")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return marketplaceState{}, fmt.Errorf("failed to read Codex personal marketplace at %s: %w", path, err)
	}

	root := map[string]json.RawMessage{}
	changed := false
	if errors.Is(err, os.ErrNotExist) {
		root["name"] = mustJSON(defaultMarketplace)
		root["interface"] = mustJSON(map[string]string{"displayName": "Personal"})
		changed = true
	} else if err := json.Unmarshal(data, &root); err != nil {
		return marketplaceState{}, fmt.Errorf("failed to parse Codex personal marketplace at %s: %w", path, err)
	}
	if root == nil {
		return marketplaceState{}, fmt.Errorf("Codex personal marketplace at %s must contain a JSON object", path)
	}

	var name string
	if raw, ok := root["name"]; !ok || json.Unmarshal(raw, &name) != nil || strings.TrimSpace(name) == "" {
		return marketplaceState{}, fmt.Errorf("Codex personal marketplace at %s must contain a non-empty string name", path)
	}
	if raw, ok := root["interface"]; ok {
		var value map[string]json.RawMessage
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return marketplaceState{}, fmt.Errorf("Codex personal marketplace at %s field interface must be an object", path)
		}
	}

	var plugins []json.RawMessage
	if raw, ok := root["plugins"]; ok {
		if err := json.Unmarshal(raw, &plugins); err != nil {
			return marketplaceState{}, fmt.Errorf("Codex personal marketplace at %s field plugins must be an array", path)
		}
	}
	found := false
	for _, raw := range plugins {
		var entry struct {
			Name   string `json:"name"`
			Source struct {
				Source string `json:"source"`
				Path   string `json:"path"`
			} `json:"source"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return marketplaceState{}, fmt.Errorf("Codex personal marketplace at %s contains an invalid plugin entry: %w", path, err)
		}
		if entry.Name != pluginName {
			continue
		}
		found = true
		if entry.Source.Source != "local" || entry.Source.Path != marketplaceSourcePath {
			return marketplaceState{}, fmt.Errorf("found conflicting Codex marketplace entry named knowledger in %s; remove or rename it, then rerun `knowledger install --codex`", path)
		}
	}
	if !found {
		plugins = append(plugins, mustJSON(map[string]any{
			"name": pluginName,
			"source": map[string]string{
				"source": "local",
				"path":   marketplaceSourcePath,
			},
			"policy": map[string]string{
				"installation":   "AVAILABLE",
				"authentication": "ON_INSTALL",
			},
			"category": "Productivity",
		}))
		changed = true
	}
	root["plugins"] = mustJSON(plugins)
	if !changed {
		return marketplaceState{path: path, name: name}, nil
	}
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return marketplaceState{}, fmt.Errorf("failed to encode Codex personal marketplace: %w", err)
	}
	encoded = append(encoded, '\n')
	return marketplaceState{path: path, name: name, data: encoded, changed: true}, nil
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func materializeBundle(home, executable string) error {
	pluginsDir := filepath.Join(home, "plugins")
	target := filepath.Join(pluginsDir, pluginName)
	if err := validateExistingPluginTarget(target); err != nil {
		return err
	}
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(pluginsDir, ".knowledger-")
	if err != nil {
		return err
	}
	moved := false
	defer func() {
		if !moved {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	for _, name := range bundleFiles {
		data, err := fs.ReadFile(pluginbundle.Bundle, name)
		if err != nil {
			return err
		}
		if err := writeBundleFile(tmpDir, name, data); err != nil {
			return err
		}
	}
	mcpData, err := json.MarshalIndent(map[string]any{
		"mcpServers": map[string]any{
			"knowledger": map[string]any{
				"type":    "stdio",
				"command": executable,
				"args":    []string{"mcp"},
			},
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeBundleFile(tmpDir, ".mcp.json", append(mcpData, '\n')); err != nil {
		return err
	}

	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.Rename(tmpDir, target); err != nil {
		return err
	}
	moved = true
	return nil
}

func validateExistingPluginTarget(target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing to replace existing non-Knowledger Codex plugin path %s", target)
	}
	data, err := os.ReadFile(filepath.Join(target, ".codex-plugin", "plugin.json"))
	if err != nil {
		return fmt.Errorf("refusing to replace existing non-Knowledger Codex plugin path %s", target)
	}
	var manifest struct {
		Name       string `json:"name"`
		Repository string `json:"repository"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Name != pluginName || manifest.Repository != pluginRepository {
		return fmt.Errorf("refusing to replace existing non-Knowledger Codex plugin path %s", target)
	}
	return nil
}

func writeBundleFile(root, name string, data []byte) error {
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func writeFileAtomic(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".marketplace-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	moved := false
	defer func() {
		if !moved {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	moved = true
	return nil
}

func codexUpdateError(command string, result CommandResult, err error) error {
	return fmt.Errorf("Codex must be installed or updated, then rerun `knowledger install --codex`: %w", commandFailedError(command, result, err))
}

func commandFailedError(command string, result CommandResult, err error) error {
	message := fmt.Sprintf("%s failed: %v", command, err)
	if strings.TrimSpace(result.Stderr) != "" {
		message += "\nstderr: " + strings.TrimSpace(result.Stderr)
	}
	return errors.New(message)
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, err
}
