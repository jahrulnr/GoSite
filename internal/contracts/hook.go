package contracts

import (
	"context"
	"time"
)

// HookCallResult describes one plugin hook invocation.
type HookCallResult struct {
	PluginID string        `json:"plugin_id"`
	Version  string        `json:"version"`
	Status   string        `json:"status"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// HookResult describes a full host event dispatch.
type HookResult struct {
	EventName string           `json:"event_name"`
	Strict    bool             `json:"strict"`
	Calls     []HookCallResult `json:"calls"`
	Warnings  []string         `json:"warnings,omitempty"`
}

// HookBus dispatches host lifecycle events to enabled plugins.
type HookBus interface {
	Dispatch(ctx context.Context, eventName string, payload any) (HookResult, error)
}

// NoopHookBus accepts all hook events without side effects.
type NoopHookBus struct{}

func (NoopHookBus) Dispatch(context.Context, string, any) (HookResult, error) {
	return HookResult{}, nil
}
