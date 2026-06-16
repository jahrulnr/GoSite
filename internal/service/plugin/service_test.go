package plugin_test

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeStub struct {
	startErr error
	stopErr  error
	started  []string
	stopped  []string
}

func (r *runtimeStub) Start(_ context.Context, p sqlite.PluginVersion) error {
	r.started = append(r.started, p.PluginID+"@"+p.Version)
	return r.startErr
}

func (r *runtimeStub) Stop(_ context.Context, p sqlite.PluginVersion) error {
	r.stopped = append(r.stopped, p.PluginID+"@"+p.Version)
	return r.stopErr
}

func (r *runtimeStub) EnsureStopped(ctx context.Context, p sqlite.PluginVersion) error {
	return r.Stop(ctx, p)
}

type dispatcherStub struct {
	refreshErr error
	disableErr error
	refreshed  [][]sqlite.PluginVersion
	disabled   []string
}

func (d *dispatcherStub) Refresh(_ context.Context, plugins []sqlite.PluginVersion) error {
	copied := append([]sqlite.PluginVersion(nil), plugins...)
	d.refreshed = append(d.refreshed, copied)
	return d.refreshErr
}

func (d *dispatcherStub) Disable(_ context.Context, p sqlite.PluginVersion) error {
	d.disabled = append(d.disabled, p.PluginID+"@"+p.Version)
	return d.disableErr
}

type migratorStub struct {
	err    error
	out    string
	calls  []string
	states []string
}

func (m *migratorStub) Migrate(_ context.Context, current sqlite.PluginVersion, currentConfig string, next sqlite.PluginVersion) (plugin.MigrateResult, error) {
	m.calls = append(m.calls, current.PluginID+"@"+current.Version+"->"+next.Version)
	m.states = append(m.states, current.State+"->"+next.State)
	if m.err != nil {
		return plugin.MigrateResult{}, m.err
	}
	return plugin.MigrateResult{OK: true, MigratedConfig: m.out}, nil
}

func setupPluginService(t *testing.T, runtime plugin.RuntimeManager, dispatcher plugin.HookDispatcher) (*plugin.Service, *sqlite.PluginRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	repo := sqlite.NewPluginRepository(db)
	return plugin.NewService(repo, t.TempDir(), runtime, dispatcher), repo
}

func setupPluginServiceWithOptions(t *testing.T, runtime plugin.RuntimeManager, dispatcher plugin.HookDispatcher, opts ...plugin.Option) (*plugin.Service, *sqlite.PluginRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	repo := sqlite.NewPluginRepository(db)
	return plugin.NewService(repo, t.TempDir(), runtime, dispatcher, opts...), repo
}

func manifestJSON(id, version string, tier int) string {
	rpc := ""
	entrypoints := ""
	if tier == 1 {
		rpc = `"rpcVersion":"1",`
		entrypoints = `,"entrypoints":{"validate":{"type":"go-plugin","command":"plugin/validate"}}`
	}
	return `{
		"id":"` + id + `",
		"name":"Test Plugin",
		"version":"` + version + `",
		"tier":` + strconv.Itoa(tier) + `,
		"apiVersion":"gosite-plugin/1",
		"minGoSiteVersion":"0.1.0",
		` + rpc + `
		"capabilities":{"hooks":["nginx.before_reload"],"hookIsolation":"sequential","uiSidebar":true},
		"ui":{"sidebar":[{"label":"Test","route":"/plugins/` + id + `/settings"}]}
		` + entrypoints + `
	}`
}

func zipArtifact(t *testing.T, manifest string, files map[string]string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	mf, err := zw.Create("manifest.json")
	require.NoError(t, err)
	_, err = mf.Write([]byte(manifest))
	require.NoError(t, err)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o755)
		w, err := zw.CreateHeader(header)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestPluginServiceInstallEnableDisableUninstall(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{}
	dispatcher := &dispatcherStub{}
	svc, _ := setupPluginService(t, rt, dispatcher)

	ctx := context.Background()
	installed, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, installed.State)
	assert.NotEmpty(t, installed.ArtifactDigest)

	enabled, err := svc.Enable(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnabled, enabled.State)
	assert.Equal(t, []string{"acme/logger@1.0.0"}, rt.started)
	require.NotEmpty(t, dispatcher.refreshed)
	assert.Len(t, dispatcher.refreshed[len(dispatcher.refreshed)-1], 1)

	disabled, err := svc.Disable(ctx, "acme/logger")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, disabled.State)
	assert.Contains(t, dispatcher.disabled, "acme/logger@1.0.0")
	assert.Contains(t, rt.stopped, "acme/logger@1.0.0")

	uninstalled, err := svc.Uninstall(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateUninstalled, uninstalled.State)
	assert.NotNil(t, uninstalled.ConfigDeletedAt)
}

func TestPluginServiceInstallTier1RunsValidateEntrypoint(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginService(t, nil, nil)
	content := zipArtifact(t, manifestJSON("acme/logger", "1.0.0", 1), map[string]string{
		"plugin/validate": "#!/bin/sh\nexit 0\n",
	})

	installed, err := svc.Install(context.Background(), plugin.InstallInput{Content: content})
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, installed.State)
}

func TestPluginServiceInstallFailedCanRetrySameVersion(t *testing.T) {
	t.Parallel()

	svc, repo := setupPluginService(t, nil, nil)
	ctx := context.Background()
	bad := zipArtifact(t, manifestJSON("acme/logger", "1.0.0", 1), map[string]string{
		"plugin/validate": "#!/bin/sh\necho broken\nexit 7\n",
	})
	_, err := svc.Install(ctx, plugin.InstallInput{Content: bad})
	require.Error(t, err)
	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstallFailed, record.State)

	good := zipArtifact(t, manifestJSON("acme/logger", "1.0.0", 1), map[string]string{
		"plugin/validate": "#!/bin/sh\nexit 0\n",
	})
	installed, err := svc.Install(ctx, plugin.InstallInput{Content: good})
	require.NoError(t, err)

	assert.Equal(t, sqlite.PluginStateInstalled, installed.State)
	assert.Empty(t, installed.FailureClass)
	assert.NotEqual(t, record.ArtifactDigest, installed.ArtifactDigest)
}

func TestPluginServiceInstallRejectsCollision(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginService(t, nil, nil)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)

	_, err = svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeConflict, appErr.Code)
}

func TestPluginServiceUsesConfiguredHostVersion(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginServiceWithOptions(t, nil, nil, plugin.WithHostVersion("2.4.0"))
	manifest := strings.Replace(manifestJSON("acme/logger", "1.0.0", 0), `"minGoSiteVersion":"0.1.0"`, `"minGoSiteVersion":"2.3.0"`, 1)

	installed, err := svc.Install(context.Background(), plugin.InstallInput{Content: []byte(manifest)})
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, installed.State)
}

func TestPluginServiceRejectsNewerThanConfiguredHostVersion(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginServiceWithOptions(t, nil, nil, plugin.WithHostVersion("2.4.0"))
	manifest := strings.Replace(manifestJSON("acme/logger", "1.0.0", 0), `"minGoSiteVersion":"0.1.0"`, `"minGoSiteVersion":"2.5.0"`, 1)

	_, err := svc.Install(context.Background(), plugin.InstallInput{Content: []byte(manifest)})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodePluginInvalid, appErr.Code)
}

func TestPluginServiceRejectsUnsignedWhenPolicyRequiresSignature(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	svc := plugin.NewService(
		sqlite.NewPluginRepository(db),
		t.TempDir(),
		nil,
		nil,
		plugin.WithAllowUnsigned(false),
		plugin.WithKeyringPath(filepath.Join(t.TempDir(), "keyring.json")),
	)

	_, err = svc.Install(context.Background(), plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodePluginInvalid, appErr.Code)
	assert.Contains(t, appErr.Message, "signature required")
}

func TestPluginServiceEnableFailureStoresFailureMetadata(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{startErr: errors.New("boom")}
	svc, repo := setupPluginService(t, rt, nil)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)

	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.Error(t, err)

	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnableFailed, record.State)
	assert.Equal(t, plugin.FailureStartFailed, record.FailureClass)
	assert.Equal(t, "boom", record.FailureMessage)
	assert.NotNil(t, record.FailureAt)
}

func TestPluginServiceDisableStopFailureKeepsRegistryEnabled(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{stopErr: errors.New("stop failed")}
	dispatcher := &dispatcherStub{}
	svc, repo := setupPluginService(t, rt, dispatcher)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)

	_, err = svc.Disable(ctx, "acme/logger")
	require.Error(t, err)

	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnabled, record.State)
	assert.Equal(t, plugin.FailureStopFailed, record.FailureClass)
	assert.Equal(t, "stop failed", record.FailureMessage)
}

func TestPluginServiceDisableHookFailureKeepsRegistryEnabled(t *testing.T) {
	t.Parallel()

	dispatcher := &dispatcherStub{disableErr: errors.New("hook refresh failed")}
	svc, repo := setupPluginService(t, nil, dispatcher)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)

	_, err = svc.Disable(ctx, "acme/logger")
	require.Error(t, err)

	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnabled, record.State)
	assert.Equal(t, plugin.FailureHookRefreshFailed, record.FailureClass)
	assert.Equal(t, "hook refresh failed", record.FailureMessage)
}

func TestPluginServiceEnableRefreshFailureStopsRuntimeAndMarksEnableFailed(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{}
	dispatcher := &dispatcherStub{refreshErr: errors.New("refresh failed")}
	svc, repo := setupPluginService(t, rt, dispatcher)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)

	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.Error(t, err)

	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnableFailed, record.State)
	assert.Equal(t, plugin.FailureHookRefreshFailed, record.FailureClass)
	assert.Equal(t, []string{"acme/logger@1.0.0"}, rt.stopped)
}

func TestPluginServiceEnableWithoutVersionUsesSemverLatest(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{}
	svc, _ := setupPluginService(t, rt, nil)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.2.0", 0))})
	require.NoError(t, err)
	_, err = svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.10.0", 0))})
	require.NoError(t, err)

	enabled, err := svc.Enable(ctx, "acme/logger", "")
	require.NoError(t, err)

	assert.Equal(t, "1.10.0", enabled.Version)
	assert.Equal(t, []string{"acme/logger@1.10.0"}, rt.started)
}

func TestPluginServiceSwitchValidatesConfigMigrationBeforeDisablingCurrent(t *testing.T) {
	t.Parallel()

	rt := &runtimeStub{}
	dispatcher := &dispatcherStub{}
	migrator := &migratorStub{err: errors.New("migration rejected")}
	svc, repo := setupPluginServiceWithOptions(t, rt, dispatcher, plugin.WithConfigMigrator(migrator))
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	_, err = svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "2.0.0", 0))})
	require.NoError(t, err)
	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)

	_, err = svc.SwitchEnabledVersion(ctx, "acme/logger", "2.0.0")
	require.Error(t, err)

	current, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	next, err := repo.Find(ctx, "acme/logger", "2.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateEnabled, current.State)
	assert.Equal(t, sqlite.PluginStateInstalled, next.State)
	assert.Equal(t, plugin.FailureConfigMigration, next.FailureClass)
	assert.Equal(t, []string{"acme/logger@1.0.0->2.0.0"}, migrator.calls)
	assert.NotContains(t, dispatcher.disabled, "acme/logger@1.0.0")
}

func TestPluginServiceUninstallRejectsEnabled(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginService(t, nil, nil)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	_, err = svc.Enable(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)

	_, err = svc.Uninstall(ctx, "acme/logger", "1.0.0")
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeConflict, appErr.Code)
}

func TestPluginServiceReconcileCompletesPendingFSDeleteCleanup(t *testing.T) {
	t.Parallel()

	svc, repo := setupPluginService(t, nil, nil)
	ctx := context.Background()
	installed, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)
	_, err = repo.SetFailure(ctx, "acme/logger", "1.0.0", sqlite.PluginStateInstalled, plugin.FailureFSDeleteFailed, "unlink failed")
	require.NoError(t, err)
	require.DirExists(t, filepath.Dir(installed.ArtifactPath))

	require.NoError(t, svc.Reconcile(ctx))

	record, err := repo.Find(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateUninstalled, record.State)
	assert.Empty(t, record.FailureClass)
	assert.NotNil(t, record.ConfigDeletedAt)
	require.NoDirExists(t, filepath.Dir(installed.ArtifactPath))
}

func TestPluginServicePurgeRequiresUninstalledRecord(t *testing.T) {
	t.Parallel()

	svc, repo := setupPluginService(t, nil, nil)
	ctx := context.Background()
	_, err := svc.Install(ctx, plugin.InstallInput{Content: []byte(manifestJSON("acme/logger", "1.0.0", 0))})
	require.NoError(t, err)

	err = svc.Purge(ctx, "acme/logger", "1.0.0")
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeConflict, appErr.Code)

	_, err = svc.Uninstall(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	require.NoError(t, svc.Purge(ctx, "acme/logger", "1.0.0"))

	_, err = repo.Find(ctx, "acme/logger", "1.0.0")
	require.Error(t, err)
}

func TestPluginServiceUnsupportedAPIVersion(t *testing.T) {
	t.Parallel()

	svc, _ := setupPluginService(t, nil, nil)
	_, err := svc.Install(context.Background(), plugin.InstallInput{Content: []byte(strings.Replace(manifestJSON("acme/logger", "1.0.0", 0), "gosite-plugin/1", "gosite-plugin/2", 1))})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodePluginInvalid, appErr.Code)
}
