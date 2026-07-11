import { useMemo, useState } from 'preact/hooks';
import { plugins } from '../api/endpoints';
import type { IntegrationToken, PluginVersion } from '../api/types';
import { IconCopy, IconShield, IconTrash } from '../components/Icons';
import { Page } from '../components/Layout';
import { AsyncView, Badge, EmptyState, Field, InlineNotice, Modal, Spinner } from '../components/Ui';
import { formatDate, formatRelative } from '../lib/format';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

const READ_SCOPES = ['system:read', 'websites:read', 'nginx:read', 'docker:read', 'cron:read', 'plugins:read'];
const WRITE_SCOPES = ['websites:write'];
const MANAGE_SCOPES = ['nginx:manage', 'docker:manage'];

function manifestCeiling(plugin: PluginVersion): string[] {
  const perms = plugin.manifest?.permissions ?? [];
  return perms.length ? perms : READ_SCOPES;
}

function groupScopes(manifestScopes: string[]) {
  const inManifest = (scope: string) => manifestScopes.includes(scope);
  return {
    read: READ_SCOPES.filter(inManifest),
    write: WRITE_SCOPES.filter(inManifest),
    manage: MANAGE_SCOPES.filter(inManifest),
  };
}

function defaultScopes(manifestScopes: string[]) {
  const groups = groupScopes(manifestScopes);
  return groups.read.length ? groups.read : manifestScopes.slice(0, 1);
}

function mcpJsonSnippet(token: string) {
  const base = {
    mcpServers: {
      gosite: {
        command: 'npx',
        args: ['-y', '@gosite/mcp'],
        env: {
          GOSITE_URL: globalThis.location.origin,
          GOSITE_ACCESS_TOKEN: token,
        },
      },
    },
  };
  return JSON.stringify(base, null, 2);
}

function ScopePicker({
  manifestScopes,
  value,
  onChange,
}: Readonly<{
  manifestScopes: string[];
  value: string[];
  onChange: (scopes: string[]) => void;
}>) {
  const groups = groupScopes(manifestScopes);
  const toggle = (scope: string) => {
    onChange(value.includes(scope) ? value.filter((s) => s !== scope) : [...value, scope]);
  };
  const renderGroup = (title: string, scopes: string[]) => scopes.length > 0 && (
    <div class="integration-scope-group">
      <div class="integration-scope-title">{title}</div>
      <div class="integration-scope-grid">
        {scopes.map((scope) => (
          <label key={scope} class="integration-scope-chip">
            <input
              type="checkbox"
              checked={value.includes(scope)}
              onChange={() => toggle(scope)}
            />
            <span class="mono">{scope}</span>
          </label>
        ))}
      </div>
    </div>
  );
  return (
    <div class="col" style="gap:10px;">
      {renderGroup('Read', groups.read)}
      {renderGroup('Write', groups.write)}
      {renderGroup('Manage', groups.manage)}
    </div>
  );
}

function TokenRow({
  pluginID,
  row,
  manifestScopes,
  onChanged,
}: Readonly<{
  pluginID: string;
  row: IntegrationToken;
  manifestScopes: string[];
  onChanged: () => void;
}>) {
  const { toast } = useStore();
  const update = useAction(plugins.updateIntegrationToken);
  const revoke = useAction(plugins.revokeIntegrationToken);
  const [editing, setEditing] = useState(false);
  const [scopes, setScopes] = useState(row.scopes);
  const revoked = Boolean(row.revoked_at);

  const save = async () => {
    try {
      await update.run(pluginID, row.id, scopes);
      toast('Scopes updated');
      setEditing(false);
      onChanged();
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const doRevoke = async () => {
    if (!globalThis.confirm(`Revoke token "${row.label || row.id}"?`)) return;
    try {
      await revoke.run(pluginID, row.id);
      toast('Token revoked');
      onChanged();
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  return (
    <div class={`integration-token-row${revoked ? ' revoked' : ''}`}>
      <div class="integration-token-head">
        <div>
          <strong>{row.label || 'untitled'}</strong>
          <div class="dim mono" style="font-size:12px;margin-top:2px;">{row.id}</div>
        </div>
        <div class="row wrap">
          {revoked ? <Badge kind="danger">revoked</Badge> : <Badge kind="ok">active</Badge>}
          {!revoked && (
            <>
              <button type="button" class="btn sm ghost" onClick={() => setEditing((v) => !v)}>
                {editing ? 'Cancel' : 'Edit scopes'}
              </button>
              <button type="button" class="btn sm danger ghost" disabled={revoke.loading} onClick={doRevoke}>
                {revoke.loading ? <Spinner /> : <><IconTrash /> Revoke</>}
              </button>
            </>
          )}
        </div>
      </div>
      <div class="integration-token-meta dim">
        <span>Created {formatRelative(row.created_at)}</span>
        {row.last_used_at && <span>Last used {formatRelative(row.last_used_at)}</span>}
        {row.expires_at && <span>Expires {formatDate(row.expires_at)}</span>}
      </div>
      {editing ? (
        <div class="integration-token-edit">
          <ScopePicker manifestScopes={manifestScopes} value={scopes} onChange={setScopes} />
          <button type="button" class="btn sm primary" disabled={update.loading || scopes.length === 0} onClick={save}>
            {update.loading ? <Spinner /> : 'Save scopes'}
          </button>
        </div>
      ) : (
        <div class="integration-token-scopes">
          {row.scopes.map((scope) => <Badge key={scope} kind="info">{scope}</Badge>)}
        </div>
      )}
    </div>
  );
}

export function PluginMCPIntegrationView({ plugin }: Readonly<{ plugin: PluginVersion }>) {
  const { meta, toast } = useStore();
  const manifestScopes = useMemo(() => manifestCeiling(plugin), [plugin]);
  const state = useAsync(() => plugins.listIntegrationTokens(plugin.plugin_id), [plugin.plugin_id]);
  const create = useAction(plugins.createIntegrationToken);

  const [createOpen, setCreateOpen] = useState(false);
  const [label, setLabel] = useState('cursor-laptop');
  const [scopes, setScopes] = useState(() => defaultScopes(manifestScopes));
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  const submitCreate = async () => {
    try {
      const res = await create.run(plugin.plugin_id, { label: label.trim(), scopes });
      if (!res?.secret?.token) return;
      setCreatedSecret(res.secret.token);
      setCreateOpen(false);
      state.reload();
      toast('Integration token created');
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const copySnippet = async () => {
    if (!createdSecret) return;
    const snippet = mcpJsonSnippet(createdSecret);
    try {
      await navigator.clipboard.writeText(snippet);
      toast('mcp.json copied');
    } catch {
      toast('Copy failed — select text manually', 'error');
    }
  };

  return (
    <Page
      title="MCP Integration"
      subtitle={`Scoped access tokens for ${plugin.name}`}
      eyebrow="integration"
      actions={(
        <button type="button" class="btn primary" onClick={() => { setScopes(defaultScopes(manifestScopes)); setCreateOpen(true); }}>
          Generate token
        </button>
      )}
    >
      <InlineNotice kind="info">
        Tokens are shown <strong>once</strong> at creation. MCP clients use <span class="mono">X-Gosite-Access-Token</span> — never your panel password.
      </InlineNotice>

      {createdSecret && (
        <section class="card integration-secret-card">
          <div class="card-head">
            <h3><IconShield /> New token — copy now</h3>
            <button type="button" class="btn sm ghost" onClick={copySnippet}><IconCopy /> Copy mcp.json</button>
          </div>
          <div class="card-body">
            <Field label="Access token">
              <input class="input mono" readOnly value={createdSecret} onFocus={(e) => (e.currentTarget as HTMLInputElement).select()} />
            </Field>
            <pre class="logbox integration-mcp-json">{mcpJsonSnippet(createdSecret)}</pre>
          </div>
        </section>
      )}

      <AsyncView state={state} loadingLabel="Loading tokens">
        {(rows) => rows.length === 0 ? (
          <EmptyState title="No integration tokens" hint="Generate a scoped token to connect Cursor, Claude Desktop, or OpenClaw." />
        ) : (
          <div class="integration-token-list">
            {rows.map((row) => (
              <TokenRow
                key={row.id}
                pluginID={plugin.plugin_id}
                row={row}
                manifestScopes={manifestScopes}
                onChanged={state.reload}
              />
            ))}
          </div>
        )}
      </AsyncView>

      {createOpen && (
        <Modal title="Generate integration token" onClose={() => setCreateOpen(false)}>
          <div class="col" style="gap:14px;">
            <Field label="Label">
              <input class="input" value={label} onInput={(e) => setLabel((e.target as HTMLInputElement).value)} placeholder="cursor-laptop" />
            </Field>
            <div>
              <div class="field-label">Scope whitelist</div>
              <ScopePicker manifestScopes={manifestScopes} value={scopes} onChange={setScopes} />
            </div>
            <div class="row wrap" style="justify-content:flex-end;">
              <button type="button" class="btn ghost" onClick={() => setCreateOpen(false)}>Cancel</button>
              <button type="button" class="btn primary" disabled={create.loading || !label.trim() || scopes.length === 0} onClick={submitCreate}>
                {create.loading ? <Spinner /> : 'Create token'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </Page>
  );
}
