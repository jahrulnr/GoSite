package plugin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// HealthSupervisor periodically pings enabled tier-1 plugins and
// restarts (or disables) them when health fails. It is the runtime
// counterpart of Reconcile: Reconcile enforces registry truth at
// startup; Supervisor enforces runtime truth during steady-state.
type HealthSupervisor struct {
	repo        *sqlite.PluginRepository
	runtime     RuntimeManager
	health      HealthChecker
	interval    time.Duration
	maxAttempts int
	window      time.Duration
	backoffMin  time.Duration
	backoffMax  time.Duration

	mu        sync.Mutex
	failures  map[string]*failureWindow
}

// HealthChecker reports the health of a single running plugin. The
// default implementation calls pluginrpc.Plugin.Health via the runtime
// manager; tests can swap in a fake.
type HealthChecker interface {
	Health(ctx context.Context, plugin sqlite.PluginVersion) error
}

// SupervisorOption configures a HealthSupervisor.
type SupervisorOption func(*HealthSupervisor)

// WithSupervisorInterval overrides the health check interval.
func WithSupervisorInterval(d time.Duration) SupervisorOption {
	return func(s *HealthSupervisor) {
		if d > 0 {
			s.interval = d
		}
	}
}

// WithSupervisorMaxAttempts sets the number of consecutive failures
// (within the configured window) after which the plugin is auto-disabled.
func WithSupervisorMaxAttempts(n int) SupervisorOption {
	return func(s *HealthSupervisor) {
		if n > 0 {
			s.maxAttempts = n
		}
	}
}

// WithSupervisorWindow sets the rolling window for failure counting.
func WithSupervisorWindow(d time.Duration) SupervisorOption {
	return func(s *HealthSupervisor) {
		if d > 0 {
			s.window = d
		}
	}
}

// WithSupervisorBackoff sets the restart backoff bounds.
func WithSupervisorBackoff(min, max time.Duration) SupervisorOption {
	return func(s *HealthSupervisor) {
		if min > 0 {
			s.backoffMin = min
		}
		if max > 0 {
			s.backoffMax = max
		}
	}
}

// NewHealthSupervisor returns a supervisor configured with sensible
// defaults. Defaults match docs/sequences/19-plugin-installer.md:
//
//	healthCheckInterval=30s, restartMaxAttempts=5, restartWindow=10m,
//	restartBackoffInitial=1s, restartBackoffCap=2m.
func NewHealthSupervisor(repo *sqlite.PluginRepository, runtime RuntimeManager, health HealthChecker, opts ...SupervisorOption) *HealthSupervisor {
	s := &HealthSupervisor{
		repo:        repo,
		runtime:     runtime,
		health:      health,
		interval:    30 * time.Second,
		maxAttempts: 5,
		window:      10 * time.Minute,
		backoffMin:  1 * time.Second,
		backoffMax:  2 * time.Minute,
		failures:    map[string]*failureWindow{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type failureWindow struct {
	firstAt  time.Time
	count    int
	attempts int
}

// Run blocks until ctx is canceled, polling the registry for enabled
// plugins and verifying their health. Intended to be called in a
// dedicated goroutine at startup.
func (s *HealthSupervisor) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *HealthSupervisor) tick(ctx context.Context) {
	enabled, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, plugin := range enabled {
		key := plugin.PluginID + "@" + plugin.Version
		if err := s.health.Health(ctx, plugin); err != nil {
			s.recordFailure(ctx, key, plugin, err)
			continue
		}
		s.recordSuccess(key)
	}
}

func (s *HealthSupervisor) recordFailure(ctx context.Context, key string, plugin sqlite.PluginVersion, err error) {
	s.mu.Lock()
	state := s.failures[key]
	now := time.Now()
	if state == nil || now.Sub(state.firstAt) > s.window {
		state = &failureWindow{firstAt: now}
	}
	state.count++
	state.attempts++
	s.failures[key] = state
	s.mu.Unlock()
	if state.count >= s.maxAttempts {
		_ = s.disableAfterRepeatedFailure(ctx, plugin, err)
		return
	}
	if err := s.restartWithBackoff(ctx, plugin, state.attempts); err != nil {
		_ = s.markRestartFailed(ctx, plugin, err)
	}
}

func (s *HealthSupervisor) recordSuccess(key string) {
	s.mu.Lock()
	delete(s.failures, key)
	s.mu.Unlock()
}

func (s *HealthSupervisor) restartWithBackoff(ctx context.Context, plugin sqlite.PluginVersion, attempt int) error {
	delay := s.backoffMin
	if attempt > 1 {
		delay = s.backoffMin * time.Duration(1<<(attempt-1))
	}
	if delay > s.backoffMax {
		delay = s.backoffMax
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}
	if err := s.runtime.Stop(ctx, plugin); err != nil {
		return err
	}
	return s.runtime.Start(ctx, plugin)
}

func (s *HealthSupervisor) disableAfterRepeatedFailure(ctx context.Context, plugin sqlite.PluginVersion, cause error) error {
	if _, err := s.repo.SetFailure(ctx, plugin.PluginID, plugin.Version, sqlite.PluginStateInstalled, FailureHookRefreshFailed, fmt.Sprintf("auto-disabled after %d health failures: %v", s.maxAttempts, cause)); err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "auto-disable plugin failed", err)
	}
	if err := s.runtime.Stop(ctx, plugin); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func (s *HealthSupervisor) markRestartFailed(ctx context.Context, plugin sqlite.PluginVersion, err error) error {
	_, dbErr := s.repo.SetFailure(ctx, plugin.PluginID, plugin.Version, sqlite.PluginStateEnableFailed, FailureStartFailed, err.Error())
	return dbErr
}
