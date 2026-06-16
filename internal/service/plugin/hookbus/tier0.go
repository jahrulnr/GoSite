package hookbus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Tier0Caller is the HTTP webhooks hook implementation for tier-0
// plugins. It dispatches host events to the URLs declared in the plugin
// manifest's webhooks array. Tier-1 plugins are still routed through the
// HashiCorp go-plugin adapter.
type Tier0Caller struct {
	mu     sync.RWMutex
	client *http.Client
	secret string
}

// NewTier0Caller returns a tier-0 caller with the supplied HTTP timeout.
// secret is sent as a header (X-Gosite-Webhook-Secret) so tier-0 plugins
// can verify the request origin.
func NewTier0Caller(timeout time.Duration, secret string) *Tier0Caller {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Tier0Caller{
		client: &http.Client{Timeout: timeout},
		secret: secret,
	}
}

// CallHook dispatches an event to the matching tier-0 webhook target. If
// the manifest declares no webhooks for this event the call is a no-op
// success.
func (c *Tier0Caller) CallHook(ctx context.Context, target Target, eventName string, payload json.RawMessage) (json.RawMessage, error) {
	for _, hook := range targetWebhookTargets(target) {
		if hook.Event != eventName {
			continue
		}
		if err := c.dispatch(ctx, hook, eventName, payload); err != nil {
			return nil, err
		}
	}
	return json.RawMessage("{}"), nil
}

func (c *Tier0Caller) dispatch(ctx context.Context, hook Tier0Webhook, eventName string, payload json.RawMessage) error {
	method := strings.ToUpper(strings.TrimSpace(hook.Method))
	if method == "" {
		method = http.MethodPost
	}
	body, err := json.Marshal(map[string]any{
		"event":   eventName,
		"plugin":  hook.URL, // placeholder, caller is expected to know its own URL
		"payload": json.RawMessage(payload),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, hook.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gosite-Webhook-Event", eventName)
	if c.secret != "" {
		req.Header.Set("X-Gosite-Webhook-Secret", c.secret)
	}
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("tier-0 webhook returned %d", resp.StatusCode)
	}
	return nil
}

// targetWebhookTargets extracts declared tier-0 webhooks from the
// manifest snapshot of a target.
func targetWebhookTargets(target Target) []Tier0Webhook {
	var manifest manifestSnapshot
	if err := json.Unmarshal([]byte(target.ManifestJSON), &manifest); err != nil {
		return nil
	}
	return manifest.Webhooks
}
