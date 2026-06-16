import { useMemo, useState } from 'preact/hooks';
import { mounts } from '../api/endpoints';
import type { Mount, MountS3Config } from '../api/types';
import { IconEdit, IconPlay, IconPlus, IconTrash } from '../components/Icons';
import { AsyncView, Badge, EmptyState, ErrorState, Field, Modal, Spinner } from '../components/Ui';
import { Page } from '../components/Layout';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function isS3Type(type: string) {
  const value = type.toLowerCase();
  return value === 'fuse.s3fs' || value === 's3' || value === 's3fs';
}

function emptyS3(): MountS3Config {
  return { bucket: '', endpoint: '', region: '', access_key: '', secret_key: '', path_style: false };
}

function deviceLabel(row: Mount) {
  if (isS3Type(row.type)) return `s3://${row.s3?.bucket ?? row.device}`;
  return row.device;
}

function MountModal({
  mount,
  original,
  onClose,
  onSaved,
}: Readonly<{ mount: Mount; original?: Mount; onClose: () => void; onSaved: () => void }>) {
  const { meta, toast } = useStore();
  const [form, setForm] = useState(mount);
  const s3Enabled = isS3Type(form.type);
  const selectedType = useMemo(
    () => (meta?.mounts?.fs_types ?? []).find((item) => item.value === form.type),
    [meta?.mounts?.fs_types, form.type],
  );

  const setS3 = (patch: Partial<MountS3Config>) => {
    setForm({ ...form, s3: { ...(form.s3 ?? emptyS3()), ...patch } });
  };

  const onTypeChange = (type: string) => {
    if (isS3Type(type)) {
      setForm({
        ...form,
        type,
        device: form.s3?.bucket ?? form.device,
        options: '_netdev,allow_other',
        dump: '0',
        fsck: '0',
        s3: form.s3 ?? emptyS3(),
      });
      return;
    }
    setForm({ ...form, type, s3: undefined });
  };

  const save = useAction((entry: Mount) =>
    original ? mounts.update(original.device, original.dir, entry) : mounts.create(entry),
  );

  const submit = async (event: Event) => {
    event.preventDefault();
    const payload: Mount = { ...form };
    if (s3Enabled) {
      payload.s3 = { ...(form.s3 ?? emptyS3()) };
      payload.device = payload.s3.bucket ?? '';
      payload.dump = '0';
      payload.fsck = '0';
    } else {
      delete payload.s3;
    }
    try {
      await save.run(payload);
      toast(`${form.dir} saved`);
      onSaved();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Modal title={original ? `Edit ${original.dir}` : 'New mount'} onClose={onClose} footer={<button type="submit" form="mount-form" class="btn primary" disabled={save.loading}>{save.loading ? <Spinner /> : 'Save'}</button>}>
      <form id="mount-form" onSubmit={submit}>
        <Field label="Filesystem" hint={selectedType?.hint}>
          <select class="select" value={form.type} onChange={(e) => onTypeChange((e.target as HTMLSelectElement).value)}>
            {(meta?.mounts?.fs_types ?? [{ value: form.type, label: form.type }]).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </Field>

        {s3Enabled ? (
          <>
            <Field label="Bucket" hint="S3 bucket name, e.g. my-site-assets">
              <input class="input mono" value={form.s3?.bucket ?? ''} onInput={(e) => setS3({ bucket: (e.target as HTMLInputElement).value })} required />
            </Field>
            <Field label="Mount directory" hint="Where the bucket appears on the server, e.g. /storage/mnt/s3-backup">
              <input class="input mono" value={form.dir} onInput={(e) => setForm({ ...form, dir: (e.target as HTMLInputElement).value })} required />
            </Field>
            <Field label="Endpoint URL" hint="Leave empty for AWS. MinIO example: https://minio.example.com">
              <input class="input mono" value={form.s3?.endpoint ?? ''} onInput={(e) => setS3({ endpoint: (e.target as HTMLInputElement).value })} placeholder="https://s3.amazonaws.com" />
            </Field>
            <Field label="Region" hint="AWS region code, e.g. ap-southeast-1">
              <input class="input mono" value={form.s3?.region ?? ''} onInput={(e) => setS3({ region: (e.target as HTMLInputElement).value })} placeholder="ap-southeast-1" />
            </Field>
            <div class="grid cols-2">
              <Field label="Access key" hint={original ? 'Leave blank to keep existing key' : 'Required for new mount'}>
                <input class="input mono" type="password" autoComplete="off" value={form.s3?.access_key ?? ''} onInput={(e) => setS3({ access_key: (e.target as HTMLInputElement).value })} required={!original} />
              </Field>
              <Field label="Secret key" hint={original ? 'Leave blank to keep existing secret' : 'Required for new mount'}>
                <input class="input mono" type="password" autoComplete="off" value={form.s3?.secret_key ?? ''} onInput={(e) => setS3({ secret_key: (e.target as HTMLInputElement).value })} required={!original} />
              </Field>
            </div>
            <label class="checkbox-row">
              <input type="checkbox" checked={!!form.s3?.path_style} onChange={(e) => setS3({ path_style: (e.target as HTMLInputElement).checked })} />
              <span>Use path-style requests (required for most S3-compatible providers)</span>
            </label>
          </>
        ) : (
          <>
            <Field label="Device" hint="NFS: server:/export · Block: /dev/sdb1 · CIFS: //server/share">
              <input class="input mono" value={form.device} onInput={(e) => setForm({ ...form, device: (e.target as HTMLInputElement).value })} required />
            </Field>
            <Field label="Mount directory" hint="Where this storage appears on the server, e.g. /mnt/backup">
              <input class="input mono" value={form.dir} onInput={(e) => setForm({ ...form, dir: (e.target as HTMLInputElement).value })} required />
            </Field>
            <Field label="Options" hint="Comma separated fstab options">
              <input class="input mono" value={form.options} onInput={(e) => setForm({ ...form, options: (e.target as HTMLInputElement).value })} required />
            </Field>
            <div class="grid cols-2">
              <Field label="Backup flag" hint="Usually 0"><input class="input mono" value={form.dump} onInput={(e) => setForm({ ...form, dump: (e.target as HTMLInputElement).value })} /></Field>
              <Field label="Filesystem check order" hint="Usually 0"><input class="input mono" value={form.fsck} onInput={(e) => setForm({ ...form, fsck: (e.target as HTMLInputElement).value })} /></Field>
            </div>
          </>
        )}
        {save.error && <ErrorState error={save.error} message={humanizeError(save.error, meta)} />}
      </form>
    </Modal>
  );
}

export function MountsView() {
  const { meta, toast } = useStore();
  const state = useAsync(() => mounts.list());
  const remove = useAction(mounts.remove);
  const enable = useAction(mounts.enable);
  const [editing, setEditing] = useState<Mount | null>(null);

  const newMount = (): Mount => ({
    device: '',
    dir: '',
    type: meta?.mounts?.fs_types?.[0]?.value ?? 'nfs',
    options: meta?.mounts?.default_options ?? 'defaults',
    dump: meta?.mounts?.dump_default ?? '0',
    fsck: meta?.mounts?.fsck_default ?? '0',
    mounted: false,
  });

  const doEnable = async (row: Mount) => {
    try {
      await enable.run(row.device, row.dir);
      toast(`${row.dir} enabled`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const doRemove = async (row: Mount) => {
    if (!globalThis.confirm(`Delete mount ${row.dir}?`)) return;
    try {
      await remove.run(row.device, row.dir);
      toast(`${row.dir} deleted`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const openEdit = (row: Mount) => {
    if (isS3Type(row.type)) {
      setEditing({
        ...row,
        s3: {
          bucket: row.s3?.bucket ?? row.device,
          endpoint: row.s3?.endpoint ?? '',
          region: row.s3?.region ?? '',
          access_key: '',
          secret_key: '',
          path_style: row.s3?.path_style ?? false,
        },
      });
      return;
    }
    setEditing(row);
  };

  return (
    <Page
      title="Mounts"
      subtitle={meta?.mounts?.example}
      eyebrow="runtime"
      actions={<button type="button" class="btn primary" onClick={() => setEditing(newMount())}><IconPlus /> New</button>}
    >
      <AsyncView state={state} isEmpty={(rows) => rows.length === 0} empty={<EmptyState title="No mounts" hint="Add a mount entry to manage fstab from the panel." />}>
        {(rows: Mount[]) => (
          <div class="card">
            <div class="table-wrap">
              <table class="table">
                <thead><tr><th>Device</th><th>Directory</th><th>Type</th><th>Options</th><th>Status</th><th class="right">Actions</th></tr></thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={`${row.device}-${row.dir}`}>
                      <td class="mono">{deviceLabel(row)}</td>
                      <td class="mono">{row.dir}</td>
                      <td><span class="badge info" style="font-size:10px;padding:2px 7px;">{isS3Type(row.type) ? 's3' : row.type}</span></td>
                      <td class="mono truncate" style="max-width:200px;">{row.options}</td>
                      <td><Badge kind={row.mounted ? 'ok' : 'off'}>{row.mounted ? 'mounted' : 'off'}</Badge></td>
                      <td class="right nowrap">
                        <button type="button" class="btn sm ghost" onClick={() => doEnable(row)}><IconPlay /> Enable</button>
                        <button type="button" class="btn sm ghost" onClick={() => openEdit(row)}><IconEdit /></button>
                        <button type="button" class="btn sm danger" onClick={() => doRemove(row)}><IconTrash /></button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AsyncView>
      {editing && <MountModal mount={editing} original={state.data?.find((row) => row.device === editing.device && row.dir === editing.dir)} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); state.reload(); }} />}
    </Page>
  );
}
