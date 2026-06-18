package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/bundled"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const bundledSourceType = "bundled"

// SeedBundled registers official plugins from the bundled index when artifacts are present.
func (s *Service) SeedBundled(ctx context.Context) error {
	if !s.bundledEnabled || s.bundled == nil {
		return nil
	}
	index, err := s.bundled.LoadIndex()
	if err != nil {
		return apperror.Wrap(apperror.CodeConfig, "load bundled plugin index failed", err)
	}
	for _, entry := range index.Plugins {
		if _, err := s.seedBundledEntry(ctx, entry, false); err != nil {
			if errors.Is(err, bundled.ErrArtifactsUnavailable) {
				slog.Debug("bundled plugin artifact unavailable", "plugin_id", entry.PluginID)
				continue
			}
			slog.Warn("bundled plugin seed failed", "plugin_id", entry.PluginID, "err", err)
		}
	}
	if s.bundledAutoEnable && s.appEnv != "production" {
		for _, entry := range index.Plugins {
			records, listErr := s.repo.ListInstallable(ctx, entry.PluginID)
			if listErr != nil || len(records) == 0 {
				continue
			}
			latest := records[0]
			for _, row := range records[1:] {
				if compareSemver(row.Version, latest.Version) > 0 {
					latest = row
				}
			}
			if latest.State != sqlite.PluginStateInstalled {
				continue
			}
			if _, enableErr := s.Enable(ctx, entry.PluginID, latest.Version); enableErr != nil {
				slog.Warn("bundled auto-enable failed", "plugin_id", entry.PluginID, "err", enableErr)
			}
		}
	}
	return nil
}

// RestoreBundled re-installs one official plugin from the bundled artifact directory.
func (s *Service) RestoreBundled(ctx context.Context, pluginID string) (sqlite.PluginVersion, error) {
	if s.bundled == nil {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeInvalidInput, "bundled plugins are not configured")
	}
	entry, err := s.bundled.Entry(pluginID)
	if err != nil {
		if errors.Is(err, bundled.ErrNotFound) {
			return sqlite.PluginVersion{}, apperror.New(apperror.CodeNotFound, "plugin is not a bundled official plugin")
		}
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "lookup bundled plugin failed", err)
	}
	if !entry.Restorable {
		return sqlite.PluginVersion{}, apperror.New(apperror.CodeInvalidInput, "bundled plugin is not restorable")
	}
	return s.seedBundledEntry(ctx, entry, true)
}

func (s *Service) seedBundledEntry(ctx context.Context, entry bundled.Entry, force bool) (sqlite.PluginVersion, error) {
	content, err := s.bundled.LoadArtifact(entry)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	manifest, _, err := parseManifest(content)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	digest := sha256Hex(content)
	existing, findErr := s.repo.Find(ctx, manifest.ID, manifest.Version)
	if !s.shouldSeedBundled(ctx, entry, existing, findErr, digest, force) {
		if findErr == nil {
			return existing, nil
		}
		return sqlite.PluginVersion{}, nil
	}
	record, err := s.installBundled(ctx, entry, content)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	if force || (findErr == nil && existing.State == sqlite.PluginStateUninstalled) {
		slog.Info("bundled plugin restored", "plugin_id", record.PluginID, "version", record.Version)
	} else {
		slog.Info("bundled plugin seeded", "plugin_id", record.PluginID, "version", record.Version, "state", record.State)
	}
	return record, nil
}

func (s *Service) shouldSeedBundled(ctx context.Context, entry bundled.Entry, existing sqlite.PluginVersion, findErr error, digest string, force bool) bool {
	if force {
		return true
	}
	if findErr != nil {
		return true
	}
	if existing.ArtifactDigest != digest {
		return true
	}
	if existing.State == sqlite.PluginStateInstallFailed {
		return true
	}
	if existing.State == sqlite.PluginStateUninstalled && entry.Restorable {
		return true
	}
	if entry.Restorable && s.bundledNeedsRestore(ctx, entry, existing.PluginID) {
		return true
	}
	return false
}

func (s *Service) bundledNeedsRestore(ctx context.Context, entry bundled.Entry, pluginID string) bool {
	if !entry.Restorable {
		return false
	}
	installable, err := s.repo.ListInstallable(ctx, pluginID)
	if err != nil {
		return false
	}
	return len(installable) == 0
}

func bundledPermissionsPreAck(entry bundled.Entry) bool {
	if strings.HasPrefix(entry.PluginID, "gosite/") {
		return true
	}
	return entry.PermissionsPreAck
}

func (s *Service) installBundled(ctx context.Context, entry bundled.Entry, content []byte) (sqlite.PluginVersion, error) {
	return s.Install(ctx, InstallInput{
		Content: content,
		Provenance: &InstallProvenance{
			SourceType:       bundledSourceType,
			SourceRef:        "gosite@" + s.hostVersion,
			ResolvedDigest:   sha256Hex(content),
			SourceRepository: "https://github.com/jahrulnr/gosite",
			InstallPath:      bundledSourceType,
		},
		PermissionsAck: bundledPermissionsPreAck(entry),
	})
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func isBundledInstall(in InstallInput) bool {
	return in.Provenance != nil && strings.TrimSpace(in.Provenance.SourceType) == bundledSourceType
}
