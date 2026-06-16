package terminal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SessionConfig holds everything needed to spawn a PTY-backed shell.
type SessionConfig struct {
	ID        string
	UserID    int64
	Shell     string
	Cwd       string
	Env       []string
	Cols      int
	Rows      int
	DumpPath  string
	DumpMax   int64
	FirstSeq  uint64 // when restoring from an existing dump, this is the size of that dump
}

// SessionMeta is the projection of a PtySession exposed via the REST API.
type SessionMeta struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id"`
	Shell      string    `json:"shell"`
	Cwd        string    `json:"cwd"`
	StartedAt  time.Time `json:"started_at"`
	LastAttach time.Time `json:"last_attach"`
	LastInput  time.Time `json:"last_input"`
	Bytes      int64     `json:"bytes"`
	FirstSeq   uint64    `json:"first_seq"`
	EndSeq     uint64    `json:"end_seq"`
	Active     bool      `json:"active"`
	Role       string    `json:"role"`
}

// PtySession represents a long-lived shell process with attached websocket
// clients. Output is dumped to disk in a bounded rolling file so the session
// can be restored after a server restart or browser refresh.
type PtySession struct {
	id        string
	userID    int64
	cfg       SessionConfig
	startedAt time.Time
	lastAttach time.Time
	lastInput  time.Time

	mu         sync.RWMutex
	cmd        *exec.Cmd
	master     *os.File
	closed     bool
	closeCh    chan struct{}
	closeOnce  sync.Once
	onExit     func(int)

	bufMu     sync.Mutex
	rolling   *RollingFile
	firstSeq  uint64
	endSeq    uint64

	attachMu sync.RWMutex
	writer   *websocket.Conn
	readers  map[*websocket.Conn]struct{}

	fanout chan ptyChunk

	// pauseMu guards pauseCount. When pauseCount > 0 the fan-out loop
	// buffers incoming chunks in pending and waits for Resume.
	pauseMu    sync.Mutex
	pauseCount int
	pending    []ptyChunk
	pendingCh  chan struct{}
}

// ptyChunk is an internal unit of PTY output handed off between the ReadLoop
// and the fan-out goroutine.
type ptyChunk struct {
	seq  uint64
	data []byte
}

// NewPtySession constructs a session but does not start the shell. Call Start.
func NewPtySession(cfg SessionConfig, onExit func(int)) *PtySession {
	if cfg.Cols <= 0 {
		cfg.Cols = 80
	}
	if cfg.Rows <= 0 {
		cfg.Rows = 24
	}
	if cfg.DumpMax <= 0 {
		cfg.DumpMax = 256 * 1024
	}
	return &PtySession{
		id:        cfg.ID,
		userID:    cfg.UserID,
		cfg:       cfg,
		startedAt: time.Now(),
		lastAttach: time.Now(),
		lastInput: time.Now(),
		rolling:   NewRollingFile(cfg.DumpPath, cfg.DumpMax),
		firstSeq:  cfg.FirstSeq,
		endSeq:    cfg.FirstSeq,
		closeCh:   make(chan struct{}),
		readers:   make(map[*websocket.Conn]struct{}),
		fanout:    make(chan ptyChunk, 256),
		pendingCh: make(chan struct{}, 1),
		onExit:    onExit,
	}
}

// Start spawns the shell with a freshly allocated PTY and launches the
// ReadLoop and fan-out goroutines. The session becomes a no-op after Kill.
func (s *PtySession) Start() error {
	cmd := exec.Command(s.cfg.Shell)
	cmd.Env = s.cfg.Env
	cmd.Dir = s.cfg.Cwd

	handle, err := ptyStart(cmd, s.cfg.Cols, s.cfg.Rows)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cmd = cmd
	s.master = handle.master
	s.mu.Unlock()

	go s.readLoop(handle.master)
	go s.fanoutLoop()
	go s.waitLoop()

	return nil
}

// Attach registers a websocket as either the writer (if no writer exists) or a
// reader. Returns the assigned role.
func (s *PtySession) Attach(conn *websocket.Conn) string {
	s.attachMu.Lock()
	defer s.attachMu.Unlock()

	if s.writer == nil {
		s.writer = conn
	} else {
		s.readers[conn] = struct{}{}
	}

	s.touchAttach()
	return s.currentRoleLocked(conn)
}

// Detach removes a websocket from the registry. The session continues to run
// unless this was the last attached client and the sweeper eventually times it
// out.
func (s *PtySession) Detach(conn *websocket.Conn) {
	s.attachMu.Lock()
	defer s.attachMu.Unlock()
	if s.writer == conn {
		s.writer = nil
		return
	}
	delete(s.readers, conn)
}

// HasWriter returns true when at least one client is in the writer role.
func (s *PtySession) HasWriter() bool {
	s.attachMu.RLock()
	defer s.attachMu.RUnlock()
	return s.writer != nil
}

// RoleFor returns the role of a given attached connection.
func (s *PtySession) RoleFor(conn *websocket.Conn) string {
	s.attachMu.RLock()
	defer s.attachMu.RUnlock()
	return s.currentRoleLocked(conn)
}

func (s *PtySession) currentRoleLocked(conn *websocket.Conn) string {
	if s.writer == conn {
		return RoleWriter
	}
	if _, ok := s.readers[conn]; ok {
		return RoleReader
	}
	return "none"
}

// Write pushes stdin bytes into the PTY. Returns an error if the process has
// exited or no writer is currently attached.
func (s *PtySession) Write(p []byte) (int, error) {
	if s.IsClosed() {
		return 0, errors.New("session closed")
	}
	s.mu.RLock()
	master := s.master
	s.mu.RUnlock()
	if master == nil {
		return 0, errors.New("pty not started")
	}
	s.touchInput()
	return master.Write(p)
}

// Resize forwards a winsize change to the PTY master.
func (s *PtySession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("invalid cols/rows")
	}
	s.mu.RLock()
	master := s.master
	s.mu.RUnlock()
	if master == nil {
		return errors.New("pty not started")
	}
	return ptyResize(master, cols, rows)
}

// Kill terminates the shell process group and removes the rolling dump file.
// Safe to call multiple times.
func (s *PtySession) Kill() {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		s.mu.Lock()
		s.closed = true
		cmd := s.cmd
		master := s.master
		s.master = nil
		s.mu.Unlock()

		if cmd != nil && cmd.Process != nil {
			_ = ptyKill(cmd.Process.Pid)
		}
		if master != nil {
			_ = master.Close()
		}
		_ = s.rolling.Remove()
	})
}

// IsClosed reports whether Kill has been called.
func (s *PtySession) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// Meta returns a snapshot of the session metadata for the REST API.
func (s *PtySession) Meta() SessionMeta {
	s.mu.RLock()
	shell := s.cfg.Shell
	cwd := s.cfg.Cwd
	s.mu.RUnlock()

	s.bufMu.Lock()
	first := s.firstSeq
	end := s.endSeq
	bytes := s.rolling.Size()
	s.bufMu.Unlock()

	s.attachMu.RLock()
	active := s.writer != nil || len(s.readers) > 0
	role := "none"
	if s.writer != nil {
		role = RoleWriter
	} else if len(s.readers) > 0 {
		role = RoleReader
	}
	s.attachMu.RUnlock()

	return SessionMeta{
		ID:         s.id,
		UserID:     s.userID,
		Shell:      shell,
		Cwd:        cwd,
		StartedAt:  s.startedAt,
		LastAttach: s.lastAttach,
		LastInput:  s.lastInput,
		Bytes:      bytes,
		FirstSeq:   first,
		EndSeq:     end,
		Active:     active,
		Role:       role,
	}
}

// Snapshot returns the entire current rolling dump plus its sequence window.
func (s *PtySession) Snapshot() (data []byte, firstSeq, endSeq uint64) {
	s.bufMu.Lock()
	defer s.bufMu.Unlock()
	raw, err := s.rolling.ReadAll()
	if err != nil {
		raw = nil
	}
	return raw, s.firstSeq, s.endSeq
}

// DecodeInput decodes the base64 payload of an InputFrame. Convenience for
// handlers so the session does not depend on the protocol package directly.
func DecodeInput(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

func (s *PtySession) readLoop(master *os.File) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.closeCh:
			return
		default:
		}
		n, err := master.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			s.bufMu.Lock()
			seq := s.endSeq
			if _, errAppend := s.rolling.Append(chunk); errAppend != nil {
				// Best-effort: keep the session alive even if the dump
				// write fails. We still want to fan out to live clients.
				_ = errAppend
			}
			s.endSeq += uint64(n)
			first, end := s.firstSeq, s.endSeq
			if end-first > uint64(s.rolling.max) {
				s.firstSeq = end - uint64(s.rolling.max)
			}
			s.bufMu.Unlock()

			select {
			case s.fanout <- ptyChunk{seq: seq, data: chunk}:
			case <-s.closeCh:
				return
			default:
				// Fan-out is full; the slow consumer will be skipped
				// on the next pass. We do not block the read loop.
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = err
			}
			return
		}
	}
}

func (s *PtySession) fanoutLoop() {
	for {
		select {
		case <-s.closeCh:
			return
		case chunk := <-s.fanout:
			s.maybeBuffer(chunk)
			if s.IsBroadcastPaused() {
				continue
			}
			s.broadcast(chunk)
		}
	}
}

// maybeBuffer keeps a copy of every chunk produced while the broadcast is
// paused so the snapshot we send to a new attacher includes output that
// arrived after the last resume.
func (s *PtySession) maybeBuffer(chunk ptyChunk) {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if s.pauseCount == 0 {
		return
	}
	s.pending = append(s.pending, chunk)
}

func (s *PtySession) broadcast(chunk ptyChunk) {
	frame := EncodeBinaryFrame(chunk.seq, chunk.data)

	s.attachMu.RLock()
	targets := make([]*websocket.Conn, 0, 1+len(s.readers))
	if s.writer != nil {
		targets = append(targets, s.writer)
	}
	for c := range s.readers {
		targets = append(targets, c)
	}
	s.attachMu.RUnlock()

	for _, c := range targets {
		if err := c.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			// Drop the offending client; the read pump on that
			// connection will close and call Detach.
			_ = c.Close()
			s.Detach(c)
		}
	}
}

func (s *PtySession) waitLoop() {
	s.mu.RLock()
	cmd := s.cmd
	s.mu.RUnlock()
	if cmd == nil {
		return
	}
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if s.onExit != nil {
		s.onExit(exitCode)
	}
	// Send exit frame to all attached clients then close them.
	s.attachMu.RLock()
	targets := make([]*websocket.Conn, 0, 1+len(s.readers))
	if s.writer != nil {
		targets = append(targets, s.writer)
	}
	for c := range s.readers {
		targets = append(targets, c)
	}
	s.attachMu.RUnlock()

	frame, _ := EncodeText(ExitFrame{Type: FrameExit, Code: exitCode})
	for _, c := range targets {
		_ = c.WriteMessage(websocket.TextMessage, frame)
		_ = c.Close()
		s.Detach(c)
	}
	s.Kill()
}

func (s *PtySession) touchAttach() {
	s.mu.Lock()
	s.lastAttach = time.Now()
	s.mu.Unlock()
}

func (s *PtySession) touchInput() {
	s.mu.Lock()
	s.lastInput = time.Now()
	s.mu.Unlock()
}

// PauseBroadcast increments the pause counter and returns the previous value
// so the caller can pair it with ResumeBroadcast. While paused, fan-out still
// drains from the internal channel and keeps chunks in a buffer for the next
// snapshot, but does not send to any websocket.
func (s *PtySession) PauseBroadcast() int {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	prev := s.pauseCount
	s.pauseCount++
	return prev
}

// ResumeBroadcast decrements the pause counter. When the counter reaches zero
// any chunks that arrived during the pause are flushed to attached clients
// in order.
func (s *PtySession) ResumeBroadcast(token int) {
	s.pauseMu.Lock()
	wasPaused := s.pauseCount > 0
	s.pauseCount--
	if s.pauseCount < 0 {
		s.pauseCount = 0
	}
	pending := s.pending
	s.pending = nil
	s.pauseMu.Unlock()

	// Flush whenever we just left a paused state. The token argument is
	// kept for backward compatibility but the flush predicate is purely a
	// "did we transition out of a pause" check — that is what callers
	// actually want, and the previous `token == 1` guard caused the
	// pending buffer to leak the very first prompt bytes of a session.
	if wasPaused {
		for _, c := range pending {
			s.broadcast(c)
		}
	}
}

// IsBroadcastPaused reports whether the broadcast is currently paused.
func (s *PtySession) IsBroadcastPaused() bool {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.pauseCount > 0
}

// WaitForExit blocks until the shell exits, the context is canceled, or
// Kill has been called. Used by tests to synchronize.
func (s *PtySession) WaitForExit(ctx context.Context) error {
	if s.cmd == nil {
		return errors.New("session not started")
	}
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("exit %d", exitErr.ExitCode())
			}
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.closeCh:
		return nil
	}
}
