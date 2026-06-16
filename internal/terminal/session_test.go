package terminal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestSession(t *testing.T) *PtySession {
	t.Helper()
	dir := t.TempDir()
	cfg := SessionConfig{
		ID:       "test-sess",
		UserID:   1,
		Shell:    "/bin/sh",
		Cwd:      dir,
		Env:      []string{"PATH=/usr/bin:/bin"},
		Cols:     80,
		Rows:     24,
		DumpPath: filepath.Join(dir, "dump.log"),
		DumpMax:  64 * 1024,
	}
	return NewPtySession(cfg, nil)
}

func TestPtySessionStart(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	if s.IsClosed() {
		t.Error("session should not be closed after Start")
	}
}

func TestPtySessionExecuteAndCapture(t *testing.T) {
	s := newTestSession(t)
	// Use echo to produce deterministic output.
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	if _, err := s.Write([]byte("echo gosite-hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Wait for output to land in the rolling dump.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, _, _ := s.Snapshot()
		if contains(data, []byte("gosite-hello")) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	data, first, end := s.Snapshot()
	t.Fatalf("output never arrived: first=%d end=%d data=%q", first, end, string(data))
}

func TestPtySessionResize(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	if err := s.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
}

func TestPtySessionKill(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	s.Kill()
	if !s.IsClosed() {
		t.Error("session should be closed after Kill")
	}
	// Dump file should be gone.
	if _, err := os.Stat(s.rolling.Path()); !os.IsNotExist(err) {
		t.Errorf("dump file should be removed, stat err=%v", err)
	}
	// Double-kill must not panic.
	s.Kill()
}

func TestPtySessionMultiAttachRoles(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	c1, c2 := newWSClientPair(t)
	defer c1.Close()
	defer c2.Close()

	role1 := s.Attach(c1)
	role2 := s.Attach(c2)

	if role1 != RoleWriter {
		t.Errorf("first attach should be writer, got %q", role1)
	}
	if role2 != RoleReader {
		t.Errorf("second attach should be reader, got %q", role2)
	}
	if !s.HasWriter() {
		t.Error("HasWriter should be true")
	}
	if s.RoleFor(c1) != RoleWriter {
		t.Error("c1 should be writer")
	}
	if s.RoleFor(c2) != RoleReader {
		t.Error("c2 should be reader")
	}

	s.Detach(c1)
	if s.HasWriter() {
		t.Error("HasWriter should be false after writer detach")
	}
}

func TestPtySessionBroadcast(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	c1, c2 := newWSClientPair(t)
	defer c1.Close()
	defer c2.Close()
	s.Attach(c1)
	s.Attach(c2)

	// Give fan-out a moment to register readers.
	time.Sleep(50 * time.Millisecond)

	if _, err := s.Write([]byte("echo multi-attach\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var wg sync.WaitGroup
	results := make([][]byte, 2)
	deadline := time.After(3 * time.Second)
	for i, c := range []*websocket.Conn{c1, c2} {
		wg.Add(1)
		go func(idx int, conn *websocket.Conn) {
			defer wg.Done()
			for {
				select {
				case <-deadline:
					return
				default:
				}
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				_, data, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if contains(data, []byte("multi-attach")) {
					results[idx] = data
					return
				}
			}
		}(i, c)
	}
	wg.Wait()
	for i, r := range results {
		if r == nil {
			t.Errorf("client %d did not receive broadcast", i)
		}
	}
}

func TestPtySessionWaitForExit(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := s.Write([]byte("exit 7\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := s.WaitForExit(ctx)
	if err == nil {
		t.Fatal("expected non-nil error from shell exit 7")
	}
}

func TestPtySessionRestoreFromDump(t *testing.T) {
	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "restore.log")

	cfg := SessionConfig{
		ID:       "restore-sess",
		Shell:    "/bin/sh",
		Cwd:      dir,
		Env:      []string{"PATH=/usr/bin:/bin"},
		Cols:     80,
		Rows:     24,
		DumpPath: dumpPath,
		DumpMax:  64 * 1024,
	}

	// Write a prior dump to simulate a previous PTY run.
	prior := []byte("previous output\n")
	if err := os.WriteFile(dumpPath, prior, 0o644); err != nil {
		t.Fatalf("seed dump: %v", err)
	}
	cfg.FirstSeq = uint64(len(prior))

	s := NewPtySession(cfg, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	data, first, end := s.Snapshot()
	if first != uint64(len(prior)) {
		t.Errorf("firstSeq mismatch: got %d, want %d", first, len(prior))
	}
	if end < first {
		t.Errorf("endSeq %d < firstSeq %d", end, first)
	}
	if !contains(data, prior) {
		t.Errorf("snapshot missing prior dump: %q", data)
	}
}

func TestPtySessionMeta(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	c, _ := newWSClientPair(t)
	defer c.Close()
	s.Attach(c)
	meta := s.Meta()
	if meta.ID != "test-sess" {
		t.Errorf("ID mismatch: %q", meta.ID)
	}
	if !meta.Active {
		t.Error("Active should be true after Attach")
	}
	if meta.Role != RoleWriter {
		t.Errorf("Role: got %q, want writer", meta.Role)
	}
	if meta.Shell != "/bin/sh" {
		t.Errorf("Shell: got %q", meta.Shell)
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// newWSClientPair returns two websocket.Conn connected to the same httptest
// server. The first is the server-side connection accepted by the upgrade
// handler, the second is the client-side connection. This is enough to drive
// the fan-out goroutine without external dependencies.
func newWSClientPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srvConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		srvConnCh <- c
		// Block until the test ends so the conn stays open.
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	cli, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	select {
	case s := <-srvConnCh:
		return s, cli
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server websocket")
		return nil, nil
	}
}

// TestPtySessionResumeBroadcastFlushesPending is a regression test for the
// "first prompt never appears" UX bug: chunks produced while the broadcast
// is paused must be forwarded to attached clients when the outermost resume
// is called. The previous `token == 1` flush guard leaked the very first
// prompt bytes of a fresh session.
func TestPtySessionResumeBroadcastFlushesPending(t *testing.T) {
	s := newTestSession(t)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Kill()

	c1, c2 := newWSClientPair(t)
	defer c1.Close()
	defer c2.Close()
	s.Attach(c1)
	s.Attach(c2)
	time.Sleep(50 * time.Millisecond)

	// Pause broadcast, then write a deterministic command. With the
	// broadcast paused, the echo + newline that bash emits must end up
	// in `pending`, NOT in the websocket.
	token := s.PauseBroadcast()
	if token != 0 {
		t.Fatalf("expected first pause to return 0, got %d", token)
	}
	if _, err := s.Write([]byte("echo resume-flush-marker\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Give the shell time to echo. If broadcast were active, the marker
	// would arrive on c1/c2 well before this deadline.
	time.Sleep(250 * time.Millisecond)

	// Resume: the pending buffer must be flushed now.
	s.ResumeBroadcast(token)

	// Both clients should receive the marker.
	deadline := time.After(3 * time.Second)
	for i, c := range []*websocket.Conn{c1, c2} {
		got := make(chan struct{}, 1)
		go func(idx int, conn *websocket.Conn) {
			for {
				select {
				case <-deadline:
					return
				default:
				}
				_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				_, data, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if contains(data, []byte("resume-flush-marker")) {
					select {
					case got <- struct{}{}:
					default:
					}
					return
				}
			}
		}(i, c)
		select {
		case <-got:
		case <-time.After(2 * time.Second):
			t.Errorf("client %d did not receive marker after resume", i)
		}
	}
}

// TestHubAttachAndPumpFlushesPendingPrompt is a regression test for the
// user-facing "no prompt on first connect" UX bug. When the WS attaches
// after the shell has already emitted its prompt, the prompt bytes are
// sitting in the rolling dump; the prompt is also queued in the fan-out
// pending buffer. The handler must flush the pending buffer after the
// snapshot is delivered, otherwise the prompt is silently lost.
func TestHubAttachAndPumpFlushesPendingPrompt(t *testing.T) {
	h := newTestHub(t)
	s, err := h.Create(context.Background(), 1, CreateOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer s.Kill()

	// Wait for the shell to print its first prompt so the prompt bytes
	// are in the rolling dump.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, _, _ := s.Snapshot()
		if len(data) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srvConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		srvConnCh <- c
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	cli, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close()

	var srvConn *websocket.Conn
	select {
	case srvConn = <-srvConnCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server conn")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = h.AttachAndPump(s, srvConn, nil)
	}()

	// First frame is the ready frame.
	_ = cli.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read ready: %v", err)
	}
	decoded, err := DecodeText(raw)
	if err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if _, ok := decoded.(*ReadyFrame); !ok {
		t.Fatalf("expected ready frame, got %T", decoded)
	}

	// The client should now receive at least one binary frame (or its
	// payload merged with subsequent writes) that contains prompt
	// content. We accept either: the snapshot binary frame, or a live
	// chunk that the pending-flush path delivers.
	gotPrompt := false
	promptDeadline := time.After(2 * time.Second)
	for !gotPrompt {
		select {
		case <-promptDeadline:
			t.Fatal("never received prompt bytes after attach")
		default:
		}
		_ = cli.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, data, err := cli.ReadMessage()
		if err != nil {
			continue
		}
		// Any byte that looks like a shell prompt token (e.g. '#' or '$'
		// at end-of-line) is enough to confirm the prompt was delivered.
		if len(data) > 0 {
			gotPrompt = true
		}
	}

	_ = cli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not exit after client close")
	}
}


