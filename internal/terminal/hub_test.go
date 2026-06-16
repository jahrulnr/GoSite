package terminal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	dir := t.TempDir()
	h := NewHub(HubConfig{
		StickyTTL:    100 * time.Millisecond,
		DumpDir:      dir,
		DumpMax:      16 * 1024,
		PerUser:      3,
		DefaultShell: "/bin/sh",
		DefaultCwd:   dir,
		DefaultEnv:   []string{"PATH=/usr/bin:/bin"},
	})
	if err := h.EnsureDumpDir(); err != nil {
		t.Fatalf("ensure dump dir: %v", err)
	}
	t.Cleanup(h.Stop)
	return h
}

func TestHubCreateAndGet(t *testing.T) {
	h := newTestHub(t)
	s, err := h.Create(context.Background(), 1, CreateOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s == nil {
		t.Fatal("nil session")
	}
	got, ok := h.Get(s.id)
	if !ok {
		t.Fatalf("Get: not found")
	}
	if got.id != s.id {
		t.Errorf("id mismatch: got %q want %q", got.id, s.id)
	}
	defer s.Kill()
}

func TestHubCreateRespectsPerUserCap(t *testing.T) {
	h := newTestHub(t)
	created := []*PtySession{}
	for i := 0; i < h.perUser; i++ {
		s, err := h.Create(context.Background(), 42, CreateOptions{})
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		created = append(created, s)
	}
	defer func() {
		for _, s := range created {
			s.Kill()
		}
	}()

	if _, err := h.Create(context.Background(), 42, CreateOptions{}); err == nil {
		t.Fatal("expected per-user cap error")
	}
}

func TestHubListByUser(t *testing.T) {
	h := newTestHub(t)
	s1, _ := h.Create(context.Background(), 7, CreateOptions{})
	s2, _ := h.Create(context.Background(), 7, CreateOptions{})
	defer s1.Kill()
	defer s2.Kill()

	other, _ := h.Create(context.Background(), 8, CreateOptions{})
	defer other.Kill()

	list := h.ListByUser(7)
	if len(list) != 2 {
		t.Errorf("expected 2 sessions for user 7, got %d", len(list))
	}
	if h.Count() < 3 {
		t.Errorf("expected Count >= 3, got %d", h.Count())
	}
}

func TestHubAttachOrCreate(t *testing.T) {
	h := newTestHub(t)
	s, _, err := h.AttachOrCreate(context.Background(), 1, "", CreateOptions{})
	if err != nil {
		t.Fatalf("AttachOrCreate: %v", err)
	}
	defer s.Kill()

	same, _, err := h.AttachOrCreate(context.Background(), 1, s.id, CreateOptions{})
	if err != nil {
		t.Fatalf("AttachOrCreate existing: %v", err)
	}
	if same.id != s.id {
		t.Errorf("expected same session, got %q vs %q", same.id, s.id)
	}
}

func TestHubAttachOrCreateUserIsolation(t *testing.T) {
	h := newTestHub(t)
	s, _ := h.Create(context.Background(), 1, CreateOptions{})
	defer s.Kill()

	_, _, err := h.AttachOrCreate(context.Background(), 2, s.id, CreateOptions{})
	if err == nil {
		t.Fatal("expected cross-user access to be rejected")
	}
}

func TestHubAttachOrCreateRestoresFromDump(t *testing.T) {
	h := newTestHub(t)
	// Seed a dump file as if a previous run left it behind.
	id := h.NewID()
	dumpPath := filepath.Join(h.DumpDir(), "gosite-term-"+id+".log")
	if err := os.WriteFile(dumpPath, []byte("hello from previous run\n"), 0o644); err != nil {
		t.Fatalf("seed dump: %v", err)
	}

	s, gotID, err := h.AttachOrCreate(context.Background(), 1, id, CreateOptions{})
	if err != nil {
		t.Fatalf("AttachOrCreate: %v", err)
	}
	defer s.Kill()
	if gotID != id {
		t.Errorf("id mismatch: got %q, want %q", gotID, id)
	}
	_, first, _ := s.Snapshot()
	if first != uint64(len("hello from previous run\n")) {
		t.Errorf("firstSeq mismatch: got %d", first)
	}
}

func TestHubKillRemovesDump(t *testing.T) {
	h := newTestHub(t)
	s, _ := h.Create(context.Background(), 1, CreateOptions{})
	id := s.id

	if err := h.Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if _, err := os.Stat(h.DumpFileFor(id)); !os.IsNotExist(err) {
		t.Errorf("dump file should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(h.ResolvePath(id)); !os.IsNotExist(err) {
		t.Errorf("ResolvePath dump should be gone, stat err=%v", err)
	}
	if _, ok := h.Get(id); ok {
		t.Errorf("session should be unregistered after Kill")
	}
}

func TestHubKillMissing(t *testing.T) {
	h := newTestHub(t)
	if err := h.Kill("does-not-exist"); err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestHubSweeperKillsStale(t *testing.T) {
	h := newTestHub(t)
	s, _ := h.Create(context.Background(), 1, CreateOptions{})
	id := s.id
	// wait past the sticky TTL
	time.Sleep(150 * time.Millisecond)
	h.sweep()
	if _, ok := h.Get(id); ok {
		t.Error("sweeper should have killed stale session")
	}
}

func TestHubSweeperKeepsActive(t *testing.T) {
	h := newTestHub(t)
	s, _ := h.Create(context.Background(), 1, CreateOptions{})
	defer s.Kill()
	// Refresh the attach timer repeatedly so the session is never stale.
	for i := 0; i < 5; i++ {
		time.Sleep(40 * time.Millisecond)
		s.touchAttach()
		s.touchInput()
	}
	if _, ok := h.Get(s.id); !ok {
		t.Error("active session should not be swept")
	}
}

func TestHubSweeperKillsNoWriterStale(t *testing.T) {
	h := newTestHub(t)
	s, _ := h.Create(context.Background(), 1, CreateOptions{})
	defer s.Kill()
	// Detach any writer. With no writer, the sweeper kills after
	// last_attach > TTL.
	time.Sleep(150 * time.Millisecond)
	h.sweep()
	if _, ok := h.Get(s.id); ok {
		t.Error("sweeper should have killed session with no writer past TTL")
	}
}

func TestHubAttachAndPumpSendsReadyAndSnapshot(t *testing.T) {
	h := newTestHub(t)
	s, err := h.Create(context.Background(), 1, CreateOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer s.Kill()

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

	// Read the first frame; it should be the ready frame (text).
	_ = cli.SetReadDeadline(time.Now().Add(3 * time.Second))
	msgType, raw, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read ready: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Fatalf("expected text ready frame, got %d", msgType)
	}
	decoded, err := DecodeText(raw)
	if err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	ready, ok := decoded.(*ReadyFrame)
	if !ok {
		t.Fatalf("expected *ReadyFrame, got %T", decoded)
	}
	if ready.Role != RoleWriter {
		t.Errorf("expected writer role, got %q", ready.Role)
	}
	if ready.SessionID != s.id {
		t.Errorf("session id mismatch: %q vs %q", ready.SessionID, s.id)
	}

	// Send a command via the client; verify the server forwards it to the
	// session and the client receives the echo output. The snapshot is
	// skipped (it is binary; we already verified the ready frame is text).
	input := InputFrame{Type: FrameInput, Data: base64EncodeString([]byte("echo hub-pump\n"))}
	inputBytes, _ := EncodeText(input)
	if err := cli.WriteMessage(websocket.TextMessage, inputBytes); err != nil {
		t.Fatalf("write input: %v", err)
	}

	got := false
	func() {
		defer func() { _ = recover() }()
		for !got {
			_ = cli.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, data, err := cli.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return
				}
				continue
			}
			if len(data) > 8 && contains(data[8:], []byte("hub-pump")) {
				got = true
				return
			}
		}
	}()
	if !got {
		t.Fatal("never received echo output through pump")
	}

	_ = cli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not exit after client close")
	}
}

// base64EncodeString is a local helper so the test file does not have to
// import encoding/base64 (which would shadow the inline encoder below).
func base64EncodeString(b []byte) string {
	out := base64Encode(b)
	return string(out)
}

// base64Encode is split out so we don't pull encoding/base64 into the file
// twice; the rest of the test file uses the public package functions.
func base64Encode(b []byte) []byte {
	out := make([]byte, ((len(b)+2)/3)*4)
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	for i := 0; i < len(b); i += 3 {
		var n uint32
		switch len(b) - i {
		case 1:
			n = uint32(b[i]) << 16
			out[(i/3)*4+0] = tbl[(n>>18)&0x3F]
			out[(i/3)*4+1] = tbl[(n>>12)&0x3F]
			out[(i/3)*4+2] = '='
			out[(i/3)*4+3] = '='
		case 2:
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8
			out[(i/3)*4+0] = tbl[(n>>18)&0x3F]
			out[(i/3)*4+1] = tbl[(n>>12)&0x3F]
			out[(i/3)*4+2] = tbl[(n>>6)&0x3F]
			out[(i/3)*4+3] = '='
		default:
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
			out[(i/3)*4+0] = tbl[(n>>18)&0x3F]
			out[(i/3)*4+1] = tbl[(n>>12)&0x3F]
			out[(i/3)*4+2] = tbl[(n>>6)&0x3F]
			out[(i/3)*4+3] = tbl[n&0x3F]
		}
	}
	return out
}
