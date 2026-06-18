import { useEffect, useMemo, useState } from 'preact/hooks';
import { plugins } from '../api/endpoints';
import type { PluginCatalogEntry, PluginInstallSource, PluginResolvePreview, PluginState, PluginVersion } from '../api/types';
import { IconArrowUp, IconPlay, IconPlug, IconRefresh, IconShield, IconTrash } from '../components/Icons';
import { Page, Stat } from '../components/Layout';
import { AsyncView, Badge, EmptyState, Field, InlineNotice, Modal, Spinner } from '../components/Ui';
import { formatBytes, formatDate, formatRelative } from '../lib/format';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { navigate } from '../lib/router';
import { useStore } from '../lib/store';
import { PluginsKeyringPanel } from './PluginsKeyring';
import { PluginMCPIntegrationView } from './PluginMCPIntegration';

type InstallMode = 'artifact' | 'url' | 'github' | 'gitlab' | 'catalog' | 'manifest';

const HOST_CRITICAL_PERMS = new Set(['docker:manage', 'nginx:modify', 'ssl:issue', 'filesystem:write']);

interface PluginGroup {
  id: string;
  name: string;
  versions: PluginVersion[];
  enabled?: PluginVersion;
  latest?: PluginVersion;
}

function semverParts(version: string) {
  return version.replace(/^v/, '').split('-', 1)[0].split('.').slice(0, 3).map((part) => Number.parseInt(part, 10) || 0);
}

function compareSemverDesc(a: string, b: string) {
  const left = semverParts(a);
  const right = semverParts(b);
  for (let i = 0; i < 3; i += 1) {
    const delta = (right[i] ?? 0) - (left[i] ?? 0);
    if (delta !== 0) return delta;
  }
  return b.localeCompare(a);
}

function groupPlugins(rows: PluginVersion[]): PluginGroup[] {
  const map = new Map<string, PluginVersion[]>();
  for (const row of rows.filter((item) => item.state !== 'uninstalled')) {
    map.set(row.plugin_id, [...(map.get(row.plugin_id) ?? []), row]);
  }
  return [...map.entries()]
    .map(([id, versions]) => {
      const sorted = [...versions].sort((a, b) => compareSemverDesc(a.version, b.version));
      return {
        id,
        name: sorted[0]?.name ?? id,
        versions: sorted,
        enabled: sorted.find((item) => item.state === 'enabled'),
        latest: sorted[0],
      };
    })
    .sort((a, b) => a.id.localeCompare(b.id));
}

function stateKind(state: PluginState): 'ok' | 'off' | 'warn' | 'danger' | 'info' {
  if (state === 'enabled') return 'ok';
  if (state === 'installed') return 'info';
  if (state.endsWith('_failed')) return 'danger';
  if (state === 'enabling' || state === 'disabling' || state === 'installing' || state === 'uninstalling') return 'warn';
  return 'off';
}

function stateLabel(state: string) {
  return state.replace(/_/g, ' ');
}

function stableForUninstall(state: PluginState) {
  return state === 'installed' || state === 'install_failed' || state === 'enable_failed';
}

function capabilityLabels(plugin: PluginVersion) {
  const caps = plugin.capabilities ?? {};
  const labels: string[] = [];
  if (Array.isArray(caps.hooks) && caps.hooks.length) labels.push(`${caps.hooks.length} hooks`);
  if (caps.loggingSink) labels.push('logging');
  if (caps.uiSidebar) labels.push('sidebar');
  if (caps.configSchema) labels.push('config');
  if (caps.rulesAndRoles) labels.push(`rules:${caps.rulesAndRoles}`);
  return labels;
}

function sidebarEntries(plugin: PluginVersion) {
  return plugin.ui?.sidebar ?? plugin.manifest?.ui?.sidebar ?? [];
}

function permissions(plugin: PluginVersion) {
  return plugin.manifest?.permissions ?? [];
}

function sourceDisplayLabel(sourceType?: string) {
  if (!sourceType || sourceType === 'upload') return 'upload';
  if (sourceType === 'bundled') return 'built-in';
  if (sourceType === 'github-release') return 'github';
  if (sourceType === 'url') return 'url';
  return sourceType;
}

function isBundledPlugin(plugin: PluginVersion) {
  return plugin.source_type === 'bundled';
}

function truncateText(value: string, max = 40) {
  if (value.length <= max) return value;
  return `${value.slice(0, max - 1)}…`;
}

function truncateSha256(digest: string) {
  if (digest.length <= 16) return digest;
  return `${digest.slice(0, 8)}…${digest.slice(-8)}`;
}

function hasDistribution(plugin: PluginVersion) {
  const sourceType = plugin.source_type;
  return !!sourceType && sourceType !== 'upload';
}

function parseGitHubReleasePaste(text: string) {
  const re = /github\.com\/([^/]+\/[^/]+)\/releases\/tag\/(.+?)\/?$/i;
  const match = re.exec(text.trim());
  if (!match) return {};
  return { repo: match[1], tag: decodeURIComponent(match[2]) };
}

function InstallPreviewCard({
  preview,
  permissionsAck,
  onPermissionsAck,
}: Readonly<{
  preview: PluginResolvePreview;
  permissionsAck: boolean;
  onPermissionsAck: (checked: boolean) => void;
}>) {
  const visibleHooks = preview.hooks.slice(0, 5);
  const extraHooks = preview.hooks.length - visibleHooks.length;
  return (
    <div class="plugin-detail-card plugin-install-preview">
      <div class="row wrap" style="margin-bottom:8px;">
        <span class="mono">{preview.plugin_id}</span>
        <Badge kind="info">v{preview.version}</Badge>
        <Badge kind="info">tier {preview.tier}</Badge>
        {preview.signed ? <Badge kind="ok">signed ✓</Badge> : <Badge kind="warn">unsigned ✗</Badge>}
      </div>
      <dl class="plugin-facts">
        <div><dt>Min GoSite</dt><dd>{preview.minGoSiteVersion || '—'}</dd></div>
        <div><dt>SHA-256</dt><dd class="mono">{truncateSha256(preview.sha256)}</dd></div>
        <div><dt>Size</dt><dd>{formatBytes(preview.size)}</dd></div>
        <div><dt>Source</dt><dd>{preview.source_type} · {preview.source_ref}</dd></div>
      </dl>
      <div style="margin-top:12px;">
        <div class="dim" style="margin-bottom:6px;">Hooks ({preview.hooks.length})</div>
        <div class="chip-row">
          {visibleHooks.map((hook) => <span key={hook} class="chip mono">{hook}</span>)}
          {extraHooks > 0 && <span class="chip dim">{extraHooks} more</span>}
          {preview.hooks.length === 0 && <span class="dim">—</span>}
        </div>
      </div>
      <div style="margin-top:12px;">
        <div class="dim" style="margin-bottom:6px;">Permissions ({preview.permissions.length})</div>
        <div class="chip-row">
          {preview.permissions.map((perm) => (
            HOST_CRITICAL_PERMS.has(perm)
              ? <Badge key={perm} kind="warn">{perm}</Badge>
              : <span key={perm} class="chip mono">{perm}</span>
          ))}
          {preview.permissions.length === 0 && <span class="dim">—</span>}
        </div>
      </div>
      <label class="row wrap" style="margin-top:16px; gap:8px; cursor:pointer;">
        <input
          type="checkbox"
          checked={permissionsAck}
          onChange={(event) => onPermissionsAck((event.target as HTMLInputElement).checked)}
        />
        <span>I understand the permissions this plugin requests</span>
      </label>
    </div>
  );
}

function InstallModal({ onClose, onInstalled }: Readonly<{ onClose: () => void; onInstalled: () => void }>) {
  const { meta, toast } = useStore();
  const settingsState = useAsync(() => plugins.installSettings(), []);
  const remoteEnabled = settingsState.data?.remote_install_enabled ?? false;
  const [mode, setMode] = useState<InstallMode>('artifact');
  const [file, setFile] = useState<File>();
  const [sha256, setSHA256] = useState('');
  const [manifest, setManifest] = useState('');
  const [url, setUrl] = useState('');
  const [urlSha256, setUrlSha256] = useState('');
  const [repo, setRepo] = useState('');
  const [tag, setTag] = useState('');
  const [gitlabRepo, setGitlabRepo] = useState('');
  const [gitlabTag, setGitlabTag] = useState('');
  const [catalogQuery, setCatalogQuery] = useState('');
  const [catalogEntry, setCatalogEntry] = useState<PluginCatalogEntry>();
  const catalogState = useAsync(() => plugins.catalog(catalogQuery.trim() || undefined), [catalogQuery, mode]);
  const registryState = useAsync(() => plugins.list(), []);
  const [preview, setPreview] = useState<PluginResolvePreview>();
  const [permissionsAck, setPermissionsAck] = useState(false);
  const installFile = useAction(plugins.installFile);
  const installManifest = useAction(plugins.installManifest);
  const resolveInstall = useAction(plugins.resolveInstall);
  const installRemote = useAction(plugins.installRemote);
  const busy = installFile.loading || installManifest.loading || installRemote.loading;
  const resolveBusy = resolveInstall.loading;
  const error = installFile.error ?? installManifest.error ?? installRemote.error ?? resolveInstall.error;
  const isRemoteMode = mode === 'url' || mode === 'github' || mode === 'gitlab' || mode === 'catalog';
  const catalogBundled = mode === 'catalog' && catalogEntry?.bundled;
  const catalogInstalled = catalogEntry
    ? (registryState.data ?? []).some((row) => row.plugin_id === catalogEntry.plugin_id && row.state !== 'uninstalled')
    : false;

  useEffect(() => {
    if (!remoteEnabled && isRemoteMode) setMode('artifact');
  }, [remoteEnabled, isRemoteMode]);

  const switchMode = (next: InstallMode) => {
    setMode(next);
    setPreview(undefined);
    setPermissionsAck(false);
    setCatalogEntry(undefined);
  };

  const resolve = async () => {
    try {
      let source: PluginInstallSource;
      if (mode === 'url') {
        source = { type: 'url', url: url.trim(), sha256: urlSha256.trim() };
      } else if (mode === 'gitlab') {
        source = { type: 'gitlab-release', repo: gitlabRepo.trim(), tag: gitlabTag.trim() };
      } else if (mode === 'catalog') {
        if (!catalogEntry) throw new Error('Select a catalog entry first');
        if (catalogEntry.bundled) throw new Error('Built-in plugins are seeded on init — enable from the registry');
        source = catalogEntry.source;
      } else {
        source = { type: 'github-release', repo: repo.trim(), tag: tag.trim() };
      }
      const result = await resolveInstall.run(source);
      setPreview(result?.preview);
      setPermissionsAck(false);
    } catch (err) {
      setPreview(undefined);
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      if (mode === 'artifact') {
        if (!file) throw new Error('Choose an artifact first');
        await installFile.run(file, sha256.trim() || undefined);
      } else if (mode === 'manifest') {
        JSON.parse(manifest);
        await installManifest.run(manifest, sha256.trim() || undefined);
      } else if (mode === 'url') {
        if (!preview) throw new Error('Resolve the URL before installing');
        if (!permissionsAck) throw new Error('Acknowledge plugin permissions first');
        await installRemote.run(
          { type: 'url', url: url.trim(), sha256: urlSha256.trim() },
          true,
          preview.resolveToken,
        );
      } else if (mode === 'gitlab') {
        if (!preview) throw new Error('Resolve the release before installing');
        if (!permissionsAck) throw new Error('Acknowledge plugin permissions first');
        await installRemote.run(
          { type: 'gitlab-release', repo: gitlabRepo.trim(), tag: gitlabTag.trim() },
          true,
          preview.resolveToken,
        );
      } else if (mode === 'catalog') {
        if (catalogEntry?.bundled) throw new Error('Built-in plugins are seeded on init — enable from the registry');
        if (!preview || !catalogEntry) throw new Error('Resolve the catalog entry before installing');
        if (!permissionsAck) throw new Error('Acknowledge plugin permissions first');
        await installRemote.run(catalogEntry.source, true, preview.resolveToken);
      } else {
        if (!preview) throw new Error('Resolve the release before installing');
        if (!permissionsAck) throw new Error('Acknowledge plugin permissions first');
        await installRemote.run(
          { type: 'github-release', repo: repo.trim(), tag: tag.trim() },
          true,
          preview.resolveToken,
        );
      }
      toast('Plugin installed');
      onInstalled();
      onClose();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const installDisabled = busy || catalogBundled || (isRemoteMode && (!preview || !permissionsAck || (mode === 'catalog' && !catalogEntry)));

  return (
    <Modal
      title="Install plugin"
      onClose={onClose}
      wide
      footer={
        <>
          {error && <InlineNotice kind="danger">{humanizeError(error, meta)}</InlineNotice>}
          <button type="button" class="btn ghost" onClick={onClose}>Cancel</button>
          <button type="submit" form="plugin-install-form" class="btn primary" disabled={installDisabled}>
            {busy ? <Spinner /> : <><IconArrowUp /> Install</>}
          </button>
        </>
      }
    >
      <form id="plugin-install-form" onSubmit={submit}>
        {!settingsState.loading && !remoteEnabled && (
          <div style="margin-bottom:12px;">
            <InlineNotice kind="info">
              Remote install (URL and GitHub) is disabled on this host. Use Artifact or Manifest JSON.
            </InlineNotice>
          </div>
        )}
        <div class="tabs">
          <button type="button" class={mode === 'artifact' ? 'active' : ''} onClick={() => switchMode('artifact')}>Artifact</button>
          {remoteEnabled && (
            <button type="button" class={mode === 'url' ? 'active' : ''} onClick={() => switchMode('url')}>URL</button>
          )}
          {remoteEnabled && (
            <button type="button" class={mode === 'github' ? 'active' : ''} onClick={() => switchMode('github')}>GitHub</button>
          )}
          {remoteEnabled && (
            <button type="button" class={mode === 'gitlab' ? 'active' : ''} onClick={() => switchMode('gitlab')}>GitLab</button>
          )}
          {remoteEnabled && (
            <button type="button" class={mode === 'catalog' ? 'active' : ''} onClick={() => switchMode('catalog')}>Catalog</button>
          )}
          <button type="button" class={mode === 'manifest' ? 'active' : ''} onClick={() => switchMode('manifest')}>Manifest JSON</button>
        </div>
        {mode === 'artifact' && (
          <Field label="Artifact">
            <input
              class="input"
              type="file"
              onChange={(event) => setFile((event.target as HTMLInputElement).files?.[0])}
              required
            />
          </Field>
        )}
        {mode === 'manifest' && (
          <Field label="Manifest JSON">
            <textarea
              class="textarea plugin-manifest-input"
              value={manifest}
              onInput={(event) => setManifest((event.target as HTMLTextAreaElement).value)}
              required
            />
          </Field>
        )}
        {mode === 'url' && (
          <>
            <Field label="HTTPS URL" hint="Direct link to a plugin artifact.">
              <input
                class="input mono"
                type="url"
                value={url}
                onInput={(event) => {
                  setUrl((event.target as HTMLInputElement).value);
                  setPreview(undefined);
                }}
                required
              />
            </Field>
            <Field label="SHA-256" hint="Required digest check for remote URL installs.">
              <input
                class="input mono"
                value={urlSha256}
                onInput={(event) => {
                  setUrlSha256((event.target as HTMLInputElement).value);
                  setPreview(undefined);
                }}
                required
              />
            </Field>
            <div class="row wrap" style="margin-bottom:12px;">
              <button type="button" class="btn ghost" disabled={resolveBusy || !url.trim() || !urlSha256.trim()} onClick={() => void resolve()}>
                {resolveBusy ? <Spinner /> : 'Resolve'}
              </button>
            </div>
            {preview && (
              <InstallPreviewCard preview={preview} permissionsAck={permissionsAck} onPermissionsAck={setPermissionsAck} />
            )}
          </>
        )}
        {mode === 'github' && (
          <>
            <Field label="Repository" hint="owner/repo — paste a GitHub release URL to auto-fill.">
              <input
                class="input mono"
                value={repo}
                onInput={(event) => {
                  setRepo((event.target as HTMLInputElement).value);
                  setPreview(undefined);
                }}
                onPaste={(event) => {
                  const text = event.clipboardData?.getData('text') ?? '';
                  const parsed = parseGitHubReleasePaste(text);
                  if (!parsed.repo) return;
                  event.preventDefault();
                  setRepo(parsed.repo);
                  if (parsed.tag) setTag(parsed.tag);
                  setPreview(undefined);
                }}
                placeholder="acme/my-plugin"
                required
              />
            </Field>
            <Field label="Tag" hint="Release tag, e.g. v1.2.3">
              <input
                class="input mono"
                value={tag}
                onInput={(event) => {
                  setTag((event.target as HTMLInputElement).value);
                  setPreview(undefined);
                }}
                placeholder="v1.2.3"
                required
              />
            </Field>
            <div class="row wrap" style="margin-bottom:12px;">
              <button type="button" class="btn ghost" disabled={resolveBusy || !repo.trim() || !tag.trim()} onClick={() => void resolve()}>
                {resolveBusy ? <Spinner /> : 'Resolve'}
              </button>
            </div>
            {preview && (
              <InstallPreviewCard preview={preview} permissionsAck={permissionsAck} onPermissionsAck={setPermissionsAck} />
            )}
          </>
        )}
        {mode === 'gitlab' && (
          <>
            <Field label="Repository" hint="group/project on GitLab">
              <input class="input mono" value={gitlabRepo} onInput={(e) => { setGitlabRepo((e.target as HTMLInputElement).value); setPreview(undefined); }} placeholder="acme/my-plugin" required />
            </Field>
            <Field label="Tag" hint="Release tag, e.g. v1.2.3">
              <input class="input mono" value={gitlabTag} onInput={(e) => { setGitlabTag((e.target as HTMLInputElement).value); setPreview(undefined); }} placeholder="v1.2.3" required />
            </Field>
            <div class="row wrap" style="margin-bottom:12px;">
              <button type="button" class="btn ghost" disabled={resolveBusy || !gitlabRepo.trim() || !gitlabTag.trim()} onClick={() => void resolve()}>
                {resolveBusy ? <Spinner /> : 'Resolve'}
              </button>
            </div>
            {preview && <InstallPreviewCard preview={preview} permissionsAck={permissionsAck} onPermissionsAck={setPermissionsAck} />}
          </>
        )}
        {mode === 'catalog' && (
          <>
            <Field label="Search catalog">
              <input class="input" value={catalogQuery} onInput={(e) => setCatalogQuery((e.target as HTMLInputElement).value)} placeholder="plugin name or id" />
            </Field>
            <div class="plugin-catalog-list">
              {(catalogState.data ?? []).map((entry) => (
                <button
                  type="button"
                  key={entry.plugin_id}
                  class={`plugin-catalog-item ${catalogEntry?.plugin_id === entry.plugin_id ? 'active' : ''}`}
                  onClick={() => { setCatalogEntry(entry); setPreview(undefined); setPermissionsAck(false); }}
                >
                  <div class="row wrap" style="gap:8px; align-items:center;">
                    <strong class="mono">{entry.plugin_id}</strong>
                    {entry.bundled && <Badge kind="info">built-in</Badge>}
                  </div>
                  <span class="dim">{entry.description || entry.name}</span>
                </button>
              ))}
              {!catalogState.loading && (catalogState.data?.length ?? 0) === 0 && (
                <span class="dim">No catalog entries. Add plugins to the host catalog JSON.</span>
              )}
            </div>
            {catalogEntry && catalogBundled && (
              <InlineNotice kind="info">
                {catalogInstalled
                  ? 'Built-in with GoSite — already in the registry (installed, disabled by default). Close this dialog and Enable it from the plugin list.'
                  : 'Built-in with GoSite — seeded on init when bundled artifacts are present. Run gosite init (or make dev-api-setup), then Enable from the plugin list. Remote catalog install is not required.'}
              </InlineNotice>
            )}
            {catalogEntry && !catalogBundled && (
              <div class="row wrap" style="margin:12px 0;">
                <button type="button" class="btn ghost" disabled={resolveBusy} onClick={() => void resolve()}>
                  {resolveBusy ? <Spinner /> : 'Resolve'}
                </button>
              </div>
            )}
            {preview && !catalogBundled && <InstallPreviewCard preview={preview} permissionsAck={permissionsAck} onPermissionsAck={setPermissionsAck} />}
          </>
        )}
        {(mode === 'artifact' || mode === 'manifest') && (
          <Field label="SHA-256" hint="Optional digest check before registry insert.">
            <input
              class="input mono"
              value={sha256}
              onInput={(event) => setSHA256((event.target as HTMLInputElement).value)}
            />
          </Field>
        )}
      </form>
    </Modal>
  );
}

function PluginActions({ plugin, group, reload }: Readonly<{ plugin: PluginVersion; group: PluginGroup; reload: () => void }>) {
  const { meta, toast } = useStore();
  const enable = useAction(plugins.enable);
  const disable = useAction(plugins.disable);
  const switchVersion = useAction(plugins.switchVersion);
  const uninstall = useAction(plugins.uninstall);
  const busy = enable.loading || disable.loading || switchVersion.loading || uninstall.loading;

  const run = async (action: () => Promise<unknown>, message: string) => {
    try {
      await action();
      toast(message);
      reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const targetEnabledElsewhere = group.enabled && group.enabled.version !== plugin.version;
  return (
    <div class="row nowrap plugin-actions">
      {(plugin.state === 'installed' || plugin.state === 'enable_failed') && !targetEnabledElsewhere && (
        <button type="button" class="btn sm primary" disabled={busy} onClick={() => run(() => enable.run(plugin.plugin_id, plugin.version), `${plugin.name} enabled`)}>
          <IconPlay /> Enable
        </button>
      )}
      {(plugin.state === 'installed' || plugin.state === 'enable_failed') && targetEnabledElsewhere && (
        <button type="button" class="btn sm primary" disabled={busy} onClick={() => run(() => switchVersion.run(plugin.plugin_id, plugin.version), `${plugin.name} switched`)}>
          <IconRefresh /> Switch
        </button>
      )}
      {plugin.state === 'enabled' && (
        <button type="button" class="btn sm ghost" disabled={busy} onClick={() => run(() => disable.run(plugin.plugin_id), `${plugin.name} disabled`)}>
          Disable
        </button>
      )}
      {stableForUninstall(plugin.state) && (
        <button
          type="button"
          class="btn sm danger"
          disabled={busy}
          onClick={() => {
            if (!globalThis.confirm(`Uninstall ${plugin.name} ${plugin.version}?`)) return;
            void run(() => uninstall.run(plugin.plugin_id, plugin.version), `${plugin.name} uninstalled`);
          }}
        >
          <IconTrash />
        </button>
      )}
    </div>
  );
}

function PluginRegistry({ rows, reload }: Readonly<{ rows: PluginVersion[]; reload: () => void }>) {
  const groups = useMemo(() => groupPlugins(rows), [rows]);
  if (!groups.length) {
    return (
      <EmptyState
        title="No plugins installed"
        hint="Official built-in plugins ship with GoSite — run init or enable GoSite MCP from the registry after seeding."
      />
    );
  }
  return (
    <div class="plugin-registry">
      {groups.map((group) => (
        <section key={group.id} class="plugin-group">
          <div class="plugin-group-head">
            <div>
              <h2>{group.name}</h2>
              <div class="mono">{group.id}</div>
            </div>
            <div class="row wrap">
              {group.enabled ? <Badge kind="ok">enabled {group.enabled.version}</Badge> : <Badge kind="off">disabled</Badge>}
              {group.latest && isBundledPlugin(group.latest) && <Badge kind="info">built-in</Badge>}
              <Badge kind="info">tier {group.latest?.tier ?? '—'}</Badge>
            </div>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Source</th>
                  <th>State</th>
                  <th>Capabilities</th>
                  <th>UI</th>
                  <th>Failure</th>
                  <th>Updated</th>
                  <th class="right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {group.versions.map((plugin) => (
                  <tr key={`${plugin.plugin_id}:${plugin.version}`}>
                    <td>
                      <div class="mono">{plugin.version}</div>
                      <div class="dim">api {plugin.api_version}</div>
                    </td>
                    <td>
                      <span title={plugin.source_ref ? truncateText(plugin.source_ref, 80) : undefined}>
                        {sourceDisplayLabel(plugin.source_type)}
                      </span>
                      {plugin.source_ref && (
                        <div class="dim mono" style="font-size:11px; max-width:120px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
                          {truncateText(plugin.source_ref, 24)}
                        </div>
                      )}
                    </td>
                    <td><Badge kind={stateKind(plugin.state)}>{stateLabel(plugin.state)}</Badge></td>
                    <td>
                      <div class="chip-row">
                        {capabilityLabels(plugin).map((label) => <span key={label} class="chip">{label}</span>)}
                        {capabilityLabels(plugin).length === 0 && <span class="dim">—</span>}
                      </div>
                    </td>
                    <td>
                      <div class="plugin-sidebar-links">
                        {sidebarEntries(plugin).map((entry) => (
                          <button
                            type="button"
                            class="link-button"
                            key={entry.route}
                            onClick={() => entry.route && navigate(entry.route)}
                          >
                            {entry.label || entry.route}
                          </button>
                        ))}
                        {sidebarEntries(plugin).length === 0 && <span class="dim">—</span>}
                      </div>
                    </td>
                    <td>
                      {plugin.failure_class ? (
                        <div class="plugin-failure">
                          <Badge kind={plugin.failure_class === 'compensation_failed' ? 'danger' : 'warn'}>{plugin.failure_class}</Badge>
                          <span>{plugin.failure_message}</span>
                        </div>
                      ) : <span class="dim">—</span>}
                    </td>
                    <td>{formatRelative(plugin.updated_at)}</td>
                    <td class="right"><PluginActions plugin={plugin} group={group} reload={reload} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ))}
    </div>
  );
}

function PluginDetailPanel({ rows }: Readonly<{ rows: PluginVersion[] }>) {
  const selected = rows.find((item) => item.state === 'enabled') ?? rows.find((item) => item.state !== 'uninstalled');
  if (!selected) return null;
  const hooks = selected.capabilities.hooks ?? [];
  const entries = sidebarEntries(selected);
  const perms = permissions(selected);
  return (
    <aside class="plugin-detail-panel">
      <div class="plugin-detail-card">
        <div class="plugin-emblem"><IconPlug width={22} height={22} /></div>
        <h3>{selected.name}</h3>
        <p class="mono">{selected.plugin_id}</p>
        <div class="divider" />
        <dl class="plugin-facts">
          <div><dt>Runtime</dt><dd>tier {selected.tier}</dd></div>
          <div><dt>RPC</dt><dd>{selected.rpc_version || '—'}</dd></div>
          <div><dt>Config</dt><dd>{selected.config_version || '—'}</dd></div>
          <div><dt>Installed</dt><dd>{formatDate(selected.created_at)}</dd></div>
        </dl>
      </div>
      {hasDistribution(selected) && (
        <div class="plugin-detail-card">
          <h3>{selected.source_type === 'bundled' ? 'Built-in' : 'Distribution'}</h3>
          <dl class="plugin-facts">
            <div><dt>Source type</dt><dd>{sourceDisplayLabel(selected.source_type)}</dd></div>
            <div><dt>Source ref</dt><dd class="mono">{selected.source_ref || '—'}</dd></div>
            <div><dt>Install path</dt><dd>{selected.install_path || '—'}</dd></div>
            {selected.source_commit && (
              <div><dt>Commit</dt><dd class="mono">{truncateText(selected.source_commit, 16)}</dd></div>
            )}
            {selected.resolved_url && (
              <div><dt>Resolved URL</dt><dd class="mono">{truncateText(selected.resolved_url, 48)}</dd></div>
            )}
          </dl>
        </div>
      )}
      {(selected.install_log?.length ?? 0) > 0 && (
        <div class="plugin-detail-card">
          <h3>Install log</h3>
          <ol class="plugin-install-log">
            {selected.install_log!.map((entry) => (
              <li key={`${entry.step}-${entry.at}`} class={entry.status === 'failed' ? 'failed' : 'ok'}>
                <span class="mono">{entry.step}</span>
                {entry.status === 'failed' && entry.failure_class && (
                  <Badge kind="danger">{entry.failure_class}</Badge>
                )}
                {entry.detail && <span class="dim">{entry.detail}</span>}
              </li>
            ))}
          </ol>
        </div>
      )}
      <div class="plugin-detail-card">
        <h3>Hooks</h3>
        <div class="chip-row vertical">
          {hooks.map((hook) => <span key={hook} class="chip mono">{hook}</span>)}
          {hooks.length === 0 && <span class="dim">No hooks declared</span>}
        </div>
      </div>
      <div class="plugin-detail-card">
        <h3>Permissions</h3>
        <div class="chip-row vertical">
          {perms.map((perm) => <span key={perm} class="chip mono">{perm}</span>)}
          {perms.length === 0 && <span class="dim">No permissions declared</span>}
        </div>
      </div>
      <div class="plugin-detail-card">
        <h3>UI routes</h3>
        <div class="chip-row vertical">
          {entries.map((entry) => (
            <button type="button" key={entry.route} class="btn sm ghost" onClick={() => entry.route && navigate(entry.route)}>
              {entry.label || entry.route}
            </button>
          ))}
          {entries.length === 0 && <span class="dim">No sidebar entries</span>}
        </div>
      </div>
    </aside>
  );
}

export function PluginsView({ tab: initialTab = 'registry' }: Readonly<{ tab?: 'registry' | 'keyring' }>) {
  const state = useAsync(() => plugins.list());
  const [installOpen, setInstallOpen] = useState(false);
  const [tab, setTab] = useState<'registry' | 'keyring'>(initialTab);

  useEffect(() => {
    setTab(initialTab);
  }, [initialTab]);

  return (
    <Page
      title="Plugins"
      subtitle={tab === 'keyring'
        ? 'Manage trusted vendor signing keys for strict-mode installs.'
        : 'Install artifacts, validate compatibility, and control enabled versions.'}
      eyebrow="runtime"
      actions={
        <div class="row wrap">
          {tab === 'registry' && (
            <>
              <button type="button" class="btn ghost" onClick={state.reload}><IconRefresh /> Refresh</button>
              <button type="button" class="btn primary" onClick={() => setInstallOpen(true)}><IconArrowUp /> Install</button>
            </>
          )}
          {tab === 'keyring' && (
            <button type="button" class="btn ghost" onClick={() => navigate('/plugins')}><IconPlug /> Registry</button>
          )}
        </div>
      }
    >
      <div class="tabs" style="margin-bottom:16px;">
        <button
          type="button"
          class={tab === 'registry' ? 'active' : ''}
          onClick={() => { setTab('registry'); navigate('/plugins'); }}
        >
          Registry
        </button>
        <button
          type="button"
          class={tab === 'keyring' ? 'active' : ''}
          onClick={() => { setTab('keyring'); navigate('/plugins/keyring'); }}
        >
          Keyring
        </button>
      </div>
      {tab === 'keyring' ? (
        <PluginsKeyringPanel />
      ) : (
        <AsyncView state={state} loadingLabel="Loading plugins">
        {(rows) => {
          const activeRows = rows.filter((item) => item.state !== 'uninstalled');
          const enabled = activeRows.filter((item) => item.state === 'enabled').length;
          const failed = activeRows.filter((item) => item.state.endsWith('_failed')).length;
          const withUI = activeRows.filter((item) => sidebarEntries(item).length > 0).length;
          return (
            <>
              <div class="grid cols-4 plugin-stats">
                <Stat label="Records" value={activeRows.length} sub="installed versions" />
                <Stat label="Enabled" value={enabled} sub="runtime source of truth" tone={enabled ? 'default' : 'info'} />
                <Stat label="Failures" value={failed} sub="retry metadata" tone={failed ? 'danger' : 'default'} />
                <Stat label="UI routes" value={withUI} sub="host-rendered entries" tone="info" />
              </div>
              <div class="plugin-layout">
                <PluginRegistry rows={activeRows} reload={state.reload} />
                <PluginDetailPanel rows={activeRows} />
              </div>
            </>
          );
        }}
      </AsyncView>
      )}
      {installOpen && <InstallModal onClose={() => setInstallOpen(false)} onInstalled={state.reload} />}
    </Page>
  );
}

function parsePluginRoute(path: string) {
  const parts = path.split('/').filter(Boolean);
  if (parts[0] !== 'plugins' || parts.length < 3) return undefined;
  const pluginID = `${parts[1]}/${parts[2]}`;
  const rest = parts.slice(3);
  return { pluginID, route: `/plugins/${pluginID}/${rest.join('/')}`.replace(/\/$/, '') };
}

export function PluginContributionView({ path }: Readonly<{ path: string }>) {
  const parsed = parsePluginRoute(path);
  const state = useAsync(() => plugins.list(), [path]);
  const enable = useAction(plugins.enable);
  const { meta, toast } = useStore();

  if (!parsed) {
    return (
      <Page title="Plugin route" eyebrow="runtime">
        <EmptyState title="Plugin route not found" />
      </Page>
    );
  }

  const doEnable = async (plugin: PluginVersion) => {
    try {
      await enable.run(plugin.plugin_id, plugin.version);
      toast(`${plugin.name} enabled`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Page title="Plugin route" subtitle={parsed.route} eyebrow="runtime">
      <AsyncView state={state} loadingLabel="Loading plugin route">
        {(rows) => {
          const versions = rows.filter((item) => item.plugin_id === parsed.pluginID && item.state !== 'uninstalled');
          const enabled = versions.find((item) => item.state === 'enabled');
          const latest = versions[0];
          if (!latest) {
            return <EmptyState title="Plugin missing" hint="The registry has no installed version for this route." />;
          }
          if (!enabled) {
            return (
              <div class="plugin-route-fallback">
                <IconShield width={28} height={28} />
                <h2>{latest.name} is disabled</h2>
                <p class="dim">Host UI keeps this route safe until a compatible plugin version is enabled.</p>
                {(latest.state === 'installed' || latest.state === 'enable_failed') && (
                  <button type="button" class="btn primary" disabled={enable.loading} onClick={() => doEnable(latest)}>
                    {enable.loading ? <Spinner /> : <><IconPlay /> Enable {latest.version}</>}
                  </button>
                )}
              </div>
            );
          }
          const schema = enabled.ui.configSchema ?? enabled.manifest.ui?.configSchema;
          if (parsed.route.endsWith('/integration')) {
            return <PluginMCPIntegrationView plugin={enabled} />;
          }
          return (
            <div class="plugin-contribution">
              <section class="card">
                <div class="card-head">
                  <h3>{enabled.name}</h3>
                  <Badge kind="ok">enabled {enabled.version}</Badge>
                </div>
                <div class="card-body">
                  <div class="grid cols-3">
                    <Stat label="Plugin" value={enabled.name} sub={`${enabled.plugin_id} · tier ${enabled.tier}`} />
                    <Stat label="Route" value={parsed.route.split('/').pop() || 'index'} sub="host rendered" tone="info" />
                    <Stat label="Updated" value={formatRelative(enabled.updated_at)} sub={formatDate(enabled.updated_at)} />
                  </div>
                </div>
              </section>
              <section class="card">
                <div class="card-head"><h3>Configuration schema</h3></div>
                <div class="card-body">
                  {schema ? (
                    <pre class="logbox plugin-schema">{JSON.stringify(schema, null, 2)}</pre>
                  ) : (
                    <EmptyState title="No configuration schema" hint="This plugin route is registered, but no host-rendered schema is stored." />
                  )}
                </div>
              </section>
            </div>
          );
        }}
      </AsyncView>
    </Page>
  );
}
