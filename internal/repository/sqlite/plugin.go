package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	PluginStateInstalling    = "installing"
	PluginStateInstalled     = "installed"
	PluginStateInstallFailed = "install_failed"
	PluginStateEnabling      = "enabling"
	PluginStateEnabled       = "enabled"
	PluginStateEnableFailed  = "enable_failed"
	PluginStateDisabling     = "disabling"
	PluginStateUninstalling  = "uninstalling"
	PluginStateUninstalled   = "uninstalled"
)

var ErrPluginVersionExists = errors.New("plugin version already exists")

// PluginVersion is one installed artifact version in the plugin registry.
type PluginVersion struct {
	ID               int64
	PluginID         string
	Version          string
	Name             string
	Tier             int
	APIVersion       string
	MinGoSiteVersion string
	RPCVersion       string
	ConfigVersion    string
	ManifestJSON     string
	CapabilitiesJSON string
	UIJSON           string
	ArtifactDigest   string
	ArtifactPath     string
	State            string
	FailureClass     string
	FailureMessage   string
	FailureAt        *time.Time
	ConfigDeletedAt  *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// Provenance (migration 007)
	SourceType           string
	SourceRef            string
	ResolvedURL          string
	ResolvedDigest       string
	SigningKeyID         string
	SourceCommit         string
	BuilderImageDigest   string
	SourceRepository     string
	InstallPath          string
	PermissionsAckAt     *time.Time
	PermissionsAckedCaps string
	InstallLog           string
}

// PluginRepository persists plugin registry records.
type PluginRepository struct {
	db *sql.DB
}

// NewPluginRepository returns a plugin registry backed by db.
func NewPluginRepository(db *sql.DB) *PluginRepository {
	return &PluginRepository{db: db}
}

// Create inserts a plugin version record.
func (r *PluginRepository) Create(ctx context.Context, p PluginVersion) (PluginVersion, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO plugin_versions (
			plugin_id, version, name, tier, api_version, min_gosite_version, rpc_version,
			config_version, manifest_json, capabilities_json, ui_json, artifact_digest,
			artifact_path, state, failure_class, failure_message, failure_at,
			config_deleted_at, created_at, updated_at,
			source_type, source_ref, resolved_url, resolved_digest, signing_key_id,
			source_commit, builder_image_digest, source_repository, install_path,
			permissions_ack_at, permissions_acked_caps, install_log
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.PluginID, p.Version, p.Name, p.Tier, p.APIVersion, p.MinGoSiteVersion, p.RPCVersion,
		p.ConfigVersion, p.ManifestJSON, p.CapabilitiesJSON, p.UIJSON, p.ArtifactDigest,
		p.ArtifactPath, p.State, p.FailureClass, p.FailureMessage, nullTime(p.FailureAt),
		nullTime(p.ConfigDeletedAt), now, now,
		p.SourceType, p.SourceRef, p.ResolvedURL, p.ResolvedDigest, p.SigningKeyID,
		p.SourceCommit, p.BuilderImageDigest, p.SourceRepository, p.InstallPath,
		nullTime(p.PermissionsAckAt), p.PermissionsAckedCaps, p.InstallLog)
	if err != nil {
		return PluginVersion{}, fmt.Errorf("insert plugin version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return PluginVersion{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByRowID(ctx, id)
}

// CreateOrRetryInstall inserts a fresh installing record or reopens install_failed.
func (r *PluginRepository) CreateOrRetryInstall(ctx context.Context, p PluginVersion) (PluginVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PluginVersion{}, fmt.Errorf("begin plugin install tx: %w", err)
	}
	defer tx.Rollback()

	var id int64
	var state string
	err = tx.QueryRowContext(ctx, `
		SELECT id, state FROM plugin_versions
		WHERE plugin_id = ? AND version = ?
	`, p.PluginID, p.Version).Scan(&id, &state)
	now := time.Now().UTC()
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO plugin_versions (
				plugin_id, version, name, tier, api_version, min_gosite_version, rpc_version,
				config_version, manifest_json, capabilities_json, ui_json, artifact_digest,
				artifact_path, state, failure_class, failure_message, failure_at,
				config_deleted_at, created_at, updated_at,
				source_type, source_ref, resolved_url, resolved_digest, signing_key_id,
				source_commit, builder_image_digest, source_repository, install_path,
				permissions_ack_at, permissions_acked_caps, install_log
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', NULL, NULL, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, p.PluginID, p.Version, p.Name, p.Tier, p.APIVersion, p.MinGoSiteVersion, p.RPCVersion,
			p.ConfigVersion, p.ManifestJSON, p.CapabilitiesJSON, p.UIJSON, p.ArtifactDigest,
			p.ArtifactPath, PluginStateInstalling, now, now,
			p.SourceType, p.SourceRef, p.ResolvedURL, p.ResolvedDigest, p.SigningKeyID,
			p.SourceCommit, p.BuilderImageDigest, p.SourceRepository, p.InstallPath,
			nullTime(p.PermissionsAckAt), p.PermissionsAckedCaps, p.InstallLog)
		if err != nil {
			return PluginVersion{}, fmt.Errorf("insert plugin version: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return PluginVersion{}, fmt.Errorf("last insert id: %w", err)
		}
	case err != nil:
		return PluginVersion{}, fmt.Errorf("find existing plugin version: %w", err)
	case state != PluginStateInstallFailed:
		return PluginVersion{}, ErrPluginVersionExists
	default:
		_, err := tx.ExecContext(ctx, `
			UPDATE plugin_versions
			SET name = ?, tier = ?, api_version = ?, min_gosite_version = ?, rpc_version = ?,
				config_version = ?, manifest_json = ?, capabilities_json = ?, ui_json = ?,
				artifact_digest = ?, artifact_path = ?, state = ?, failure_class = '',
				failure_message = '', failure_at = NULL, config_deleted_at = NULL, updated_at = ?,
				source_type = ?, source_ref = ?, resolved_url = ?, resolved_digest = ?,
				signing_key_id = ?, source_commit = ?, builder_image_digest = ?,
				source_repository = ?, install_path = ?, permissions_ack_at = ?,
				permissions_acked_caps = ?, install_log = ?
			WHERE id = ?
		`, p.Name, p.Tier, p.APIVersion, p.MinGoSiteVersion, p.RPCVersion,
			p.ConfigVersion, p.ManifestJSON, p.CapabilitiesJSON, p.UIJSON, p.ArtifactDigest,
			p.ArtifactPath, PluginStateInstalling, now,
			p.SourceType, p.SourceRef, p.ResolvedURL, p.ResolvedDigest, p.SigningKeyID,
			p.SourceCommit, p.BuilderImageDigest, p.SourceRepository, p.InstallPath,
			nullTime(p.PermissionsAckAt), p.PermissionsAckedCaps, p.InstallLog, id)
		if err != nil {
			return PluginVersion{}, fmt.Errorf("retry plugin install: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return PluginVersion{}, fmt.Errorf("commit plugin install tx: %w", err)
	}
	return r.FindByRowID(ctx, id)
}

// List returns all plugin records.
func (r *PluginRepository) List(ctx context.Context) ([]PluginVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions
		ORDER BY plugin_id, version DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list plugin versions: %w", err)
	}
	defer rows.Close()

	var plugins []PluginVersion
	for rows.Next() {
		p, err := scanPluginVersion(rows)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// FindByRowID loads a plugin record by row id.
func (r *PluginRepository) FindByRowID(ctx context.Context, id int64) (PluginVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions WHERE id = ?
	`, id)
	return scanPluginVersion(row)
}

// Find loads a plugin version by plugin id and version.
func (r *PluginRepository) Find(ctx context.Context, pluginID, version string) (PluginVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions WHERE plugin_id = ? AND version = ?
	`, pluginID, version)
	return scanPluginVersion(row)
}

// FindEnabled loads the currently enabled version for a plugin id.
func (r *PluginRepository) FindEnabled(ctx context.Context, pluginID string) (PluginVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions WHERE plugin_id = ? AND state = ?
	`, pluginID, PluginStateEnabled)
	return scanPluginVersion(row)
}

// ListEnabled returns every enabled plugin version.
func (r *PluginRepository) ListEnabled(ctx context.Context) ([]PluginVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions
		WHERE state = ?
		ORDER BY plugin_id ASC, version DESC
	`, PluginStateEnabled)
	if err != nil {
		return nil, fmt.Errorf("list enabled plugin versions: %w", err)
	}
	defer rows.Close()

	var plugins []PluginVersion
	for rows.Next() {
		p, err := scanPluginVersion(rows)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// ListInstallable returns stable versions that can be enabled.
func (r *PluginRepository) ListInstallable(ctx context.Context, pluginID string) ([]PluginVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+pluginVersionColumns+`
		FROM plugin_versions
		WHERE plugin_id = ? AND state IN (?, ?)
		ORDER BY created_at DESC, id DESC
	`, pluginID, PluginStateInstalled, PluginStateEnableFailed)
	if err != nil {
		return nil, fmt.Errorf("list installable plugin versions: %w", err)
	}
	defer rows.Close()

	var plugins []PluginVersion
	for rows.Next() {
		p, err := scanPluginVersion(rows)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// SetState updates lifecycle state and clears any previous failure metadata.
func (r *PluginRepository) SetState(ctx context.Context, pluginID, version, state string) (PluginVersion, error) {
	return r.setState(ctx, pluginID, version, state, "", "", nil)
}

// SetFailure updates lifecycle state and stores failure metadata.
func (r *PluginRepository) SetFailure(ctx context.Context, pluginID, version, state, failureClass, failureMessage string) (PluginVersion, error) {
	now := time.Now().UTC()
	return r.setState(ctx, pluginID, version, state, failureClass, failureMessage, &now)
}

// MarkConfigDeleted soft-deletes retained config/secrets after uninstall.
func (r *PluginRepository) MarkConfigDeleted(ctx context.Context, pluginID, version string) (PluginVersion, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE plugin_versions
		SET state = ?, failure_class = '', failure_message = '', failure_at = NULL,
			config_deleted_at = ?, updated_at = ?
		WHERE plugin_id = ? AND version = ?
	`, PluginStateUninstalled, now, now, pluginID, version)
	if err != nil {
		return PluginVersion{}, fmt.Errorf("mark plugin config deleted: %w", err)
	}
	return r.Find(ctx, pluginID, version)
}

// DeleteUninstalled permanently removes an already-uninstalled registry row.
func (r *PluginRepository) DeleteUninstalled(ctx context.Context, pluginID, version string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM plugin_versions
		WHERE plugin_id = ? AND version = ? AND state = ?
	`, pluginID, version, PluginStateUninstalled)
	if err != nil {
		return fmt.Errorf("delete uninstalled plugin version: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SetInstallLog updates the install audit log for a plugin version.
func (r *PluginRepository) SetInstallLog(ctx context.Context, pluginID, version, installLog string) (PluginVersion, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE plugin_versions SET install_log = ?, updated_at = ?
		WHERE plugin_id = ? AND version = ?
	`, installLog, now, pluginID, version)
	if err != nil {
		return PluginVersion{}, fmt.Errorf("set install log: %w", err)
	}
	return r.Find(ctx, pluginID, version)
}

func (r *PluginRepository) setState(ctx context.Context, pluginID, version, state, failureClass, failureMessage string, failureAt *time.Time) (PluginVersion, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE plugin_versions
		SET state = ?, failure_class = ?, failure_message = ?, failure_at = ?, updated_at = ?
		WHERE plugin_id = ? AND version = ?
	`, state, failureClass, failureMessage, nullTime(failureAt), now, pluginID, version)
	if err != nil {
		return PluginVersion{}, fmt.Errorf("set plugin state: %w", err)
	}
	return r.Find(ctx, pluginID, version)
}

type pluginScanner interface {
	Scan(dest ...any) error
}

func scanPluginVersion(row pluginScanner) (PluginVersion, error) {
	var p PluginVersion
	var failureAt, configDeletedAt, createdAt, updatedAt, permissionsAckAt sql.NullTime
	err := row.Scan(&p.ID, &p.PluginID, &p.Version, &p.Name, &p.Tier, &p.APIVersion,
		&p.MinGoSiteVersion, &p.RPCVersion, &p.ConfigVersion, &p.ManifestJSON,
		&p.CapabilitiesJSON, &p.UIJSON, &p.ArtifactDigest, &p.ArtifactPath,
		&p.State, &p.FailureClass, &p.FailureMessage, &failureAt, &configDeletedAt,
		&createdAt, &updatedAt,
		&p.SourceType, &p.SourceRef, &p.ResolvedURL, &p.ResolvedDigest, &p.SigningKeyID,
		&p.SourceCommit, &p.BuilderImageDigest, &p.SourceRepository, &p.InstallPath,
		&permissionsAckAt, &p.PermissionsAckedCaps, &p.InstallLog)
	if err != nil {
		return PluginVersion{}, err
	}
	if failureAt.Valid {
		t := failureAt.Time
		p.FailureAt = &t
	}
	if configDeletedAt.Valid {
		t := configDeletedAt.Time
		p.ConfigDeletedAt = &t
	}
	if createdAt.Valid {
		p.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		p.UpdatedAt = updatedAt.Time
	}
	if permissionsAckAt.Valid {
		t := permissionsAckAt.Time
		p.PermissionsAckAt = &t
	}
	return p, nil
}
