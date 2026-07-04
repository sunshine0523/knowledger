package specregistry

import (
	"fmt"
	"sort"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
)

const (
	SourceStatic  = "static"
	SourceRuntime = "runtime"
)

// SpecificationRecord wraps a SpecificationConfig with metadata about
// where it came from. Scope reflects the storage location (global or
// project); Source distinguishes yaml-declared static specs from
// runtime-created ones; Deletable is false for static entries.
type SpecificationRecord struct {
	Specification config.SpecificationConfig
	Scope         string
	Source        string
	Deletable     bool
}

type Registry struct {
	static       []config.SpecificationConfig
	globalStore  Store
	projectStore Store
	projectRoot  string
}

func New(static []config.SpecificationConfig, globalStore, projectStore Store, projectRoot string) *Registry {
	return &Registry{
		static:       static,
		globalStore:  globalStore,
		projectStore: projectStore,
		projectRoot:  projectRoot,
	}
}

func (r *Registry) HasProjectStore() bool {
	return r.projectStore != nil
}

func (r *Registry) ProjectRoot() string {
	return r.projectRoot
}

// Signature returns a token that changes whenever the underlying runtime
// stores change, including changes made by other processes that write the
// same backing file. Callers cache the value and compare against a fresh
// Signature() to decide whether to reload.
func (r *Registry) Signature() (string, error) {
	globalVer, err := r.globalStore.Version()
	if err != nil {
		return "", err
	}
	projectVer := ""
	if r.projectStore != nil {
		projectVer, err = r.projectStore.Version()
		if err != nil {
			return "", err
		}
	}
	return "g=" + globalVer + "|p=" + projectVer, nil
}

// ListWithSources returns every spec record across all scopes. When a spec
// id appears in multiple scopes, all entries are returned separately —
// callers use ListWithSources when they need the full picture (e.g. for
// listing what could be deleted). Ordering: project scope first, then
// global; within a scope sorted by id.
func (r *Registry) ListWithSources() ([]SpecificationRecord, error) {
	type scopedKey struct {
		Scope string
		ID    string
	}
	merged := map[scopedKey]SpecificationRecord{}

	for _, item := range r.static {
		key := scopedKey{Scope: core.ScopeGlobal, ID: item.ID}
		merged[key] = SpecificationRecord{Specification: item, Scope: core.ScopeGlobal, Source: SourceStatic, Deletable: false}
	}

	globalRuntime, err := r.globalStore.List()
	if err != nil {
		return nil, err
	}
	for _, item := range globalRuntime {
		key := scopedKey{Scope: core.ScopeGlobal, ID: item.ID}
		merged[key] = SpecificationRecord{Specification: item, Scope: core.ScopeGlobal, Source: SourceRuntime, Deletable: true}
	}

	if r.projectStore != nil {
		projectRuntime, err := r.projectStore.List()
		if err != nil {
			return nil, err
		}
		for _, item := range projectRuntime {
			key := scopedKey{Scope: core.ScopeProject, ID: item.ID}
			merged[key] = SpecificationRecord{Specification: item, Scope: core.ScopeProject, Source: SourceRuntime, Deletable: true}
		}
	}

	keys := make([]scopedKey, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Scope != keys[j].Scope {
			return keys[i].Scope == core.ScopeProject
		}
		return keys[i].ID < keys[j].ID
	})

	out := make([]SpecificationRecord, 0, len(keys))
	for _, k := range keys {
		out = append(out, merged[k])
	}
	return out, nil
}

// List returns a deduplicated flat view of all specs. When the same id
// exists in multiple scopes, project overrides global runtime overrides
// static. This is the shape most consumers want — spec engine registers
// providers by id and cannot handle duplicate ids.
func (r *Registry) List() ([]config.SpecificationConfig, error) {
	records, err := r.ListWithSources()
	if err != nil {
		return nil, err
	}
	// Priority order: project runtime > global runtime > static.
	// ListWithSources emits project entries first, so iterating in reverse
	// and keeping the LAST seen per id would flip the priority. Iterate
	// forward and keep the FIRST seen per id instead.
	seen := map[string]struct{}{}
	out := make([]config.SpecificationConfig, 0, len(records))
	for _, rec := range records {
		if _, ok := seen[rec.Specification.ID]; ok {
			continue
		}
		seen[rec.Specification.ID] = struct{}{}
		out = append(out, rec.Specification)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *Registry) storeForScope(scope string) (Store, error) {
	switch scope {
	case core.ScopeGlobal:
		return r.globalStore, nil
	case core.ScopeProject:
		if r.projectStore == nil {
			return nil, fmt.Errorf("not in a project directory; cannot operate on scope=project")
		}
		return r.projectStore, nil
	default:
		return nil, fmt.Errorf("unknown scope %q", scope)
	}
}

// Create adds a runtime spec to the store for the given scope. Errors
// when: id is empty; a runtime spec with the same id already exists in
// the same scope; or (for global scope) a static spec with the same id
// exists.
func (r *Registry) Create(scope string, spec config.SpecificationConfig) error {
	scope, err := core.NormalizeScope(scope)
	if err != nil {
		return err
	}
	if spec.ID == "" {
		return fmt.Errorf("specification id is required")
	}
	store, err := r.storeForScope(scope)
	if err != nil {
		return err
	}
	if scope == core.ScopeGlobal {
		for _, s := range r.static {
			if s.ID == spec.ID {
				return fmt.Errorf("specification %q already exists", spec.ID)
			}
		}
	}
	existing, err := store.List()
	if err != nil {
		return err
	}
	for _, e := range existing {
		if e.ID == spec.ID {
			return fmt.Errorf("specification %q already exists", spec.ID)
		}
	}
	existing = append(existing, spec)
	return store.Save(existing)
}

// Delete removes a runtime spec from the store for the given scope.
// Static specs cannot be deleted; attempting to delete one returns an
// explanatory error.
func (r *Registry) Delete(scope, id string) error {
	scope, err := core.NormalizeScope(scope)
	if err != nil {
		return err
	}
	store, err := r.storeForScope(scope)
	if err != nil {
		return err
	}
	items, err := store.List()
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].ID == id {
			items = append(items[:i], items[i+1:]...)
			return store.Save(items)
		}
	}
	if scope == core.ScopeGlobal {
		for _, s := range r.static {
			if s.ID == id {
				return fmt.Errorf("specification %q is defined in static config", id)
			}
		}
	}
	return fmt.Errorf("specification %q not found in %s runtime registry", id, scope)
}

// RuntimeItems exposes the raw contents of one store (unmerged). Rarely
// needed outside tests; kept for symmetry with registry.Registry.
func (r *Registry) RuntimeItems(scope string) ([]config.SpecificationConfig, error) {
	scope, err := core.NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	store, err := r.storeForScope(scope)
	if err != nil {
		return nil, err
	}
	return store.List()
}
