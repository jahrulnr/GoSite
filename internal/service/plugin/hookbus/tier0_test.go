package hookbus_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTier0CallerDispatchesToDeclaredWebhook(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		assert.Equal(t, "notify", r.Header.Get("X-Gosite-Webhook-Event"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	caller := hookbus.NewTier0Caller(time.Second, "shared-secret")
	target := sqlite.PluginVersion{
		PluginID: "acme/logger",
		Version:  "1.0.0",
		ManifestJSON: `{"webhooks":[{"event":"notify","url":"` + srv.URL + `","method":"POST"}]}`,
	}

	hookTarget := hookbus.NewTargetForTest(target)
	payload := json.RawMessage(`{"hello":"world"}`)
	out, err := caller.CallHook(context.Background(), hookTarget, "notify", payload)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}
