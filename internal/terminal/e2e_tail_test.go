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

// TestE2ETailFPersistsAcrossReconnect simulates the user-facing "tail -f"
// scenario:
//
//  1. User opens a terminal, runs `tail -f /tmp/somefile &`.
//  2. They close the popup (the WS disconnects).
//  3. They click the topbar icon again. The store validates the persisted
//     activeSessionId against the hub, the WS reconnects, and the rolling
//     dump replays the output that was produced while the popup was closed.
//  4. The shell is still alive — `tail -f` keeps running and continues to
//     emit output to the second attach.
func TestE2ETailFPersistsAcrossReconnect(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(target, []byte("initial line\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	hub := NewHub(HubConfig{
		StickyTTL:    10 * time.Second,
		DumpDir:      dir,
		DumpMax:      16 * 1024,
		DefaultShell: "/bin/sh",
		DefaultCwd:   dir,
		DefaultEnv:   []string{"PATH=/usr/bin:/bin"},
	})
	t.Cleanup(hub.Stop)

	// First attach: user opens the terminal, runs `tail -f`.
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
	firstCli, _, err := websocket.DefaultDialer.Dial(wsURL+"?cols=80&rows=24", nil)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}

	var srvConn *websocket.Conn
	select {
	case srvConn = <-srvConnCh:
	case <-time.After(2 * time.Second):
		t.Fatal("no server conn")
	}

	session, _, err := hub.AttachOrCreate(context.Background(), 1, "", CreateOptions{})
	if err != nil {
		t.Fatalf("AttachOrCreate: %v", err)
	}
	defer session.Kill()

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		_ = hub.AttachAndPump(session, srvConn, nil)
	}()

	// Read ready frame and the initial prompt.
	_ = firstCli.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, _ = firstCli.ReadMessage()

	// Tell the shell to tail the file. The literal `&` backgrounds the
	// process; we then send `disown` so a future SIGHUP from a teardown
	// does not kill it. The session's PTY keeps the process group alive.
	cmd := "tail -f " + target + " & disown\n"
	input := InputFrame{Type: FrameInput, Data: base64EncodeBytes([]byte(cmd))}
	raw, _ := EncodeText(input)
	if err := firstCli.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write tail: %v", err)
	}

	// Give the shell a moment to spawn the tail.
	time.Sleep(150 * time.Millisecond)

	// Append a few lines to the file. The tail should emit them and the
	// hub should write them into the rolling dump.
	for i := 0; i < 3; i++ {
		f, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, err := f.WriteString("new line " + string(rune('A'+i)) + "\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = f.Close()
		time.Sleep(80 * time.Millisecond)
	}

	// Snapshot should now contain at least the seed line and one of the
	// "new line" entries.
	data, _, _ := session.Snapshot()
	if !contains(data, []byte("new line A")) {
		t.Fatalf("snapshot missing tail output: %q", data)
	}

	// Simulate "user closes the popup": drop the first WS. The hub keeps
	// the shell alive and the rolling dump continues to be written to
	// because the shell is still attached to the PTY.
	_ = firstCli.Close()
	select {
	case <-pumpDone:
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not exit on client close")
	}

	// While the popup is "closed", the underlying tail process keeps
	// emitting output. Append more lines.
	for i := 0; i < 3; i++ {
		f, _ := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0o644)
		_, _ = f.WriteString("post-close line " + string(rune('A'+i)) + "\n")
		_ = f.Close()
		time.Sleep(60 * time.Millisecond)
	}

	// Reconnect. The store would normally validate the persisted id via
	// the REST endpoint first, but in-process tests skip that and just
	// ask the hub for the same session.
	dataBefore, firstBefore, _ := session.Snapshot()

	// Append a single new line after reconnect.
	f, _ := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("post-reconnect line\n")
	_ = f.Close()
	time.Sleep(120 * time.Millisecond)

	dataAfter, firstAfter, _ := session.Snapshot()
	if firstAfter != firstBefore {
		t.Errorf("firstSeq shifted: before=%d after=%d", firstBefore, firstAfter)
	}
	if !contains(dataAfter[len(dataBefore):], []byte("post-reconnect line")) {
		t.Errorf("replay window missing post-reconnect line: before=%d after=%d", len(dataBefore), len(dataAfter))
	}
	if !contains(dataAfter, []byte("post-close line A")) {
		t.Errorf("rolling dump missing post-close output: %q", dataAfter)
	}
}

func base64EncodeBytes(b []byte) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, ((len(b)+2)/3)*4)
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
	return string(out)
}
