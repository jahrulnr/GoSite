package plugin

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeTracker struct {
	mu       sync.Mutex
	starts   []string
	stops    []string
	stopErr  error
	startErr error
}

func (r *runtimeTracker) Start(_ context.Context, p sqlite.PluginVersion) error {
	r.mu.Lock()
	r.starts = append(r.starts, p.PluginID+"@"+p.Version)
	r.mu.Unlock()
	return r.startErr
}

func (r *runtimeTracker) Stop(_ context.Context, p sqlite.PluginVersion) error {
	r.mu.Lock()
	r.stops = append(r.stops, p.PluginID+"@"+p.Version)
	r.mu.Unlock()
	return r.stopErr
}

func (r *runtimeTracker) EnsureStopped(ctx context.Context, p sqlite.PluginVersion) error {
	return r.Stop(ctx, p)
}

type healthStub struct {
	mu  sync.Mutex
	err error
}

func (h *healthStub) Health(_ context.Context, _ sqlite.PluginVersion) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func setupSupervisor(t *testing.T) (*HealthSupervisor, *runtimeTracker, *healthStub, *sqlite.PluginRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))

	repo := sqlite.NewPluginRepository(db)
	record := sqlite.PluginVersion{
		PluginID:       "acme/logger",
		Version:        "1.0.0",
		Name:           "Logger",
		Tier:           1,
		APIVersion:     "gosite-plugin/1",
		RPCVersion:     "1",
		ManifestJSON:   `{}`,
		ArtifactDigest: "deadbeef",
		ArtifactPath:   "/tmp/logger.zip",
		State:          sqlite.PluginStateEnabled,
	}
	_, err = repo.CreateOrRetryInstall(context.Background(), record)
	require.NoError(t, err)
	_, err = repo.SetState(context.Background(), record.PluginID, record.Version, sqlite.PluginStateEnabled)
	require.NoError(t, err)

	tracker := &runtimeTracker{}
	health := &healthStub{}
	sup := NewHealthSupervisor(repo, tracker, health,
		WithSupervisorInterval(10*time.Millisecond),
		WithSupervisorMaxAttempts(3),
		WithSupervisorWindow(time.Minute),
		WithSupervisorBackoff(time.Millisecond, 5*time.Millisecond),
	)
	return sup, tracker, health, repo
}

func TestSupervisorRestartsOnTransientFailure(t *testing.T) {
	sup, tracker, health, _ := setupSupervisor(t)

	health.mu.Lock()
	health.err = errTransient
	health.mu.Unlock()

	sup.tick(context.Background())

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	assert.Greater(t, len(tracker.starts), 0, "supervisor should restart at least once")
	assert.Contains(t, tracker.stops, "acme/logger@1.0.0")
}

func TestSupervisorAutoDisablesAfterRepeatedFailure(t *testing.T) {
	t.Parallel()
	sup, tracker, health, repo := setupSupervisor(t)

	health.mu.Lock()
	health.err = errors.New("hard fail")
	health.mu.Unlock()

	for i := 0; i < 5; i++ {
		sup.tick(context.Background())
		record, err := repo.Find(context.Background(), "acme/logger", "1.0.0")
		if err == nil && record.State == sqlite.PluginStateInstalled {
			break
		}
	}

	record, err := repo.Find(context.Background(), "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, record.State)
	assert.NotEmpty(t, record.FailureClass)
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	assert.Contains(t, tracker.stops, "acme/logger@1.0.0")
}

var errTransient = errors.New("transient")
var _ = sql.ErrNoRows
