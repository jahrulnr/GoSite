package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
)

// Hub owns the in-memory PtySession registry. It enforces a per-user cap, a
// sticky timeout (default 12h), and a sweeper that terminates sessions whose
// activity window has elapsed.
type Hub struct {
	mu        sync.RWMutex
	sessions  map[string]*PtySession
	byUser    map[int64]map[string]*PtySession
	perUser   int
	stickyTTL time.Duration
	dumpDir   string
	dumpMax   int64
	stopCh    chan struct{}
	idGen     func() string
	defaultShell string
	defaultCwd   string
	defaultEnv   []string
}

// HubConfig is the constructor input for Hub.
type HubConfig struct {
	StickyTTL    time.Duration
	DumpDir      string
	DumpMax      int64
	PerUser      int
	DefaultShell string
	DefaultCwd   string
	DefaultEnv   []string
}

// NewHub builds a Hub and returns a started sweeper goroutine via Run.
func NewHub(cfg HubConfig) *Hub {
	if cfg.StickyTTL <= 0 {
		cfg.StickyTTL = 12 * time.Hour
	}
	if cfg.DumpMax <= 0 {
		cfg.DumpMax = 256 * 1024
	}
	if cfg.PerUser <= 0 {
		cfg.PerUser = 8
	}
	if cfg.DumpDir == "" {
		cfg.DumpDir = "/tmp"
	}
	if cfg.DefaultShell == "" {
		cfg.DefaultShell = "/bin/bash"
	}
	if cfg.DefaultCwd == "" {
		cfg.DefaultCwd = "/storage"
	}
	if cfg.DefaultEnv == nil {
		cfg.DefaultEnv = []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"LANG=C.UTF-8",
		}
	}
	return &Hub{
		sessions:     make(map[string]*PtySession),
		byUser:       make(map[int64]map[string]*PtySession),
		perUser:      cfg.PerUser,
		stickyTTL:    cfg.StickyTTL,
		dumpDir:      cfg.DumpDir,
		dumpMax:      cfg.DumpMax,
		stopCh:       make(chan struct{}),
		idGen:        func() string { return ulid.Make().String() },
		defaultShell: cfg.DefaultShell,
		defaultCwd:   cfg.DefaultCwd,
		defaultEnv:   cfg.DefaultEnv,
	}
}

// StickyTTL returns the configured sticky timeout.
func (h *Hub) StickyTTL() time.Duration { return h.stickyTTL }

// DumpDir returns the configured dump directory.
func (h *Hub) DumpDir() string { return h.dumpDir }

// NewID generates a fresh session id. Exported so the handler can pre-allocate
// ids for clients that want a stable handle before opening a websocket.
func (h *Hub) NewID() string { return h.idGen() }

// Create spawns a new PtySession owned by userID. Returns an error if the
// per-user cap is exceeded or the shell cannot be started.
func (h *Hub) Create(ctx context.Context, userID int64, opts CreateOptions) (*PtySession, error) {
	h.mu.Lock()
	if user, ok := h.byUser[userID]; ok && len(user) >= h.perUser {
		h.mu.Unlock()
		return nil, fmt.Errorf("per-user session cap reached (%d)", h.perUser)
	}
	h.mu.Unlock()

	id := opts.SessionID
	if id == "" {
		id = h.idGen()
	}
	cols, rows := opts.Cols, opts.Rows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	shell := opts.Shell
	if shell == "" {
		shell = h.defaultShell
	}
	cwd := opts.Cwd
	if cwd == "" {
		cwd = h.defaultCwd
	}
	env := opts.Env
	if env == nil {
		env = h.defaultEnv
	}

	dumpPath := DumpPathFor(h.dumpDir, id)
	firstSeq := uint64(0)
	if info, err := os.Stat(dumpPath); err == nil {
		// Restore from an existing dump if the in-memory registry was
		// cleared (e.g. server restart) but the dump file persisted on a
		// host-mounted volume.
		firstSeq = uint64(info.Size())
	}

	cfg := SessionConfig{
		ID:        id,
		UserID:    userID,
		Shell:     shell,
		Cwd:       cwd,
		Env:       env,
		Cols:      cols,
		Rows:      rows,
		DumpPath:  dumpPath,
		DumpMax:   h.dumpMax,
		FirstSeq:  firstSeq,
	}

	s := NewPtySession(cfg, func(_ int) {
		h.unregister(id)
	})
	if err := s.Start(); err != nil {
		return nil, err
	}

	h.register(s)
	return s, nil
}

// CreateOptions customizes a Create call. All fields are optional and fall back
// to the Hub defaults.
type CreateOptions struct {
	SessionID string
	Shell     string
	Cwd       string
	Env       []string
	Cols      int
	Rows      int
}

// Get returns the session by id, or false if it is not registered.
func (h *Hub) Get(id string) (*PtySession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.sessions[id]
	return s, ok
}

// AttachOrCreate either returns the existing session (in-memory or via dump
// restore) or spawns a new one. If sessionID is empty a new session is
// always created.
func (h *Hub) AttachOrCreate(ctx context.Context, userID int64, sessionID string, opts CreateOptions) (*PtySession, string, error) {
	if sessionID != "" {
		if s, ok := h.Get(sessionID); ok {
			h.mu.RLock()
			owner := s.userID
			h.mu.RUnlock()
			if owner != userID {
				return nil, "", errors.New("session belongs to another user")
			}
			return s, sessionID, nil
		}
		// Try to restore from dump.
		dumpPath := DumpPathFor(h.dumpDir, sessionID)
		if info, err := os.Stat(dumpPath); err == nil && info.Size() > 0 {
			opts.SessionID = sessionID
			s, err := h.Create(ctx, userID, opts)
			return s, sessionID, err
		}
		// Fall through: dump missing, spawn a new session with the same id
		// so the client gets a stable handle (its old dump was already
		// gone, so the old handle is effectively expired).
		opts.SessionID = sessionID
	}
	s, err := h.Create(ctx, userID, opts)
	if err != nil {
		return nil, "", err
	}
	return s, s.id, nil
}

// ListByUser returns metadata for every active session owned by userID.
func (h *Hub) ListByUser(userID int64) []SessionMeta {
	h.mu.RLock()
	user, ok := h.byUser[userID]
	if !ok {
		h.mu.RUnlock()
		return nil
	}
	out := make([]SessionMeta, 0, len(user))
	for _, s := range user {
		out = append(out, s.Meta())
	}
	h.mu.RUnlock()
	return out
}

// Kill terminates a session by id and removes its rolling dump. Returns
// ErrSessionNotFound when the id is unknown.
func (h *Hub) Kill(id string) error {
	h.mu.RLock()
	s, ok := h.sessions[id]
	h.mu.RUnlock()
	if !ok {
		return ErrSessionNotFound
	}
	s.Kill()
	// unregister is invoked by the onExit callback; ensure the bookkeeping
	// is consistent even if the exit goroutine races.
	h.unregister(id)
	return nil
}

// ErrSessionNotFound is returned by Kill when the session id is unknown.
var ErrSessionNotFound = errors.New("terminal session not found")

func (h *Hub) register(s *PtySession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[s.id] = s
	if _, ok := h.byUser[s.userID]; !ok {
		h.byUser[s.userID] = make(map[string]*PtySession)
	}
	h.byUser[s.userID][s.id] = s
}

func (h *Hub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[id]
	if !ok {
		return
	}
	delete(h.sessions, id)
	if user, ok := h.byUser[s.userID]; ok {
		delete(user, id)
		if len(user) == 0 {
			delete(h.byUser, s.userID)
		}
	}
}

// RunSweeper launches a background goroutine that kills sessions whose
// inactivity window has elapsed. Returns immediately; the goroutine runs
// until ctx is canceled.
func (h *Hub) RunSweeper(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-h.stopCh:
				return
			case <-ticker.C:
				h.sweep()
			}
		}
	}()
}

// Stop terminates the sweeper goroutine. Idempotent.
func (h *Hub) Stop() {
	select {
	case <-h.stopCh:
	default:
		close(h.stopCh)
	}
}

func (h *Hub) sweep() {
	now := time.Now()
	var toKill []string
	h.mu.RLock()
	for id, s := range h.sessions {
		meta := s.Meta()
		if now.Sub(meta.LastInput) > h.stickyTTL {
			toKill = append(toKill, id)
			continue
		}
		if !s.HasWriter() && now.Sub(meta.LastAttach) > h.stickyTTL {
			toKill = append(toKill, id)
		}
	}
	h.mu.RUnlock()

	for _, id := range toKill {
		_ = h.Kill(id)
	}
}

// EnsureDumpDir creates the dump directory if it does not exist.
func (h *Hub) EnsureDumpDir() error {
	return os.MkdirAll(h.dumpDir, 0o755)
}

// AttachAndPump is a convenience wrapper that takes a websocket.Conn, attaches
// it to the session, sends a ReadyFrame with the current snapshot, then runs
// the bidirectional pump until the socket closes. The session is detached
// before returning. Errors are returned to the caller for logging; the
// underlying connection is always closed.
//
// This is invoked from the WS handler and is the only place where the session
// talks to the websocket layer.
func (h *Hub) AttachAndPump(s *PtySession, conn *websocket.Conn, onEvent func(string, map[string]interface{})) error {
	if s == nil {
		return errors.New("nil session")
	}

	// Pause the broadcast loop while we send the snapshot so the first
	// message the client receives is always the ReadyFrame.
	paused := s.PauseBroadcast()
	role := s.Attach(conn)
	defer func() {
		s.ResumeBroadcast(paused)
		s.Detach(conn)
	}()

	if onEvent != nil {
		onEvent("open", map[string]interface{}{
			"session_id": s.id,
			"role":       role,
		})
	}

	// Build and send ReadyFrame with snapshot.
	data, firstSeq, endSeq := s.Snapshot()
	cols, rows := s.cfg.Cols, s.cfg.Rows
	ready := ReadyFrame{
		Type:          FrameReady,
		SessionID:     s.id,
		Shell:         s.cfg.Shell,
		Cwd:           s.cfg.Cwd,
		Cols:          cols,
		Rows:          rows,
		Role:          role,
		BufferedBytes: int64(len(data)),
		FirstSeq:      firstSeq,
		EndSeq:        endSeq,
		StickyTTL:     h.stickyTTL.String(),
		StartedAt:     s.startedAt.Format(time.RFC3339Nano),
	}
	readyBytes, err := EncodeText(ready)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, readyBytes); err != nil {
		return err
	}

	// Replay snapshot as a single binary frame so the client can render the
	// entire rolling dump before live output starts. We tag the replay with
	// firstSeq so the client's dedup is consistent.
	if len(data) > 0 {
		if err := conn.WriteMessage(websocket.BinaryMessage, EncodeBinaryFrame(firstSeq, data)); err != nil {
			return err
		}
	}

	// Resume the broadcast loop now that the snapshot has been delivered;
	// further output from the PTY should be forwarded to the client in
	// real time. The deferred resume is a no-op because the counter is
	// already at zero.
	s.ResumeBroadcast(paused)

	// Write pump: forward frames to the session. This goroutine is the
	// sole reader of the websocket, satisfying gorilla's "one reader at a
	// time" contract.
	writeErr := make(chan error, 1)
	go func() {
		for {
			msgType, raw, err := conn.ReadMessage()
			if err != nil {
				writeErr <- err
				return
			}
			if msgType != websocket.TextMessage {
				continue
			}
			payload, err := DecodeText(raw)
			if err != nil {
				continue
			}
			switch p := payload.(type) {
			case *InputFrame:
				if role != RoleWriter {
					continue
				}
				decoded, err := base64Decode(p.Data)
				if err != nil {
					continue
				}
				if _, err := s.Write(decoded); err != nil {
					writeErr <- err
					return
				}
			case *ResizeFrame:
				_ = s.Resize(p.Cols, p.Rows)
			case *PingFrame:
				pong, _ := EncodeText(PongFrame{Type: FramePong})
				_ = conn.WriteMessage(websocket.TextMessage, pong)
			case *ReplayFrame:
				data, first, end := s.Snapshot()
				_ = first
				_ = end
				if len(data) > 0 {
					_ = conn.WriteMessage(websocket.BinaryMessage, EncodeBinaryFrame(first, data))
				}
			}
		}
	}()

	// Single reader: writeErr is the source of truth for both directions.
	select {
	case err := <-writeErr:
		return err
	}
}

// base64Decode is split out to avoid importing encoding/base64 in hub.go
// (the heavy import would otherwise also live in session.go via DecodeInput).
func base64Decode(s string) ([]byte, error) {
	return DecodeInput(s)
}

// Count returns the number of live sessions.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// DumpFileFor returns the absolute path to the rolling dump file for a given
// session id. Used by the snapshot endpoint to stream the file content.
func (h *Hub) DumpFileFor(id string) string {
	return DumpPathFor(h.dumpDir, id)
}

// ResolvePath is a small helper to keep dumpDir absolute. It exists mainly
// so test setups can rely on a t.TempDir() inside the Hub.
func (h *Hub) ResolvePath(id string) string {
	return filepath.Join(h.dumpDir, "gosite-term-"+id+".log")
}
