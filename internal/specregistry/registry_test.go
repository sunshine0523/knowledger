package specregistry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/specregistry"
)

func kbSpec(id string) config.SpecificationConfig {
	return config.SpecificationConfig{
		ID:      id,
		Name:    id,
		Type:    "kb",
		Enabled: true,
		Source:  config.SourceConfig{KBID: id, Tags: []string{"style"}},
	}
}

func TestFileStoreMissingFileReturnsEmptyList(t *testing.T) {
	store := specregistry.NewFileStore(filepath.Join(t.TempDir(), "specs.json"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %#v", items)
	}
}

func TestFileStoreSaveAndListRoundTrip(t *testing.T) {
	store := specregistry.NewFileStore(filepath.Join(t.TempDir(), "state", "specs.json"))
	items := []config.SpecificationConfig{kbSpec("docs")}

	if err := store.Save(items); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	got, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "docs" || got[0].Source.KBID != "docs" {
		t.Fatalf("unexpected round trip result: %#v", got)
	}
}

func TestFileStoreMalformedJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "specs.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write malformed specs: %v", err)
	}
	store := specregistry.NewFileStore(path)
	if _, err := store.List(); err == nil {
		t.Fatalf("expected malformed JSON error")
	}
}

func TestFileStoreVersionChangesOnSave(t *testing.T) {
	store := specregistry.NewFileStore(filepath.Join(t.TempDir(), "specs.json"))
	v0, err := store.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v0 != "" {
		t.Fatalf("expected empty version for missing file, got %q", v0)
	}
	if err := store.Save([]config.SpecificationConfig{kbSpec("a")}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	v1, err := store.Version()
	if err != nil {
		t.Fatalf("Version after save: %v", err)
	}
	if v1 == "" {
		t.Fatalf("expected non-empty version after save")
	}
}

func TestRegistryCreatesRuntimeSpec(t *testing.T) {
	r := specregistry.New(nil, specregistry.NewMemoryStore(nil), nil, "")
	if err := r.Create(core.ScopeGlobal, kbSpec("docs")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	records, err := r.ListWithSources()
	if err != nil {
		t.Fatalf("ListWithSources: %v", err)
	}
	if len(records) != 1 || records[0].Specification.ID != "docs" || records[0].Source != specregistry.SourceRuntime || !records[0].Deletable {
		t.Fatalf("unexpected source-aware record: %#v", records)
	}
}

func TestRegistryRejectsDuplicateCreate(t *testing.T) {
	static := []config.SpecificationConfig{kbSpec("docs")}
	r := specregistry.New(static, specregistry.NewMemoryStore(nil), nil, "")

	if err := r.Create(core.ScopeGlobal, kbSpec("docs")); err == nil {
		t.Fatalf("expected duplicate static create to fail")
	}
	if err := r.Create(core.ScopeGlobal, kbSpec("notes")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := r.Create(core.ScopeGlobal, kbSpec("notes")); err == nil {
		t.Fatalf("expected duplicate runtime create to fail")
	}
}

func TestRegistryDeletesRuntimeSpecOnly(t *testing.T) {
	static := []config.SpecificationConfig{kbSpec("static")}
	r := specregistry.New(static, specregistry.NewMemoryStore([]config.SpecificationConfig{kbSpec("runtime")}), nil, "")

	if err := r.Delete(core.ScopeGlobal, "runtime"); err != nil {
		t.Fatalf("Delete runtime: %v", err)
	}
	items, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].ID != "static" {
		t.Fatalf("expected only static item after delete, got %#v", items)
	}
	if err := r.Delete(core.ScopeGlobal, "static"); err == nil {
		t.Fatalf("expected static delete to fail")
	}
}

func TestRegistryDeleteRuntimeOverrideRevealsStaticSpec(t *testing.T) {
	staticSpec := kbSpec("docs")
	staticSpec.Name = "Static Docs"
	runtimeSpec := kbSpec("docs")
	runtimeSpec.Name = "Runtime Docs"

	r := specregistry.New(
		[]config.SpecificationConfig{staticSpec},
		specregistry.NewMemoryStore([]config.SpecificationConfig{runtimeSpec}),
		nil, "",
	)

	items, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Runtime Docs" {
		t.Fatalf("expected runtime override before delete, got %#v", items)
	}
	if err := r.Delete(core.ScopeGlobal, "docs"); err != nil {
		t.Fatalf("Delete runtime override: %v", err)
	}
	items, err = r.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Static Docs" {
		t.Fatalf("expected static revealed after delete, got %#v", items)
	}
}

func TestRegistryHasProjectStore(t *testing.T) {
	r := specregistry.New(nil, specregistry.NewMemoryStore(nil), nil, "")
	if r.HasProjectStore() {
		t.Fatalf("expected HasProjectStore=false")
	}
	r2 := specregistry.New(nil, specregistry.NewMemoryStore(nil), specregistry.NewMemoryStore(nil), "/tmp/proj")
	if !r2.HasProjectStore() {
		t.Fatalf("expected HasProjectStore=true")
	}
}

func TestRegistryProjectOverridesGlobalOnListFlat(t *testing.T) {
	globalSpec := kbSpec("shared")
	globalSpec.Name = "Global"
	projectSpec := kbSpec("shared")
	projectSpec.Name = "Project"

	r := specregistry.New(
		nil,
		specregistry.NewMemoryStore([]config.SpecificationConfig{globalSpec}),
		specregistry.NewMemoryStore([]config.SpecificationConfig{projectSpec}),
		"/tmp/proj",
	)

	items, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Project" {
		t.Fatalf("expected project spec to win, got %#v", items)
	}

	records, err := r.ListWithSources()
	if err != nil {
		t.Fatalf("ListWithSources: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected both records in ListWithSources, got %d: %#v", len(records), records)
	}
	if records[0].Scope != core.ScopeProject {
		t.Fatalf("expected first record scope=project, got %q", records[0].Scope)
	}
}

func TestRegistryCreateRoutesByScope(t *testing.T) {
	globalStore := specregistry.NewMemoryStore(nil)
	projectStore := specregistry.NewMemoryStore(nil)
	r := specregistry.New(nil, globalStore, projectStore, "/tmp/proj")

	if err := r.Create(core.ScopeGlobal, kbSpec("g1")); err != nil {
		t.Fatalf("global Create: %v", err)
	}
	if err := r.Create(core.ScopeProject, kbSpec("p1")); err != nil {
		t.Fatalf("project Create: %v", err)
	}
	g, _ := globalStore.List()
	if len(g) != 1 || g[0].ID != "g1" {
		t.Fatalf("global store should contain g1, got %#v", g)
	}
	p, _ := projectStore.List()
	if len(p) != 1 || p[0].ID != "p1" {
		t.Fatalf("project store should contain p1, got %#v", p)
	}
}

func TestRegistryCreateProjectFailsWithoutProjectStore(t *testing.T) {
	r := specregistry.New(nil, specregistry.NewMemoryStore(nil), nil, "")
	if err := r.Create(core.ScopeProject, kbSpec("x")); err == nil {
		t.Fatalf("expected error creating project spec without project store")
	}
}

func TestRegistryAllowsSameIDAcrossScopes(t *testing.T) {
	r := specregistry.New(nil, specregistry.NewMemoryStore(nil), specregistry.NewMemoryStore(nil), "/tmp/proj")
	if err := r.Create(core.ScopeGlobal, kbSpec("notes")); err != nil {
		t.Fatalf("global Create: %v", err)
	}
	if err := r.Create(core.ScopeProject, kbSpec("notes")); err != nil {
		t.Fatalf("project Create with same id should succeed: %v", err)
	}
}

func TestRegistryDeleteScopedRoutesCorrectly(t *testing.T) {
	globalStore := specregistry.NewMemoryStore([]config.SpecificationConfig{kbSpec("shared")})
	projectStore := specregistry.NewMemoryStore([]config.SpecificationConfig{kbSpec("shared")})
	r := specregistry.New(nil, globalStore, projectStore, "/tmp/proj")

	if err := r.Delete(core.ScopeProject, "shared"); err != nil {
		t.Fatalf("project Delete: %v", err)
	}
	g, _ := globalStore.List()
	if len(g) != 1 {
		t.Fatalf("global store should be untouched, got %#v", g)
	}
	p, _ := projectStore.List()
	if len(p) != 0 {
		t.Fatalf("project store should be empty, got %#v", p)
	}
}

func TestRegistrySignatureChangesOnMutation(t *testing.T) {
	globalStore := specregistry.NewMemoryStore(nil)
	r := specregistry.New(nil, globalStore, nil, "")
	sig0, err := r.Signature()
	if err != nil {
		t.Fatalf("Signature: %v", err)
	}
	if err := r.Create(core.ScopeGlobal, kbSpec("x")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sig1, err := r.Signature()
	if err != nil {
		t.Fatalf("Signature after create: %v", err)
	}
	if sig0 == sig1 {
		t.Fatalf("expected signature to change after create; got %q both times", sig0)
	}
}
