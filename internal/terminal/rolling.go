package terminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RollingFile is a simple size-bounded append-only log file. When the file
// exceeds the configured maximum, the oldest 25% of bytes are trimmed so the
// most recent output is preserved while keeping the on-disk footprint bounded.
//
// The file is single-writer (the PtySession ReadLoop goroutine). Concurrent
// reads via ReadAll() are guarded by a mutex so snapshot delivery to the
// websocket layer is race-free.
type RollingFile struct {
	path string
	max  int64
	mu   sync.Mutex
}

// NewRollingFile returns a RollingFile at path, capped at max bytes. max
// must be > 0. The file itself is not created until the first Append.
func NewRollingFile(path string, max int64) *RollingFile {
	if max < 1024 {
		max = 1024
	}
	return &RollingFile{path: path, max: max}
}

// Path returns the absolute path of the underlying file.
func (r *RollingFile) Path() string {
	return r.path
}

// Append writes p to the file, creating parent directories as needed. If the
// resulting size would exceed max, the oldest 25% of bytes are dropped and the
// remaining bytes are rewritten atomically. Returns the number of bytes
// currently in the file after the append.
func (r *RollingFile) Append(p []byte) (int64, error) {
	if len(p) == 0 {
		return r.Size(), nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return 0, fmt.Errorf("rolling: mkdir: %w", err)
	}

	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("rolling: open: %w", err)
	}

	currentSize, statErr := f.Stat()
	if statErr == nil && currentSize.Size()+int64(len(p)) > r.max {
		_ = f.Close()
		if err := r.rewriteTrimmed(p); err != nil {
			return 0, err
		}
		return r.sizeNoLock(), nil
	}

	if _, err := f.Write(p); err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("rolling: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = err
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("rolling: close: %w", err)
	}
	return r.sizeNoLock(), nil
}

// rewriteTrimmed reads the current contents, drops the oldest 25%, then writes
// back the tail plus the new bytes in a single rename for atomicity.
func (r *RollingFile) rewriteTrimmed(appendage []byte) error {
	existing, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("rolling: read for trim: %w", err)
	}

	targetKeep := int64(float64(r.max) * 0.75)
	if targetKeep < 0 {
		targetKeep = 0
	}
	if int64(len(existing)) > targetKeep {
		existing = existing[int64(len(existing))-targetKeep:]
	}

	combined := make([]byte, 0, len(existing)+len(appendage))
	combined = append(combined, existing...)
	combined = append(combined, appendage...)
	if int64(len(combined)) > r.max {
		combined = combined[int64(len(combined))-r.max:]
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, combined, 0o644); err != nil {
		return fmt.Errorf("rolling: write tmp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("rolling: rename: %w", err)
	}
	return nil
}

// ReadAll returns the current contents of the file. Returns nil if the file
// does not exist yet.
func (r *RollingFile) ReadAll() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("rolling: read: %w", err)
	}
	return data, nil
}

// Size returns the current size of the file in bytes (0 if missing).
func (r *RollingFile) Size() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sizeNoLock()
}

// sizeNoLock is Size without locking; caller must hold r.mu.
func (r *RollingFile) sizeNoLock() int64 {
	info, err := os.Stat(r.path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Remove deletes the underlying file. Idempotent — missing file is not an
// error.
func (r *RollingFile) Remove() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.Remove(r.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("rolling: remove: %w", err)
	}
	return nil
}

// DumpPathFor returns the canonical on-disk path for the rolling dump of a
// session with the given id inside dumpDir.
func DumpPathFor(dumpDir, sessionID string) string {
	return filepath.Join(dumpDir, "gosite-term-"+sessionID+".log")
}
