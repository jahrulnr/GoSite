# Wave PLUGIN-G — Remote distribution

**Status:** Complete (shipped **v1.3.1**)

Tracker for sequence 20. Full checklist: [../sequences/20-plugin-remote-distribution-impl.md](../sequences/20-plugin-remote-distribution-impl.md).

## Scope

Remote plugin install: URL, GitHub/GitLab release, git-ref (tier-0), bundled catalog, optional Docker build (G2b), provenance + install log, keyring UI, CLI.

## Packages

| Area | Path |
|------|------|
| Remote service | `internal/service/plugin/remote/` |
| Catalog | `internal/service/plugin/catalog/` |
| Install log / OpLock | `internal/service/plugin/installlog.go`, `oplock.go` |
| HTTP handlers | `internal/delivery/http/handler/plugin.go`, `plugin_catalog.go` |
| CLI | `internal/cli/plugin.go` |
| Migration | `migrations/007_plugin_provenance.sql` |
| Panel | `web/src/views/Plugins.tsx`, `PluginsKeyring.tsx`, `PluginSettingsCard.tsx` |

## Prior waves (plugin platform)

| Wave | Sequence | Status |
|------|----------|--------|
| Installer A–F | [19-plugin-installer](../sequences/19-plugin-installer.md) | ✅ v1.3.0 |
| Remote G | [20-plugin-remote-distribution](../sequences/20-plugin-remote-distribution.md) | ✅ v1.3.1 |

ADR: [../architecture/plugin-platform.md](../architecture/plugin-platform.md)
