package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	lookPath map[string]lookPathResult
	runs     map[string]runResult
	lookedUp []string
	calls    []commandCall
}

type lookPathResult struct {
	path string
	err  error
}

type runResult struct {
	result CommandResult
	err    error
}

type commandCall struct {
	name string
	args []string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		lookPath: map[string]lookPathResult{"codex": {path: "/usr/local/bin/codex"}},
		runs: map[string]runResult{
			cmdKey("codex", "plugin", "add", "--help"):  {},
			cmdKey("codex", "plugin", "list", "--help"): {},
		},
	}
}

func (r *fakeRunner) LookPath(file string) (string, error) {
	r.lookedUp = append(r.lookedUp, file)
	result, ok := r.lookPath[file]
	if !ok {
		return "", fmt.Errorf("%s not found", file)
	}
	return result.path, result.err
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	r.calls = append(r.calls, commandCall{name: name, args: append([]string(nil), args...)})
	result, ok := r.runs[cmdKey(name, args...)]
	if !ok {
		return CommandResult{}, fmt.Errorf("unexpected command: %s", cmdKey(name, args...))
	}
	return result.result, result.err
}

func cmdKey(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), "\x00")
}

func TestInstallFreshCreatesPersonalMarketplaceAndPluginBundle(t *testing.T) {
	home := t.TempDir()
	executable := filepath.Join(home, "bin", "knowledger")
	runner := newFakeRunner()
	runner.runs[cmdKey("codex", "plugin", "add", "knowledger@personal", "--json")] = runResult{}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return executable, nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)
	var out strings.Builder

	if err := installer.Install(&out, &strings.Builder{}); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	wantCalls := []commandCall{
		{name: "codex", args: []string{"plugin", "add", "--help"}},
		{name: "codex", args: []string{"plugin", "list", "--help"}},
		{name: "codex", args: []string{"plugin", "add", "knowledger@personal", "--json"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls mismatch\ngot:  %#v\nwant: %#v", runner.calls, wantCalls)
	}

	pluginRoot := filepath.Join(home, "plugins", "knowledger")
	for _, path := range []string{
		filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"),
		filepath.Join(pluginRoot, ".mcp.json"),
		filepath.Join(pluginRoot, "skills", "knowledger", "SKILL.md"),
		filepath.Join(pluginRoot, "skills", "create-knowledge-base", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "hooks")); !os.IsNotExist(err) {
		t.Fatalf("Codex bundle must not contain Claude hooks, stat error: %v", err)
	}

	mcpData, err := os.ReadFile(filepath.Join(pluginRoot, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mcp struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("parse MCP manifest: %v", err)
	}
	server := mcp.Servers["knowledger"]
	if server.Type != "stdio" || server.Command != executable || !reflect.DeepEqual(server.Args, []string{"mcp"}) {
		t.Fatalf("unexpected MCP server: %#v", server)
	}

	marketplaceData, err := os.ReadFile(filepath.Join(home, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var marketplace struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Source string `json:"source"`
				Path   string `json:"path"`
			} `json:"source"`
			Policy map[string]string `json:"policy"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(marketplaceData, &marketplace); err != nil {
		t.Fatalf("parse marketplace: %v", err)
	}
	if marketplace.Name != "personal" || len(marketplace.Plugins) != 1 {
		t.Fatalf("unexpected marketplace: %#v", marketplace)
	}
	entry := marketplace.Plugins[0]
	if entry.Name != "knowledger" || entry.Source.Source != "local" || entry.Source.Path != "./plugins/knowledger" {
		t.Fatalf("unexpected marketplace entry: %#v", entry)
	}
	if entry.Policy["installation"] != "AVAILABLE" || entry.Policy["authentication"] != "ON_INSTALL" {
		t.Fatalf("unexpected marketplace policy: %#v", entry.Policy)
	}

	for _, want := range []string{"Checking Codex...", "Installing Knowledger Codex plugin...", "Knowledger is installed for Codex.", "codex plugin list", "new Codex thread"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q\nstdout:\n%s", want, out.String())
		}
	}
}

func TestInstallPreservesExistingMarketplaceAndUsesItsName(t *testing.T) {
	home := t.TempDir()
	marketplacePath := filepath.Join(home, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(marketplacePath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{
  "name": "my-tools",
  "interface": {"displayName": "My Tools"},
  "custom": {"preserve": true},
  "plugins": [{
    "name": "other",
    "source": {"source": "local", "path": "./plugins/other"},
    "policy": {"installation": "AVAILABLE", "authentication": "ON_USE"},
    "category": "Productivity"
  }]
}`
	if err := os.WriteFile(marketplacePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := newFakeRunner()
	runner.runs[cmdKey("codex", "plugin", "add", "knowledger@my-tools", "--json")] = runResult{}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	if err := installer.Install(&strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	data, err := os.ReadFile(marketplacePath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["custom"]; !ok {
		t.Fatalf("custom marketplace metadata was removed: %s", data)
	}
	var plugins []map[string]any
	if err := json.Unmarshal(root["plugins"], &plugins); err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 || plugins[0]["name"] != "other" || plugins[1]["name"] != "knowledger" {
		t.Fatalf("unexpected plugin order: %#v", plugins)
	}
}

func TestInstallRerunDoesNotRewriteMarketplace(t *testing.T) {
	home := t.TempDir()
	runner := newFakeRunner()
	runner.runs[cmdKey("codex", "plugin", "add", "knowledger@personal", "--json")] = runResult{}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)
	if err := installer.Install(&strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	marketplacePath := filepath.Join(home, ".agents", "plugins", "marketplace.json")
	before, err := os.ReadFile(marketplacePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := installer.Install(&strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(marketplacePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("marketplace changed on rerun\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestInstallConflictingMarketplaceEntryFailsBeforeMutation(t *testing.T) {
	home := t.TempDir()
	marketplacePath := filepath.Join(home, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(marketplacePath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"name":"personal","plugins":[{"name":"knowledger","source":{"source":"local","path":"./plugins/other"}}]}`
	if err := os.WriteFile(marketplacePath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := newFakeRunner()
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	err := installer.Install(&strings.Builder{}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "conflicting Codex marketplace entry") {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "plugins", "knowledger")); !os.IsNotExist(err) {
		t.Fatalf("plugin bundle should not be created on conflict, stat error: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("plugin add should not run after conflict: %#v", runner.calls)
	}
}

func TestInstallRefusesToOverwriteUnownedPluginDirectory(t *testing.T) {
	home := t.TempDir()
	pluginRoot := filepath.Join(home, "plugins", "knowledger")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(pluginRoot, "user-data.txt")
	if err := os.WriteFile(marker, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := newFakeRunner()
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	err := installer.Install(&strings.Builder{}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace existing non-Knowledger Codex plugin path") {
		t.Fatalf("expected ownership error, got %v", err)
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "keep me" {
		t.Fatalf("existing plugin directory was changed: data=%q err=%v", data, readErr)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("plugin add should not run after ownership failure: %#v", runner.calls)
	}
}

func TestInstallMissingCodexFailsBeforeFilesystemMutation(t *testing.T) {
	home := t.TempDir()
	runner := newFakeRunner()
	runner.lookPath["codex"] = lookPathResult{err: errors.New("not found")}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	err := installer.Install(&strings.Builder{}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "Codex must be installed or updated") || !strings.Contains(err.Error(), "knowledger install --codex") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no commands after missing Codex, got %#v", runner.calls)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("marketplace should not be created, stat error: %v", err)
	}
}

func TestInstallReportsCodexPluginAddFailure(t *testing.T) {
	home := t.TempDir()
	runner := newFakeRunner()
	runner.runs[cmdKey("codex", "plugin", "add", "knowledger@personal", "--json")] = runResult{
		result: CommandResult{Stderr: "plugin rejected"},
		err:    errors.New("exit status 1"),
	}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "/abs/knowledger", nil }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	err := installer.Install(&strings.Builder{}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "codex plugin add knowledger@personal --json") || !strings.Contains(err.Error(), "plugin rejected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallFallsBackToLookPathForKnowledger(t *testing.T) {
	home := t.TempDir()
	runner := newFakeRunner()
	runner.lookPath["knowledger"] = lookPathResult{path: "/resolved/knowledger"}
	runner.runs[cmdKey("codex", "plugin", "add", "knowledger@personal", "--json")] = runResult{}
	installer := NewInstaller(
		WithRunner(runner),
		WithExecutablePath(func() (string, error) { return "", errors.New("unavailable") }),
		WithHomeDir(func() (string, error) { return home, nil }),
	)

	if err := installer.Install(&strings.Builder{}, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.lookedUp, []string{"knowledger", "codex"}) {
		t.Fatalf("lookups = %#v", runner.lookedUp)
	}
}
