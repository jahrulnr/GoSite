package hookbus

import (
	"context"
	"sync"
)

// LoggingSink routes structured host events to all enabled plugins that
// declared capabilities.loggingSink. The sink is independent of the
// per-event Dispatch path so it does not interact with the strict/lenient
// rules: log events are always best-effort.
type LoggingSink struct {
	mu     sync.RWMutex
	caller HookCaller
}

// NewLoggingSink returns an empty sink bound to the supplied caller.
func NewLoggingSink(caller HookCaller) *LoggingSink {
	return &LoggingSink{caller: caller}
}

// SetCaller swaps the underlying caller. Safe for concurrent use.
func (s *LoggingSink) SetCaller(caller HookCaller) {
	s.mu.Lock()
	if caller == nil {
		s.caller = NoopCaller{}
	} else {
		s.caller = caller
	}
	s.mu.Unlock()
}

// Emit dispatches a logging.on_event to every enabled target that
// declared the loggingSink capability. Errors are returned per target
// without aborting the remaining dispatches.
func (s *LoggingSink) Emit(ctx context.Context, targets []Target, event string, payload []byte) {
	s.mu.RLock()
	caller := s.caller
	s.mu.RUnlock()
	for _, target := range targets {
		_, _ = caller.CallHook(ctx, target, event, payload)
	}
}
