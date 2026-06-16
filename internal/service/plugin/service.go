package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jahrulnr/gosite/internal/buildinfo"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const (
	HostAPIVersion = "gosite-plugin/1"
	HostRPCVersion = "1"

	FailureValidateTimeout    = "validate_timeout"
	FailureStartFailed        = "start_failed"
	FailureHookRefreshFailed  = "hook_refresh_failed"
	FailureDBFailed           = "db_failed"
	FailureCompensationFailed = "compensation_failed"
	FailureStopFailed         = "stop_failed"
	FailureFSDeleteFailed     = "fs_delete_failed"
	FailureConfigMigration    = "config_migration_failed"
	FailureUnknown            = "unknown"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)

// Manifest is the install-time contract supplied by an artifact.
type Manifest struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Version          string             `json:"version"`
	Tier             int                `json:"tier"`
	APIVersion       string             `json:"apiVersion"`
	MinGoSiteVersion string             `json:"minGoSiteVersion"`
	RPCVersion       string             `json:"rpcVersion"`
	ConfigVersion    string             `json:"configVersion,omitempty"`
	Capabilities     Capabilities       `json:"capabilities"`
	Permissions      []string           `json:"permissions,omitempty"`
	Entrypoints      map[string]Command `json:"entrypoints,omitempty"`
	Artifact         Artifact           `json:"artifact,omitempty"`
	Signatures       []Signature        `json:"signatures,omitempty"`
	UI               UIContributions    `json:"ui,omitempty"`
}

// Capabilities declares host-visible plugin powers.
type Capabilities struct {
	Hooks         []string `json:"hooks,omitempty"`
	HookIsolation string   `json:"hookIsolation,omitempty"`
	UISidebar     bool     `json:"uiSidebar,omitempty"`
	ConfigSchema  bool     `json:"configSchema,omitempty"`
	LoggingSink   bool     `json:"loggingSink,omitempty"`
	RulesAndRoles string   `json:"rulesAndRoles,omitempty"`
}

// Command describes a manifest entrypoint.
type Command struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// Artifact contains artifact metadata from the manifest.
type Artifact struct {
	SHA256 string `json:"sha256,omitempty"`
}

// Signature is a vendor key signature reference.
type Signature struct {
	KeyID string `json:"keyId"`
	Sig   string `json:"sig"`
}

// TrustedKey is one admin-managed public key allowed to sign plugin artifacts.
type TrustedKey struct {
	Vendor    string `json:"vendor"`
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

// Keyring is the on-disk JSON keyring format.
type Keyring struct {
	Keys []TrustedKey `json:"keys"`
}

// UIContributions are host-rendered data-only UI contributions.
type UIContributions struct {
	Sidebar []SidebarEntry `json:"sidebar,omitempty"`
}

// SidebarEntry is a data-only host UI route contribution.
type SidebarEntry struct {
	Label string `json:"label"`
	Route string `json:"route"`
}

// InstallInput is one uploaded plugin artifact.
type InstallInput struct {
	Name           string
	Content        []byte
	ExpectedSHA256 string
}

// RuntimeManager starts and stops plugin runtimes.
type RuntimeManager interface {
	Start(ctx context.Context, plugin sqlite.PluginVersion) error
	Stop(ctx context.Context, plugin sqlite.PluginVersion) error
	EnsureStopped(ctx context.Context, plugin sqlite.PluginVersion) error
}

// HookDispatcher refreshes the enabled plugin set used by hook dispatch.
type HookDispatcher interface {
	Refresh(ctx context.Context, plugins []sqlite.PluginVersion) error
	Disable(ctx context.Context, plugin sqlite.PluginVersion) error
}

// ConfigMigrator validates config compatibility before switching versions.
type ConfigMigrator interface {
	Validate(ctx context.Context, current sqlite.PluginVersion, next sqlite.PluginVersion) error
}

// NoopRuntimeManager is the initial lifecycle boundary before subprocess execution exists.
type NoopRuntimeManager struct{}

func (NoopRuntimeManager) Start(context.Context, sqlite.PluginVersion) error         { return nil }
func (NoopRuntimeManager) Stop(context.Context, sqlite.PluginVersion) error          { return nil }
func (NoopRuntimeManager) EnsureStopped(context.Context, sqlite.PluginVersion) error { return nil }

// NoopHookDispatcher stores no process-local hook state.
type NoopHookDispatcher struct{}

func (NoopHookDispatcher) Refresh(context.Context, []sqlite.PluginVersion) error { return nil }
func (NoopHookDispatcher) Disable(context.Context, sqlite.PluginVersion) error   { return nil }

// NoopConfigMigrator is used until plugin config storage is implemented.
type NoopConfigMigrator struct{}

func (NoopConfigMigrator) Validate(context.Context, sqlite.PluginVersion, sqlite.PluginVersion) error {
	return nil
}

// MemoryHookDispatcher is the concrete enabled-set boundary used by lifecycle operations.
type MemoryHookDispatcher struct {
	mu                 sync.RWMutex
	maxConcurrentHooks int
	enabled            map[string]sqlite.PluginVersion
}

// NewMemoryHookDispatcher returns a deterministic in-memory hook target registry.
func NewMemoryHookDispatcher(maxConcurrentHooks int) *MemoryHookDispatcher {
	if maxConcurrentHooks <= 0 {
		maxConcurrentHooks = 10
	}
	return &MemoryHookDispatcher{
		maxConcurrentHooks: maxConcurrentHooks,
		enabled:            map[string]sqlite.PluginVersion{},
	}
}

func (d *MemoryHookDispatcher) Refresh(_ context.Context, plugins []sqlite.PluginVersion) error {
	next := make(map[string]sqlite.PluginVersion, len(plugins))
	for _, plugin := range plugins {
		next[plugin.PluginID] = plugin
	}
	d.mu.Lock()
	d.enabled = next
	d.mu.Unlock()
	return nil
}

func (d *MemoryHookDispatcher) Disable(_ context.Context, plugin sqlite.PluginVersion) error {
	d.mu.Lock()
	delete(d.enabled, plugin.PluginID)
	d.mu.Unlock()
	return nil
}

// Snapshot returns the current dispatcher source of truth for tests and diagnostics.
func (d *MemoryHookDispatcher) Snapshot() []sqlite.PluginVersion {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]sqlite.PluginVersion, 0, len(d.enabled))
	for _, plugin := range d.enabled {
		out = append(out, plugin)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PluginID == out[j].PluginID {
			return compareSemver(out[i].Version, out[j].Version) > 0
		}
		return out[i].PluginID < out[j].PluginID
	})
	return out
}

// MaxConcurrentHooks returns the host concurrency limit used by dispatchers.
func (d *MemoryHookDispatcher) MaxConcurrentHooks() int {
	return d.maxConcurrentHooks
}

// ProcessRuntimeManager starts optional manifest runtime entrypoints as child processes.
type ProcessRuntimeManager struct {
	mu        sync.Mutex
	processes map[string]*pluginProcess
}

// NewProcessRuntimeManager returns a subprocess-backed runtime boundary.
func NewProcessRuntimeManager() *ProcessRuntimeManager {
	return &ProcessRuntimeManager{processes: map[string]*pluginProcess{}}
}

type pluginProcess struct {
	cancel context.CancelFunc
}

func (m *ProcessRuntimeManager) Start(ctx context.Context, plugin sqlite.PluginVersion) error {
	manifest := manifestFromRecord(plugin)
	entry, ok := manifest.Entrypoints["runtime"]
	if !ok || strings.TrimSpace(entry.Command) == "" {
		entry = manifest.Entrypoints["serve"]
	}
	if strings.TrimSpace(entry.Command) == "" {
		return nil
	}
	commandPath, args, err := resolvePluginCommand(filepath.Dir(plugin.ArtifactPath), entry.Command)
	if err != nil {
		return err
	}
	runtimeCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runtimeCtx, commandPath, args...)
	cmd.Dir = filepath.Dir(plugin.ArtifactPath)
	cmd.Env = append(os.Environ(),
		"GOSITE_PLUGIN_ID="+plugin.PluginID,
		"GOSITE_PLUGIN_VERSION="+plugin.Version,
	)
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	key := plugin.PluginID + "@" + plugin.Version
	process := &pluginProcess{cancel: cancel}
	m.mu.Lock()
	if existing := m.processes[key]; existing != nil {
		existing.cancel()
	}
	m.processes[key] = process
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.processes[key] == process {
			delete(m.processes, key)
		}
		m.mu.Unlock()
	}()
	return nil
}

func (m *ProcessRuntimeManager) Stop(_ context.Context, plugin sqlite.PluginVersion) error {
	key := plugin.PluginID + "@" + plugin.Version
	m.mu.Lock()
	process := m.processes[key]
	delete(m.processes, key)
	m.mu.Unlock()
	if process != nil {
		process.cancel()
	}
	return nil
}

func (m *ProcessRuntimeManager) EnsureStopped(ctx context.Context, plugin sqlite.PluginVersion) error {
	return m.Stop(ctx, plugin)
}

// Service manages plugin install and lifecycle operations.
type Service struct {
	repo          *sqlite.PluginRepository
	storageDir    string
	runtime       RuntimeManager
	dispatcher    HookDispatcher
	migrator      ConfigMigrator
	validateTO    time.Duration
	allowUnsigned bool
	keyringPath   string
	hostVersion   string
}

// Option configures plugin service behavior.
type Option func(*Service)

// WithAllowUnsigned permits unsigned artifacts when explicitly configured.
func WithAllowUnsigned(allow bool) Option {
	return func(s *Service) { s.allowUnsigned = allow }
}

// WithKeyringPath sets the trusted vendor keyring JSON path.
func WithKeyringPath(path string) Option {
	return func(s *Service) { s.keyringPath = path }
}

// WithHostVersion sets the GoSite version used for plugin minGoSiteVersion checks.
func WithHostVersion(version string) Option {
	return func(s *Service) {
		if strings.TrimSpace(version) != "" {
			s.hostVersion = version
		}
	}
}

// WithConfigMigrator sets the switch-time config migration validator.
func WithConfigMigrator(migrator ConfigMigrator) Option {
	return func(s *Service) {
		if migrator != nil {
			s.migrator = migrator
		}
	}
}

// NewService returns a plugin service.
func NewService(repo *sqlite.PluginRepository, storageRoot string, runtime RuntimeManager, dispatcher HookDispatcher, opts ...Option) *Service {
	if runtime == nil {
		runtime = NoopRuntimeManager{}
	}
	if dispatcher == nil {
		dispatcher = NoopHookDispatcher{}
	}
	svc := &Service{
		repo:          repo,
		storageDir:    filepath.Join(storageRoot, "plugins"),
		runtime:       runtime,
		dispatcher:    dispatcher,
		migrator:      NoopConfigMigrator{},
		validateTO:    5 * time.Second,
		allowUnsigned: true,
		hostVersion:   buildinfo.Version,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// List returns all plugin registry records.
func (s *Service) List(ctx context.Context) ([]sqlite.PluginVersion, error) {
	plugins, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list plugins failed", err)
	}
	return plugins, nil
}

// Install verifies, stores, and validates an artifact before marking it installed.
func (s *Service) Install(ctx context.Context, in InstallInput) (sqlite.PluginVersion, error) {
	if len(in.Content) == 0 {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeInvalidInput, "plugin artifact is required")
	}

	digestBytes := sha256.Sum256(in.Content)
	digest := hex.EncodeToString(digestBytes[:])
	if expected := strings.TrimSpace(in.ExpectedSHA256); expected != "" && !strings.EqualFold(expected, digest) {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodePluginInvalid, "artifact sha256 mismatch")
	}

	manifest, manifestJSON, err := parseManifest(in.Content)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	if err := validateManifest(manifest); err != nil {
		return sqlite.PluginVersion{}, err
	}
	if err := s.compatibilityCheck(manifest); err != nil {
		return sqlite.PluginVersion{}, err
	}
	if err := s.verifyArtifact(ctx, manifest, digest); err != nil {
		return sqlite.PluginVersion{}, err
	}

	artifactDir, err := safePluginDir(s.storageDir, manifest.ID, manifest.Version)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	existing, existingErr := s.repo.Find(ctx, manifest.ID, manifest.Version)
	if existingErr == nil {
		if existing.State != sqlite.PluginStateInstallFailed {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin version already exists")
		}
		_ = os.RemoveAll(artifactDir)
	} else if !errors.Is(existingErr, sql.ErrNoRows) {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find existing plugin failed", existingErr)
	}
	if err := ensureDiskWritable(artifactDir, int64(len(in.Content))); err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "plugin storage unavailable", err)
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "create plugin storage failed", err)
	}
	if err := extractZipArtifact(in.Content, artifactDir); err != nil {
		return sqlite.PluginVersion{}, err
	}

	artifactPath := filepath.Join(artifactDir, digest+".artifact")
	if err := os.WriteFile(artifactPath, in.Content, 0o644); err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "persist plugin artifact failed", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "manifest.json"), manifestJSON, 0o644); err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "persist plugin manifest failed", err)
	}

	capsJSON, _ := json.Marshal(manifest.Capabilities)
	uiJSON, _ := json.Marshal(manifest.UI)
	record, err := s.repo.CreateOrRetryInstall(ctx, sqlite.PluginVersion{
		PluginID:         manifest.ID,
		Version:          manifest.Version,
		Name:             manifest.Name,
		Tier:             manifest.Tier,
		APIVersion:       manifest.APIVersion,
		MinGoSiteVersion: manifest.MinGoSiteVersion,
		RPCVersion:       manifest.RPCVersion,
		ConfigVersion:    manifest.ConfigVersion,
		ManifestJSON:     string(manifestJSON),
		CapabilitiesJSON: string(capsJSON),
		UIJSON:           string(uiJSON),
		ArtifactDigest:   digest,
		ArtifactPath:     artifactPath,
		State:            sqlite.PluginStateInstalling,
	})
	if err != nil {
		_ = os.RemoveAll(artifactDir)
		if errors.Is(err, sqlite.ErrPluginVersionExists) || strings.Contains(err.Error(), "constraint") {
			return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConflict, "plugin version already exists", err)
		}
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "create plugin record failed", err)
	}

	if err := s.runValidate(ctx, manifest, artifactDir); err != nil {
		record, _ = s.repo.SetFailure(ctx, manifest.ID, manifest.Version, sqlite.PluginStateInstallFailed, classifyValidationError(err), err.Error())
		return record, err
	}
	record, err = s.repo.SetState(ctx, manifest.ID, manifest.Version, sqlite.PluginStateInstalled)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "mark plugin installed failed", err)
	}
	return record, nil
}

// Purge permanently deletes an already-uninstalled registry row.
func (s *Service) Purge(ctx context.Context, pluginID, version string) error {
	if err := s.repo.DeleteUninstalled(ctx, pluginID, version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.CodeConflict, "plugin version must be uninstalled before purge")
		}
		return apperror.Wrap(apperror.CodeDatabase, "purge plugin failed", err)
	}
	return nil
}

// Enable starts a runtime, refreshes hooks, and marks one version enabled.
func (s *Service) Enable(ctx context.Context, pluginID, version string) (sqlite.PluginVersion, error) {
	record, err := s.findForEnable(ctx, pluginID, version)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	if record.State == sqlite.PluginStateEnableFailed && record.FailureClass == FailureCompensationFailed {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin needs manual recovery before retry")
	}
	if record.State != sqlite.PluginStateInstalled && record.State != sqlite.PluginStateEnableFailed {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin is not installable")
	}
	if err := s.compatibilityCheck(manifestFromRecord(record)); err != nil {
		return sqlite.PluginVersion{}, err
	}

	record, err = s.repo.SetState(ctx, record.PluginID, record.Version, sqlite.PluginStateEnabling)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "mark plugin enabling failed", err)
	}
	if err := s.runtime.Start(ctx, record); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnableFailed, FailureStartFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodePluginOperation, "start plugin runtime failed", err)
	}
	record, err = s.repo.SetState(ctx, record.PluginID, record.Version, sqlite.PluginStateEnabled)
	if err != nil {
		if stopErr := s.runtime.Stop(ctx, record); stopErr != nil {
			failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnableFailed, FailureCompensationFailed, stopErr.Error())
			return failed, apperror.Wrap(apperror.CodePluginOperation, "plugin enable compensation failed", stopErr)
		}
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnableFailed, FailureDBFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodeDatabase, "mark plugin enabled failed", err)
	}
	if err := s.refreshEnabled(ctx); err != nil {
		if stopErr := s.runtime.Stop(ctx, record); stopErr != nil {
			failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnableFailed, FailureCompensationFailed, stopErr.Error())
			return failed, apperror.Wrap(apperror.CodePluginOperation, "plugin enable compensation failed", stopErr)
		}
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnableFailed, FailureHookRefreshFailed, err.Error())
		_ = s.refreshEnabled(ctx)
		return failed, apperror.Wrap(apperror.CodePluginOperation, "refresh plugin hooks failed", err)
	}
	return record, nil
}

// Disable removes hook targets, stops runtime, and returns the version to installed.
func (s *Service) Disable(ctx context.Context, pluginID string) (sqlite.PluginVersion, error) {
	record, err := s.repo.FindEnabled(ctx, pluginID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeNotFound, "enabled plugin not found")
		}
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find enabled plugin failed", err)
	}
	record, err = s.repo.SetState(ctx, record.PluginID, record.Version, sqlite.PluginStateDisabling)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "mark plugin disabling failed", err)
	}
	if err := s.dispatcher.Disable(ctx, record); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnabled, FailureHookRefreshFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodePluginOperation, "disable plugin hooks failed", err)
	}
	if err := s.runtime.Stop(ctx, record); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnabled, FailureStopFailed, err.Error())
		if refreshErr := s.refreshEnabled(ctx); refreshErr != nil {
			failed, _ = s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateEnabled, FailureCompensationFailed, refreshErr.Error())
		}
		return failed, apperror.Wrap(apperror.CodePluginOperation, "stop plugin runtime failed", err)
	}
	record, err = s.repo.SetState(ctx, record.PluginID, record.Version, sqlite.PluginStateInstalled)
	if err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateInstalled, FailureDBFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodeDatabase, "mark plugin disabled failed", err)
	}
	if err := s.refreshEnabled(ctx); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateInstalled, FailureHookRefreshFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodePluginOperation, "refresh plugin hooks failed", err)
	}
	return record, nil
}

// SwitchEnabledVersion disables the current version and enables the target version.
func (s *Service) SwitchEnabledVersion(ctx context.Context, pluginID, version string) (sqlite.PluginVersion, error) {
	current, err := s.repo.FindEnabled(ctx, pluginID)
	if err == nil {
		if current.Version == version {
			return current, nil
		}
		target, err := s.repo.Find(ctx, pluginID, version)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sqlite.PluginVersion{}, apperror.New(apperror.CodeNotFound, "plugin version not found")
			}
			return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find switch target failed", err)
		}
		if err := s.compatibilityCheck(manifestFromRecord(target)); err != nil {
			return sqlite.PluginVersion{}, err
		}
		if target.State == sqlite.PluginStateEnableFailed && target.FailureClass == FailureCompensationFailed {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin needs manual recovery before retry")
		}
		if target.State != sqlite.PluginStateInstalled && target.State != sqlite.PluginStateEnableFailed {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin is not installable")
		}
		if err := s.migrator.Validate(ctx, current, target); err != nil {
			failed, _ := s.repo.SetFailure(ctx, target.PluginID, target.Version, sqlite.PluginStateInstalled, FailureConfigMigration, err.Error())
			return failed, apperror.Wrap(apperror.CodePluginOperation, "validate plugin config migration failed", err)
		}
		if _, err := s.Disable(ctx, pluginID); err != nil {
			return sqlite.PluginVersion{}, err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find enabled plugin failed", err)
	}

	next, err := s.Enable(ctx, pluginID, version)
	if err != nil && current.PluginID != "" {
		return next, err
	}
	return next, err
}

// Uninstall removes a stable, non-enabled plugin version and soft-deletes config.
func (s *Service) Uninstall(ctx context.Context, pluginID, version string) (sqlite.PluginVersion, error) {
	record, err := s.repo.Find(ctx, pluginID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeNotFound, "plugin version not found")
		}
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find plugin failed", err)
	}
	if !stableUninstallState(record.State) {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeConflict, "plugin must be disabled and stable before uninstall")
	}
	record, err = s.repo.SetState(ctx, record.PluginID, record.Version, sqlite.PluginStateUninstalling)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "mark plugin uninstalling failed", err)
	}
	if err := s.runtime.EnsureStopped(ctx, record); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateInstalled, FailureStopFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodePluginOperation, "ensure plugin stopped failed", err)
	}
	if err := os.RemoveAll(filepath.Dir(record.ArtifactPath)); err != nil {
		failed, _ := s.repo.SetFailure(ctx, record.PluginID, record.Version, sqlite.PluginStateInstalled, FailureFSDeleteFailed, err.Error())
		return failed, apperror.Wrap(apperror.CodePluginOperation, "delete plugin artifact failed", err)
	}
	record, err = s.repo.MarkConfigDeleted(ctx, record.PluginID, record.Version)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "mark plugin uninstalled failed", err)
	}
	return record, nil
}

// Reconcile enforces registry truth on startup.
func (s *Service) Reconcile(ctx context.Context) error {
	records, err := s.repo.List(ctx)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "list plugins failed", err)
	}
	for _, record := range records {
		if record.State != sqlite.PluginStateEnabled {
			_ = s.runtime.Stop(ctx, record)
		}
		if record.State == sqlite.PluginStateInstalled && record.FailureClass == FailureFSDeleteFailed {
			if err := os.RemoveAll(filepath.Dir(record.ArtifactPath)); err == nil {
				_, _ = s.repo.MarkConfigDeleted(ctx, record.PluginID, record.Version)
			}
		}
	}
	return s.refreshEnabled(ctx)
}

func (s *Service) findForEnable(ctx context.Context, pluginID, version string) (sqlite.PluginVersion, error) {
	var record sqlite.PluginVersion
	var err error
	if strings.TrimSpace(version) == "" {
		records, listErr := s.repo.ListInstallable(ctx, pluginID)
		if listErr == nil && len(records) == 0 {
			err = sql.ErrNoRows
		} else if listErr != nil {
			err = listErr
		} else {
			sort.SliceStable(records, func(i, j int) bool {
				return compareSemver(records[i].Version, records[j].Version) > 0
			})
			record = records[0]
		}
	} else {
		record, err = s.repo.Find(ctx, pluginID, version)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeNotFound, "plugin version not found")
		}
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeDatabase, "find plugin failed", err)
	}
	return record, nil
}

func (s *Service) refreshEnabled(ctx context.Context) error {
	enabled, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	sort.SliceStable(enabled, func(i, j int) bool {
		if enabled[i].PluginID == enabled[j].PluginID {
			return compareSemver(enabled[i].Version, enabled[j].Version) > 0
		}
		return enabled[i].PluginID < enabled[j].PluginID
	})
	return s.dispatcher.Refresh(ctx, enabled)
}

func (s *Service) runValidate(ctx context.Context, manifest Manifest, artifactDir string) error {
	validateCtx, cancel := context.WithTimeout(ctx, s.validateTO)
	defer cancel()
	select {
	case <-validateCtx.Done():
		return validateCtx.Err()
	default:
	}
	if manifest.Tier == 1 {
		validate := manifest.Entrypoints["validate"]
		if strings.TrimSpace(validate.Command) == "" || validate.Type != "go-plugin" {
			return apperror.New(apperror.CodePluginInvalid, "tier 1 plugins require a go-plugin validate entrypoint")
		}
		commandPath, args, err := resolvePluginCommand(artifactDir, validate.Command)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(validateCtx, commandPath, args...)
		cmd.Dir = artifactDir
		output, err := cmd.CombinedOutput()
		if errors.Is(validateCtx.Err(), context.DeadlineExceeded) {
			return validateCtx.Err()
		}
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}
			return apperror.New(apperror.CodePluginInvalid, "plugin validate failed: "+message)
		}
	}
	return nil
}

func (s *Service) verifyArtifact(ctx context.Context, manifest Manifest, digest string) error {
	if expected := strings.TrimSpace(manifest.Artifact.SHA256); expected != "" && !strings.EqualFold(expected, digest) {
		return apperror.New(apperror.CodePluginInvalid, "manifest artifact sha256 mismatch")
	}
	if len(manifest.Signatures) == 0 {
		if s.allowUnsigned {
			return nil
		}
		return apperror.New(apperror.CodePluginInvalid, "plugin signature required")
	}
	keyring, err := loadKeyring(ctx, s.keyringPath)
	if err != nil {
		return err
	}
	vendor := strings.SplitN(manifest.ID, "/", 2)[0]
	for _, sig := range manifest.Signatures {
		for _, key := range keyring.Keys {
			if key.Vendor != vendor || key.KeyID != sig.KeyID || strings.TrimSpace(key.RevokedAt) != "" {
				continue
			}
			publicKey, err := base64.StdEncoding.DecodeString(key.PublicKey)
			if err != nil || len(publicKey) != ed25519.PublicKeySize {
				continue
			}
			signature, err := base64.StdEncoding.DecodeString(sig.Sig)
			if err != nil {
				continue
			}
			if ed25519.Verify(ed25519.PublicKey(publicKey), []byte(digest), signature) {
				return nil
			}
		}
	}
	return apperror.New(apperror.CodePluginInvalid, "plugin signature is not trusted")
}

func loadKeyring(_ context.Context, path string) (Keyring, error) {
	if strings.TrimSpace(path) == "" {
		return Keyring{}, apperror.New(apperror.CodePluginInvalid, "plugin keyring is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Keyring{}, apperror.Wrap(apperror.CodeConfig, "read plugin keyring failed", err)
	}
	var keyring Keyring
	if err := json.Unmarshal(data, &keyring); err != nil {
		return Keyring{}, apperror.Wrap(apperror.CodeConfig, "parse plugin keyring failed", err)
	}
	return keyring, nil
}

func parseManifest(content []byte) (Manifest, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err == nil {
		for _, file := range reader.File {
			if file.Name != "manifest.json" {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return Manifest{}, nil, apperror.Wrap(apperror.CodePluginInvalid, "open plugin manifest failed", err)
			}
			data, err := io.ReadAll(io.LimitReader(rc, 1<<20))
			closeErr := rc.Close()
			if err != nil {
				return Manifest{}, nil, apperror.Wrap(apperror.CodePluginInvalid, "read plugin manifest failed", err)
			}
			if closeErr != nil {
				return Manifest{}, nil, apperror.Wrap(apperror.CodePluginInvalid, "close plugin manifest failed", closeErr)
			}
			var manifest Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return Manifest{}, nil, apperror.Wrap(apperror.CodePluginInvalid, "parse plugin manifest failed", err)
			}
			return manifest, data, nil
		}
		return Manifest{}, nil, apperror.New(apperror.CodePluginInvalid, "manifest.json not found in artifact")
	}

	var manifest Manifest
	if jsonErr := json.Unmarshal(content, &manifest); jsonErr != nil {
		return Manifest{}, nil, apperror.Wrap(apperror.CodePluginInvalid, "artifact must be zip with manifest.json or manifest json", jsonErr)
	}
	return manifest, content, nil
}

func extractZipArtifact(content []byte, dir string) error {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil
	}
	for _, file := range reader.File {
		target, err := safeArtifactPath(dir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return apperror.Wrap(apperror.CodeConfig, "create plugin artifact directory failed", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return apperror.Wrap(apperror.CodeConfig, "create plugin artifact directory failed", err)
		}
		rc, err := file.Open()
		if err != nil {
			return apperror.Wrap(apperror.CodePluginInvalid, "open plugin artifact entry failed", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, 64<<20))
		closeErr := rc.Close()
		if readErr != nil {
			return apperror.Wrap(apperror.CodePluginInvalid, "read plugin artifact entry failed", readErr)
		}
		if closeErr != nil {
			return apperror.Wrap(apperror.CodePluginInvalid, "close plugin artifact entry failed", closeErr)
		}
		mode := file.Mode()
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return apperror.Wrap(apperror.CodeConfig, "write plugin artifact entry failed", err)
		}
	}
	return nil
}

func safeArtifactPath(root, name string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.Clean(name)))
	if err != nil {
		return "", err
	}
	if target != cleanRoot && !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) {
		return "", apperror.New(apperror.CodePluginInvalid, "plugin artifact contains unsafe path")
	}
	return target, nil
}

func validateManifest(manifest Manifest) error {
	if !pluginIDPattern.MatchString(manifest.ID) {
		return apperror.New(apperror.CodePluginInvalid, "plugin id must be namespaced as vendor/name")
	}
	if strings.TrimSpace(manifest.Name) == "" || strings.TrimSpace(manifest.Version) == "" {
		return apperror.New(apperror.CodePluginInvalid, "plugin name and version are required")
	}
	if manifest.Tier != 0 && manifest.Tier != 1 {
		return apperror.New(apperror.CodePluginInvalid, "only tier 0 and tier 1 plugins are supported")
	}
	if manifest.Capabilities.HookIsolation != "" && manifest.Capabilities.HookIsolation != "sequential" && manifest.Capabilities.HookIsolation != "independent" {
		return apperror.New(apperror.CodePluginInvalid, "invalid hookIsolation")
	}
	for _, item := range manifest.UI.Sidebar {
		if strings.TrimSpace(item.Label) == "" || !strings.HasPrefix(item.Route, "/plugins/"+manifest.ID+"/") {
			return apperror.New(apperror.CodePluginInvalid, "sidebar routes must be host plugin routes")
		}
	}
	return nil
}

func (s *Service) compatibilityCheck(manifest Manifest) error {
	if manifest.APIVersion != HostAPIVersion {
		return apperror.New(apperror.CodePluginInvalid, "unsupported plugin apiVersion")
	}
	if manifest.Tier == 1 && manifest.RPCVersion != HostRPCVersion {
		return apperror.New(apperror.CodePluginInvalid, "unsupported plugin rpcVersion")
	}
	if manifest.MinGoSiteVersion != "" && compareSemver(manifest.MinGoSiteVersion, s.hostVersion) > 0 {
		return apperror.New(apperror.CodePluginInvalid, "plugin requires a newer GoSite version")
	}
	return nil
}

func manifestFromRecord(record sqlite.PluginVersion) Manifest {
	var manifest Manifest
	_ = json.Unmarshal([]byte(record.ManifestJSON), &manifest)
	return manifest
}

func classifyValidationError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return FailureValidateTimeout
	}
	return FailureUnknown
}

func stableUninstallState(state string) bool {
	return state == sqlite.PluginStateInstalled || state == sqlite.PluginStateInstallFailed || state == sqlite.PluginStateEnableFailed
}

func safePluginDir(root, pluginID, version string) (string, error) {
	if !pluginIDPattern.MatchString(pluginID) || strings.Contains(version, "..") || strings.ContainsAny(version, `/\`) {
		return "", apperror.New(apperror.CodePluginInvalid, "invalid plugin path identity")
	}
	dir := filepath.Join(root, filepath.FromSlash(pluginID), version)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	cleanDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if cleanDir != cleanRoot && !strings.HasPrefix(cleanDir, cleanRoot+string(os.PathSeparator)) {
		return "", apperror.New(apperror.CodePluginInvalid, "invalid plugin storage path")
	}
	return cleanDir, nil
}

func resolvePluginCommand(root, command string) (string, []string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, apperror.New(apperror.CodePluginInvalid, "plugin command required")
	}
	if filepath.IsAbs(parts[0]) {
		return "", nil, apperror.New(apperror.CodePluginInvalid, "plugin command must be relative to artifact")
	}
	path, err := safeArtifactPath(root, parts[0])
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, apperror.Wrap(apperror.CodePluginInvalid, "plugin command not found", err)
	}
	if info.IsDir() {
		return "", nil, apperror.New(apperror.CodePluginInvalid, "plugin command points to a directory")
	}
	return path, parts[1:], nil
}

func ensureDiskWritable(dir string, requiredBytes int64) error {
	parent := dir
	for {
		if _, err := os.Stat(parent); err == nil {
			probe, err := os.CreateTemp(parent, ".gosite-plugin-write-*")
			if err != nil {
				return err
			}
			name := probe.Name()
			if _, err := probe.Write([]byte{0}); err != nil {
				_ = probe.Close()
				_ = os.Remove(name)
				return err
			}
			if err := probe.Close(); err != nil {
				_ = os.Remove(name)
				return err
			}
			_ = os.Remove(name)
			if requiredBytes > 0 {
				var stat syscall.Statfs_t
				if err := syscall.Statfs(parent, &stat); err == nil {
					available := int64(stat.Bavail) * int64(stat.Bsize)
					if available < requiredBytes*2 {
						return fmt.Errorf("insufficient plugin storage space: need at least %d bytes, available %d", requiredBytes*2, available)
					}
				}
			}
			return nil
		}
		next := filepath.Dir(parent)
		if next == parent {
			return fmt.Errorf("plugin storage root does not exist")
		}
		parent = next
	}
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
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
