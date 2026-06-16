package hookbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusSkipped = "skipped"
)

// Target is one enabled plugin hook target.
type Target struct {
	PluginID     string
	Version      string
	Tier         int
	ManifestJSON string
}

// HookCaller invokes one concrete plugin target.
type HookCaller interface {
	CallHook(ctx context.Context, target Target, eventName string, payloadJSON json.RawMessage) (json.RawMessage, error)
}

// NoopCaller treats dispatch as successful until real runtimes are attached.
type NoopCaller struct{}

func (NoopCaller) CallHook(context.Context, Target, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// Config controls hook dispatch behavior.
type Config struct {
	MaxConcurrentHooks int
	HookTimeout        time.Duration
	StrictEvents       map[string]bool
	BreakerThreshold   int
	BreakerWindow      time.Duration
	Caller             HookCaller
	Audit              contracts.AuditWriter
}

// Dispatcher stores enabled plugins and dispatches lifecycle events.
type Dispatcher struct {
	mu       sync.RWMutex
	enabled  map[string]Target
	breakers map[string]breakerState
	cfg      Config
}

type breakerState struct {
	failures int
	firstAt  time.Time
}

type manifestSnapshot struct {
	Capabilities struct {
		Hooks         []string `json:"hooks"`
		HookIsolation string   `json:"hookIsolation"`
	} `json:"capabilities"`
}

// New returns a hook dispatcher with deterministic defaults.
func New(cfg Config) *Dispatcher {
	if cfg.MaxConcurrentHooks <= 0 {
		cfg.MaxConcurrentHooks = 10
	}
	if cfg.HookTimeout <= 0 {
		cfg.HookTimeout = 5 * time.Second
	}
	if cfg.BreakerThreshold <= 0 {
		cfg.BreakerThreshold = 3
	}
	if cfg.BreakerWindow <= 0 {
		cfg.BreakerWindow = 5 * time.Minute
	}
	if cfg.Caller == nil {
		cfg.Caller = NoopCaller{}
	}
	return &Dispatcher{
		enabled:  map[string]Target{},
		breakers: map[string]breakerState{},
		cfg:      cfg,
	}
}

// Refresh replaces the enabled set used by lifecycle and hook dispatch.
func (d *Dispatcher) Refresh(_ context.Context, plugins []sqlite.PluginVersion) error {
	next := make(map[string]Target, len(plugins))
	for _, plugin := range plugins {
		next[plugin.PluginID] = Target{
			PluginID:     plugin.PluginID,
			Version:      plugin.Version,
			Tier:         plugin.Tier,
			ManifestJSON: plugin.ManifestJSON,
		}
	}
	d.mu.Lock()
	d.enabled = next
	d.mu.Unlock()
	return nil
}

// SetCaller swaps the underlying HookCaller. Used by the host after the
// runtime manager is constructed so the dispatcher can route events to
// HashiCorp go-plugin clients. Caller is safe to swap at runtime.
func (d *Dispatcher) SetCaller(caller HookCaller) {
	d.mu.Lock()
	if caller == nil {
		d.cfg.Caller = NoopCaller{}
	} else {
		d.cfg.Caller = caller
	}
	d.mu.Unlock()
}

// Disable removes one plugin from the enabled set.
func (d *Dispatcher) Disable(_ context.Context, plugin sqlite.PluginVersion) error {
	d.mu.Lock()
	delete(d.enabled, plugin.PluginID)
	d.mu.Unlock()
	return nil
}

// Dispatch calls matching enabled plugin hooks in deterministic order.
func (d *Dispatcher) Dispatch(ctx context.Context, eventName string, payload any) (contracts.HookResult, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return contracts.HookResult{}, apperror.Wrap(apperror.CodeInvalidInput, "encode hook payload failed", err)
	}
	targets := d.targetsFor(eventName)
	strict := d.isStrict(eventName)
	result := contracts.HookResult{EventName: eventName, Strict: strict}
	if strict {
		for _, target := range targets {
			call := d.callOne(ctx, target, eventName, payloadJSON)
			result.Calls = append(result.Calls, call)
			if call.Status == StatusError {
				return result, apperror.New(apperror.CodePluginOperation, fmt.Sprintf("plugin hook %s blocked by %s@%s: %s", eventName, call.PluginID, call.Version, call.Error))
			}
		}
		return result, nil
	}

	sequential, independent := splitByIsolation(targets)
	for _, target := range sequential {
		call := d.callOne(ctx, target, eventName, payloadJSON)
		result.Calls = append(result.Calls, call)
		if call.Status == StatusError {
			result.Warnings = append(result.Warnings, call.PluginID+"@"+call.Version+": "+call.Error)
		}
	}
	concurrent := d.callIndependent(ctx, independent, eventName, payloadJSON)
	result.Calls = append(result.Calls, concurrent...)
	sort.SliceStable(result.Calls, func(i, j int) bool {
		if result.Calls[i].PluginID == result.Calls[j].PluginID {
			return compareSemver(result.Calls[i].Version, result.Calls[j].Version) > 0
		}
		return result.Calls[i].PluginID < result.Calls[j].PluginID
	})
	for _, call := range concurrent {
		if call.Status == StatusError {
			result.Warnings = append(result.Warnings, call.PluginID+"@"+call.Version+": "+call.Error)
		}
	}
	return result, nil
}

// Snapshot returns enabled targets for tests and diagnostics.
func (d *Dispatcher) Snapshot() []Target {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Target, 0, len(d.enabled))
	for _, target := range d.enabled {
		out = append(out, target)
	}
	sortTargets(out)
	return out
}

func (d *Dispatcher) targetsFor(eventName string) []Target {
	d.mu.RLock()
	targets := make([]Target, 0, len(d.enabled))
	for _, target := range d.enabled {
		if declaresHook(target, eventName) {
			targets = append(targets, target)
		}
	}
	d.mu.RUnlock()
	sortTargets(targets)
	return targets
}

func (d *Dispatcher) isStrict(eventName string) bool {
	if d.cfg.StrictEvents != nil {
		if strict, ok := d.cfg.StrictEvents[eventName]; ok {
			return strict
		}
	}
	return strings.Contains(eventName, ".before_")
}

func (d *Dispatcher) callIndependent(ctx context.Context, targets []Target, eventName string, payloadJSON json.RawMessage) []contracts.HookCallResult {
	if len(targets) == 0 {
		return nil
	}
	limit := d.cfg.MaxConcurrentHooks
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	out := make([]contracts.HookCallResult, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = d.callOne(ctx, target, eventName, payloadJSON)
		}(i, target)
	}
	wg.Wait()
	return out
}

func (d *Dispatcher) callOne(ctx context.Context, target Target, eventName string, payloadJSON json.RawMessage) contracts.HookCallResult {
	start := time.Now()
	call := contracts.HookCallResult{PluginID: target.PluginID, Version: target.Version}
	if d.breakerOpen(target) {
		call.Status = StatusSkipped
		call.Error = "circuit breaker open"
		call.Duration = time.Since(start)
		return call
	}
	callCtx, cancel := context.WithTimeout(ctx, d.cfg.HookTimeout)
	defer cancel()
	_, err := d.cfg.Caller.CallHook(callCtx, target, eventName, payloadJSON)
	call.Duration = time.Since(start)
	if err != nil {
		call.Status = StatusError
		call.Error = err.Error()
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			call.Error = "hook timeout"
		}
		d.recordFailure(target)
		d.auditError(ctx, eventName, call)
		return call
	}
	call.Status = StatusOK
	d.recordSuccess(target)
	return call
}

func (d *Dispatcher) breakerOpen(target Target) bool {
	key := target.PluginID + "@" + target.Version
	d.mu.RLock()
	state := d.breakers[key]
	d.mu.RUnlock()
	if state.failures < d.cfg.BreakerThreshold {
		return false
	}
	if time.Since(state.firstAt) <= d.cfg.BreakerWindow {
		return true
	}
	d.mu.Lock()
	delete(d.breakers, key)
	d.mu.Unlock()
	return false
}

func (d *Dispatcher) recordFailure(target Target) {
	key := target.PluginID + "@" + target.Version
	now := time.Now()
	d.mu.Lock()
	state := d.breakers[key]
	if state.firstAt.IsZero() || now.Sub(state.firstAt) > d.cfg.BreakerWindow {
		state = breakerState{firstAt: now}
	}
	state.failures++
	d.breakers[key] = state
	d.mu.Unlock()
}

func (d *Dispatcher) recordSuccess(target Target) {
	key := target.PluginID + "@" + target.Version
	d.mu.Lock()
	delete(d.breakers, key)
	d.mu.Unlock()
}

func (d *Dispatcher) auditError(ctx context.Context, eventName string, call contracts.HookCallResult) {
	if d.cfg.Audit == nil {
		return
	}
	meta, _ := json.Marshal(map[string]string{
		"event":     eventName,
		"plugin_id": call.PluginID,
		"version":   call.Version,
	})
	_ = d.cfg.Audit.Write(ctx, contracts.AuditEntry{
		Timestamp:    time.Now().UTC(),
		Action:       "plugin.hook_error",
		ResourceType: "plugin",
		ResourceID:   call.PluginID,
		Status:       "failed",
		Message:      call.Error,
		MetaJSON:     string(meta),
	})
}

func splitByIsolation(targets []Target) ([]Target, []Target) {
	sequential := make([]Target, 0, len(targets))
	independent := make([]Target, 0, len(targets))
	for _, target := range targets {
		if hookIsolation(target) == "independent" {
			independent = append(independent, target)
			continue
		}
		sequential = append(sequential, target)
	}
	return sequential, independent
}

func declaresHook(target Target, eventName string) bool {
	var manifest manifestSnapshot
	if err := json.Unmarshal([]byte(target.ManifestJSON), &manifest); err != nil {
		return false
	}
	for _, hook := range manifest.Capabilities.Hooks {
		if hook == eventName {
			return true
		}
	}
	return false
}

func hookIsolation(target Target) string {
	var manifest manifestSnapshot
	if err := json.Unmarshal([]byte(target.ManifestJSON), &manifest); err != nil {
		return "sequential"
	}
	switch manifest.Capabilities.HookIsolation {
	case "independent":
		return "independent"
	default:
		return "sequential"
	}
}

func sortTargets(targets []Target) {
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].PluginID == targets[j].PluginID {
			return compareSemver(targets[i].Version, targets[j].Version) > 0
		}
		return targets[i].PluginID < targets[j].PluginID
	})
}

func compareSemver(a, b string) int {
	ap := semverParts(a)
	bp := semverParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func semverParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.Split(v, "-")[0]
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		var n int
		fmt.Sscanf(parts[i], "%d", &n)
		out[i] = n
	}
	return out
}
