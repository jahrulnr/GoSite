import { useEffect, useState } from 'preact/hooks';
import { ssl, websites } from '../api/endpoints';
import type { Website, WebsiteCreateRequest } from '../api/types';
import {
  IconEdit,
  IconPlay,
  IconPlus,
  IconSettings,
  IconShield,
  IconTrash,
} from '../components/Icons';
import { JobStreamModal } from '../components/JobStream';
import { AsyncView, Badge, EmptyState, ErrorState, Field, InlineNotice, Modal, Spinner } from '../components/Ui';
import { Page, optionLabel } from '../components/Layout';
import { formatDate, displayPath } from '../lib/format';
import { humanizeError, humanizeValidation } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function WebsiteModal({ site, onClose, onSaved }: Readonly<{ site: Website; onClose: () => void; onSaved: () => void }>) {
  const { meta, toast } = useStore();
  const pathHint = meta?.websites?.static_path_hint ?? '';
  const [form, setForm] = useState<WebsiteCreateRequest>({
    name: site.name,
    domain: site.domain,
    path: site.path || (site.id ? '' : pathHint),
    type: site.type,
    upstream: site.upstream,
    active: site.active,
  });
  const [validation, setValidation] = useState<{ text: string; ok: boolean }>();
  const save = useAction(async (body: WebsiteCreateRequest) => {
    if (site.id) await websites.update(site.id, body);
    else await websites.create(body);
  });
  const validate = useAction(websites.validate);

  const setField = (key: keyof WebsiteCreateRequest, value: string | boolean) => setForm((current) => ({ ...current, [key]: value }));
  const onValidate = async () => {
    const res = await validate.run(form.domain, form.path);
    setValidation(humanizeValidation(res?.valid ? 'Valid' : res?.reason ?? res?.message ?? 'Not valid', meta));
  };
  const onSubmit = async (event: Event) => {
    event.preventDefault();
    await save.run(form);
    toast(`${form.domain} saved`);
    onSaved();
  };
  const pathFieldHint = form.type === 'proxy'
    ? meta?.websites?.proxy_upstream_hint
    : pathHint || `Folder inside ${meta?.websites?.web_root ?? 'web root'}`;

  return (
    <Modal
      title={site.id ? `Edit ${site.domain}` : 'New website'}
      onClose={onClose}
      footer={
        <>
          <span class="dim" style="font-size:12px;margin-right:auto;font-family:var(--mono);">Validate checks the path before you save.</span>
          <button type="button" class="btn" onClick={onValidate}>{validate.loading ? <Spinner /> : <><IconShield /> Validate</>}</button>
          <button type="submit" form="website-form" class="btn primary" disabled={save.loading}>{save.loading ? <Spinner /> : 'Save'}</button>
        </>
      }
    >
      <form id="website-form" onSubmit={onSubmit}>
        <div class="grid cols-2">
          <Field label="Name"><input class="input" value={form.name} onInput={(e) => setField('name', (e.target as HTMLInputElement).value)} required /></Field>
          <Field label="Domain"><input class="input" value={form.domain} onInput={(e) => setField('domain', (e.target as HTMLInputElement).value)} required /></Field>
        </div>
        <Field label="Type" hint={meta?.websites?.types?.find((t) => t.value === form.type)?.hint}>
          <select class="select" value={form.type} onChange={(e) => setField('type', (e.target as HTMLSelectElement).value)}>
            {(meta?.websites?.types ?? [{ value: form.type, label: form.type }]).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </Field>
        <Field
          label={form.type === 'proxy' ? 'Upstream' : 'Path'}
          hint={pathFieldHint}
          error={validation && !validation.ok ? validation.text : undefined}
        >
          <input
            class="input mono"
            value={form.type === 'proxy' ? form.upstream ?? '' : form.path}
            placeholder={form.type === 'proxy' ? meta?.websites?.proxy_upstream_hint : pathHint}
            onInput={(e) => setField(form.type === 'proxy' ? 'upstream' : 'path', (e.target as HTMLInputElement).value)}
            required
          />
        </Field>
        <label class="row wrap" style="font-size:13px;color:var(--text-muted);">
          <input type="checkbox" checked={Boolean(form.active)} onChange={(e) => setField('active', (e.target as HTMLInputElement).checked)} />
          <span>Active after save</span>
        </label>
        {validation?.ok && <InlineNotice kind="ok">{validation.text}</InlineNotice>}
        {save.error && <ErrorState error={save.error} message={humanizeError(save.error, meta)} />}
      </form>
    </Modal>
  );
}

function SslModal({ site, onClose }: Readonly<{ site: Website; onClose: () => void }>) {
  const { toast } = useStore();
  const status = useAsync(() => ssl.status(site.id), [site.id]);
  const [pub, setPub] = useState('');
  const [priv, setPriv] = useState('');
  const [streamUrl, setStreamUrl] = useState<string>();
  const manual = useAction(() => ssl.uploadManual(site.id, pub, priv));
  const certbot = useAction(() => ssl.startCertbot(site.id));

  const saveManual = async (event: Event) => {
    event.preventDefault();
    await manual.run();
    toast(`${site.domain} SSL updated`);
    status.reload();
  };

  const runCertbot = async () => {
    const res = await certbot.run();
    if (res?.job_id) {
      setStreamUrl(ssl.certbotStreamUrl(site.id, res.job_id));
    }
    toast(res?.message ?? 'Certbot queued');
    status.reload();
  };

  return (
    <>
      <Modal title={`SSL · ${site.domain}`} onClose={onClose} wide footer={<button type="button" class="btn primary" onClick={runCertbot} disabled={certbot.loading}>{certbot.loading ? <Spinner /> : <IconPlay />} Certbot</button>}>
        <AsyncView state={status}>
          {(data) => (
            <div class="col">
              <div class="grid cols-3">
                <div class="stat info"><div class="label">Enabled</div><div class="value" style="font-size:20px;">{data.enabled ? 'Yes' : 'No'}</div></div>
                <div class={`stat ${data.expired ? 'danger' : 'info'}`}><div class="label">Expired</div><div class="value" style="font-size:20px;">{data.expired ? 'Yes' : 'No'}</div></div>
                <div class="stat info"><div class="label">Expires</div><div class="value" style="font-size:20px;">{formatDate(data.expires_at)}</div></div>
              </div>
              <form onSubmit={saveManual}>
                <Field label="Certificate chain"><textarea class="textarea" value={pub || String(data.public_pem ?? '')} onInput={(e) => setPub((e.target as HTMLTextAreaElement).value)} /></Field>
                <Field label="Private key"><textarea class="textarea" value={priv || String(data.private_pem ?? '')} onInput={(e) => setPriv((e.target as HTMLTextAreaElement).value)} /></Field>
                {manual.error && <ErrorState error={manual.error} />}
                {certbot.error && <ErrorState error={certbot.error} />}
                <button type="submit" class="btn" disabled={manual.loading}>{manual.loading ? <Spinner /> : <IconEdit />} Save manual SSL</button>
              </form>
            </div>
          )}
        </AsyncView>
      </Modal>
      {streamUrl && <JobStreamModal title={`Certbot ${site.domain}`} streamUrl={streamUrl} onClose={() => setStreamUrl(undefined)} />}
    </>
  );
}

function WebsiteConfigModal({ site, onClose }: Readonly<{ site: Website; onClose: () => void }>) {
  const { toast } = useStore();
  const state = useAsync(() => websites.nginxConfig(site.id), [site.id]);
  const [config, setConfig] = useState('');
  const save = useAction(() => websites.updateNginxConfig(site.id, config));

  useEffect(() => {
    if (state.data !== undefined) setConfig(state.data);
  }, [state.data]);

  const submit = async (event: Event) => {
    event.preventDefault();
    await save.run();
    toast(`${site.domain} config saved`);
  };

  return (
    <Modal title={`Nginx config · ${site.domain}`} onClose={onClose} wide footer={<button type="submit" form="website-config" class="btn primary" disabled={save.loading}>{save.loading ? <Spinner /> : 'Save config'}</button>}>
      <AsyncView state={state}>
        {() => (
          <form id="website-config" onSubmit={submit}>
            <textarea class="textarea config-editor" value={config} onInput={(e) => setConfig((e.target as HTMLTextAreaElement).value)} />
            {save.error && <ErrorState error={save.error} />}
          </form>
        )}
      </AsyncView>
    </Modal>
  );
}

export function WebsitesView() {
  const { meta, toast } = useStore();
  const state = useAsync(() => websites.list());
  const [editing, setEditing] = useState<Website | null>(null);
  const [sslSite, setSslSite] = useState<Website | null>(null);
  const [configSite, setConfigSite] = useState<Website | null>(null);
  const toggle = useAction(websites.toggle);
  const remove = useAction(websites.remove);

  const onToggle = async (site: Website) => {
    await toggle.run(site.id);
    toast(`${site.domain} updated`);
    state.reload();
  };

  const onRemove = async (site: Website) => {
    if (!globalThis.confirm(`Delete ${site.domain}?`)) return;
    await remove.run(site.id, false);
    toast(`${site.domain} deleted`);
    state.reload();
  };

  return (
    <Page
      title="Websites"
      subtitle={`Sites are stored under ${meta?.websites?.web_root ?? 'the web folder'}`}
      eyebrow="operate"
      actions={<button type="button" class="btn primary" onClick={() => setEditing({ id: 0, name: '', domain: '', path: '', type: meta?.websites?.types?.[0]?.value ?? 'static', ssl: false, active: false })}><IconPlus /> New</button>}
    >
      <AsyncView state={state} isEmpty={(rows) => rows.length === 0} empty={<EmptyState title="No websites yet" hint="Add your first site to get started." />}>
        {(rows) => (
          <div class="card">
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr><th>Domain</th><th>Type</th><th>Path / upstream</th><th>Status</th><th>SSL</th><th class="right">Actions</th></tr>
                </thead>
                <tbody>
                  {rows.map((site) => (
                    <tr key={site.id}>
                      <td>
                        <strong>{site.name}</strong>
                        <div class="mono dim" style="font-size:11px;">{site.domain}</div>
                      </td>
                      <td><span class="badge info" style="font-size:10px;padding:2px 7px;">{optionLabel(meta?.websites?.types, site.type)}</span></td>
                      <td class="mono truncate" title={site.type === 'proxy' ? site.upstream : site.path} style="max-width:240px;">
                        {site.type === 'proxy' ? site.upstream : displayPath(meta?.websites?.web_root, site.path)}
                      </td>
                      <td><Badge kind={site.active ? 'ok' : 'off'}>{site.active ? 'active' : 'disabled'}</Badge></td>
                      <td><Badge kind={site.ssl ? 'ok' : 'off'}>{site.ssl ? 'enabled' : 'off'}</Badge></td>
                      <td class="right nowrap">
                        <button type="button" class="btn sm ghost" onClick={() => onToggle(site)} title="Toggle active"><IconPlay /></button>
                        <button type="button" class="btn sm ghost" onClick={() => setSslSite(site)} title="SSL"><IconShield /></button>
                        <button type="button" class="btn sm ghost" onClick={() => setConfigSite(site)} title="Config"><IconSettings /></button>
                        <button type="button" class="btn sm ghost" onClick={() => setEditing(site)} title="Edit"><IconEdit /></button>
                        <button type="button" class="btn sm danger" onClick={() => onRemove(site)} title="Delete"><IconTrash /></button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AsyncView>
      {editing && <WebsiteModal site={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); state.reload(); }} />}
      {sslSite && <SslModal site={sslSite} onClose={() => setSslSite(null)} />}
      {configSite && <WebsiteConfigModal site={configSite} onClose={() => setConfigSite(null)} />}
    </Page>
  );
}
