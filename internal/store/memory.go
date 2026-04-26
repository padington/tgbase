package store

import "sync"

// MemoryBackend keeps key-value pairs in process memory. Used in tests and
// when no persistent backend is configured.
type MemoryBackend struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{data: make(map[string][]byte)}
}

func (m *MemoryBackend) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *MemoryBackend) Put(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := make([]byte, len(value))
	copy(stored, value)
	m.data[key] = stored
	return nil
}

func (m *MemoryBackend) Close() error { return nil }
