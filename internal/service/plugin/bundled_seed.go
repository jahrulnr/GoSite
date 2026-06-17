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
		if err := s.seedBundledEntry(ctx, entry); err != nil {
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
	content, err := s.bundled.LoadArtifact(entry)
	if err != nil {
		return sqlite.PluginVersion{}, apperror.Wrap(apperror.CodeConfig, "read bundled artifact failed", err)
	}
	return s.installBundled(ctx, entry, content)
}

func (s *Service) seedBundledEntry(ctx context.Context, entry bundled.Entry) error {
	content, err := s.bundled.LoadArtifact(entry)
	if err != nil {
		return err
	}
	manifest, _, err := parseManifest(content)
	if err != nil {
		return err
	}
	digest := sha256Hex(content)
	existing, findErr := s.repo.Find(ctx, manifest.ID, manifest.Version)
	if findErr == nil {
		if existing.ArtifactDigest == digest &&
			existing.State != sqlite.PluginStateUninstalled &&
			existing.State != sqlite.PluginStateInstallFailed {
			return nil
		}
		if existing.State == sqlite.PluginStateUninstalled && !entry.Restorable {
			return nil
		}
		if existing.ArtifactDigest == digest && existing.State == sqlite.PluginStateInstalled {
			return nil
		}
	}
	_, err = s.installBundled(ctx, entry, content)
	return err
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
		PermissionsAck: entry.PermissionsPreAck,
	})
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func isBundledInstall(in InstallInput) bool {
	return in.Provenance != nil && strings.TrimSpace(in.Provenance.SourceType) == bundledSourceType
}
