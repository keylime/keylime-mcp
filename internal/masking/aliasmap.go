package masking

import (
	"fmt"
	"sync"
)

type AliasMap struct {
	mu      sync.RWMutex
	prefix  string
	counter int
	forward map[string]string
	inverse map[string]string
}

func NewAliasMap(prefix string) *AliasMap {
	return &AliasMap{
		prefix:  prefix,
		forward: make(map[string]string),
		inverse: make(map[string]string),
	}
}

func (m *AliasMap) GetOrCreate(real string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if alias, ok := m.forward[real]; ok {
		return alias
	}
	m.counter++
	alias := fmt.Sprintf("%s-%d", m.prefix, m.counter)
	m.forward[real] = alias
	m.inverse[alias] = real
	return alias
}

func (m *AliasMap) Resolve(alias string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	real, ok := m.inverse[alias]
	return real, ok
}
