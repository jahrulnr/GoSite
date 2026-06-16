package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollingFileAppend(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "term.log")
	rf := NewRollingFile(p, 1024)

	if _, err := rf.Append([]byte("hello ")); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if _, err := rf.Append([]byte("world\n")); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	data, err := rf.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("contents mismatch: got %q, want %q", data, "hello world\n")
	}
	if got := rf.Size(); got != int64(len("hello world\n")) {
		t.Errorf("size mismatch: got %d, want %d", got, len("hello world\n"))
	}
}

func TestRollingFileEmptyAppend(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "term.log")
	rf := NewRollingFile(p, 1024)
	size, err := rf.Append(nil)
	if err != nil {
		t.Fatalf("Append nil: %v", err)
	}
	if size != 0 {
		t.Errorf("expected size 0, got %d", size)
	}
}

func TestRollingFileTrim(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "term.log")
	// 1KB cap so we trigger trim quickly.
	rf := NewRollingFile(p, 1024)

	chunk := bytes.Repeat([]byte("a"), 300)
	for i := 0; i < 10; i++ {
		if _, err := rf.Append(chunk); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	size := rf.Size()
	if size > 1024 {
		t.Errorf("file exceeded max: got %d, want <= 1024", size)
	}

	data, err := rf.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	// The most recent chunk must be present in full, and the very first
	// chunk must have been dropped (so contents should end with a long run
	// of 'a's from the most recent write, not a partial first chunk).
	if !bytes.HasSuffix(data, chunk) {
		t.Errorf("expected tail to match latest chunk, got suffix %q", data[len(data)-20:])
	}
}

func TestRollingFileReadMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "missing.log")
	rf := NewRollingFile(p, 1024)
	data, err := rf.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll on missing file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
	if rf.Size() != 0 {
		t.Errorf("expected size 0, got %d", rf.Size())
	}
}

func TestRollingFileRemove(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "term.log")
	rf := NewRollingFile(p, 1024)
	if _, err := rf.Append([]byte("data")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := rf.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("file should be gone, stat err = %v", err)
	}
	// Second remove should be a no-op.
	if err := rf.Remove(); err != nil {
		t.Errorf("second Remove: %v", err)
	}
}

func TestRollingFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "deep", "term.log")
	rf := NewRollingFile(p, 1024)
	if _, err := rf.Append([]byte("hi")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if !strings.HasPrefix(rf.Path(), dir) {
		t.Errorf("path not preserved: %q", rf.Path())
	}
}

func TestDumpPathFor(t *testing.T) {
	got := DumpPathFor("/tmp", "abc123")
	if !strings.HasSuffix(got, "gosite-term-abc123.log") {
		t.Errorf("unexpected dump path: %s", got)
	}
}
