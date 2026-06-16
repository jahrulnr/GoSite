package plugin

import (
	"sync"
)

// OpLock serializes lifecycle operations per plugin_id (wave G).
type OpLock struct {
	mu    sync.Mutex
	locks map[string]struct{}
}

// NewOpLock returns an in-process per-plugin operation lock.
func NewOpLock() *OpLock {
	return &OpLock{locks: map[string]struct{}{}}
}

// TryAcquire returns false if plugin_id already has an in-flight operation.
func (l *OpLock) TryAcquire(pluginID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.locks[pluginID]; ok {
		return false
	}
	l.locks[pluginID] = struct{}{}
	return true
}

// Release frees the plugin_id lock.
func (l *OpLock) Release(pluginID string) {
	l.mu.Lock()
	delete(l.locks, pluginID)
	l.mu.Unlock()
}
