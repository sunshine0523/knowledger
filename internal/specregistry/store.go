package specregistry

import (
	"strconv"
	"sync"

	"github.com/kindbrave/claude-knowledger/internal/config"
)

// Store persists the runtime specification list. Version returns an opaque
// token that changes whenever the underlying data changes; callers compare
// it across calls to detect external mutations (e.g. another process
// writing the same specs file) and reload accordingly. A consistent empty
// token is returned when the store has no data yet.
type Store interface {
	List() ([]config.SpecificationConfig, error)
	Save([]config.SpecificationConfig) error
	Version() (string, error)
}

type MemoryStore struct {
	mu      sync.Mutex
	items   []config.SpecificationConfig
	version uint64
}

func NewMemoryStore(items []config.SpecificationConfig) *MemoryStore {
	return &MemoryStore{items: items}
}

func (m *MemoryStore) List() ([]config.SpecificationConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]config.SpecificationConfig, len(m.items))
	copy(out, m.items)
	return out, nil
}

func (m *MemoryStore) Save(items []config.SpecificationConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make([]config.SpecificationConfig, len(items))
	copy(m.items, items)
	m.version++
	return nil
}

func (m *MemoryStore) Version() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strconv.FormatUint(m.version, 10), nil
}
