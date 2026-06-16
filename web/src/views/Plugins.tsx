import { useMemo, useState } from 'preact/hooks';
import { plugins } from '../api/endpoints';
import type { PluginState, PluginVersion } from '../api/types';
import { IconArrowUp, IconPlay, IconPlug, IconRefresh, IconShield, IconTrash } from '../components/Icons';
import { Page, Stat } from '../components/Layout';
import { AsyncView, Badge, EmptyState, Field, InlineNotice, Modal, Spinner } from '../components/Ui';
import { formatDate, formatRelative } from '../lib/format';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { navigate } from '../lib/router';
import { useStore } from '../lib/store';

type InstallMode = 'artifact' | 'manifest';

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

function InstallModal({ onClose, onInstalled }: Readonly<{ onClose: () => void; onInstalled: () => void }>) {
  const { meta, toast } = useStore();
  const [mode, setMode] = useState<InstallMode>('artifact');
  const [file, setFile] = useState<File>();
  const [sha256, setSHA256] = useState('');
  const [manifest, setManifest] = useState('');
  const installFile = useAction(plugins.installFile);
  const installManifest = useAction(plugins.installManifest);
  const busy = installFile.loading || installManifest.loading;
  const error = installFile.error ?? installManifest.error;

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      if (mode === 'artifact') {
        if (!file) throw new Error('Choose an artifact first');
        await installFile.run(file, sha256.trim() || undefined);
      } else {
        JSON.parse(manifest);
        await installManifest.run(manifest, sha256.trim() || undefined);
      }
      toast('Plugin installed');
      onInstalled();
      onClose();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Modal
      title="Install plugin"
      onClose={onClose}
      wide
      footer={
        <>
          {error && <InlineNotice kind="danger">{humanizeError(error, meta)}</InlineNotice>}
          <button type="button" class="btn ghost" onClick={onClose}>Cancel</button>
          <button type="submit" form="plugin-install-form" class="btn primary" disabled={busy}>
            {busy ? <Spinner /> : <><IconArrowUp /> Install</>}
          </button>
        </>
      }
    >
      <form id="plugin-install-form" onSubmit={submit}>
        <div class="tabs">
          <button type="button" class={mode === 'artifact' ? 'active' : ''} onClick={() => setMode('artifact')}>Artifact</button>
          <button type="button" class={mode === 'manifest' ? 'active' : ''} onClick={() => setMode('manifest')}>Manifest JSON</button>
        </div>
        {mode === 'artifact' ? (
          <Field label="Artifact">
            <input
              class="input"
              type="file"
              onChange={(event) => setFile((event.target as HTMLInputElement).files?.[0])}
              required
            />
          </Field>
        ) : (
          <Field label="Manifest JSON">
            <textarea
              class="textarea plugin-manifest-input"
              value={manifest}
              onInput={(event) => setManifest((event.target as HTMLTextAreaElement).value)}
              required
            />
          </Field>
        )}
        <Field label="SHA-256" hint="Optional digest check before registry insert.">
          <input
            class="input mono"
            value={sha256}
            onInput={(event) => setSHA256((event.target as HTMLInputElement).value)}
          />
        </Field>
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
    return <EmptyState title="No plugins installed" hint="Install an artifact or manifest to create the first registry record." />;
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
              <Badge kind="info">tier {group.latest?.tier ?? '—'}</Badge>
            </div>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Version</th>
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

export function PluginsView() {
  const state = useAsync(() => plugins.list());
  const [installOpen, setInstallOpen] = useState(false);

  return (
    <Page
      title="Plugins"
      subtitle="Install artifacts, validate compatibility, and control enabled versions."
      eyebrow="runtime"
      actions={
        <div class="row wrap">
          <button type="button" class="btn ghost" onClick={state.reload}><IconRefresh /> Refresh</button>
          <button type="button" class="btn primary" onClick={() => setInstallOpen(true)}><IconArrowUp /> Install</button>
        </div>
      }
    >
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
