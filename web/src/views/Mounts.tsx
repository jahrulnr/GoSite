import { useState } from 'preact/hooks';
import { mounts } from '../api/endpoints';
import type { Mount } from '../api/types';
import { IconEdit, IconPlay, IconPlus, IconTrash } from '../components/Icons';
import { AsyncView, Badge, EmptyState, ErrorState, Field, Modal, Spinner } from '../components/Ui';
import { Page } from '../components/Layout';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function MountModal({
  mount,
  original,
  onClose,
  onSaved,
}: Readonly<{ mount: Mount; original?: Mount; onClose: () => void; onSaved: () => void }>) {
  const { meta, toast } = useStore();
  const [form, setForm] = useState(mount);
  const save = useAction((entry: Mount) =>
    original ? mounts.update(original.device, original.dir, entry) : mounts.create(entry),
  );

  const submit = async (event: Event) => {
    event.preventDefault();
    await save.run(form);
    toast(`${form.dir} saved`);
    onSaved();
  };

  return (
    <Modal title={original ? `Edit ${original.dir}` : 'New mount'} onClose={onClose} footer={<button type="submit" form="mount-form" class="btn primary" disabled={save.loading}>{save.loading ? <Spinner /> : 'Save'}</button>}>
      <form id="mount-form" onSubmit={submit}>
        <Field label="Device" hint="NFS: server:/export · Block: /dev/sdb1"><input class="input mono" value={form.device} onInput={(e) => setForm({ ...form, device: (e.target as HTMLInputElement).value })} required /></Field>
        <Field label="Mount directory" hint="Where this storage appears on the server, e.g. /mnt/backup"><input class="input mono" value={form.dir} onInput={(e) => setForm({ ...form, dir: (e.target as HTMLInputElement).value })} required /></Field>
        <Field label="Filesystem">
          <select class="select" value={form.type} onChange={(e) => setForm({ ...form, type: (e.target as HTMLSelectElement).value })}>
            {(meta?.mounts?.fs_types ?? [{ value: form.type, label: form.type }]).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </Field>
        <Field label="Options" hint="Comma separated fstab options"><input class="input mono" value={form.options} onInput={(e) => setForm({ ...form, options: (e.target as HTMLInputElement).value })} required /></Field>
        <div class="grid cols-2">
          <Field label="Backup flag" hint="Usually 0"><input class="input mono" value={form.dump} onInput={(e) => setForm({ ...form, dump: (e.target as HTMLInputElement).value })} /></Field>
          <Field label="Filesystem check order" hint="Usually 0"><input class="input mono" value={form.fsck} onInput={(e) => setForm({ ...form, fsck: (e.target as HTMLInputElement).value })} /></Field>
        </div>
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

  const doEnable = async (row: Mount) => {
    await enable.run(row.device, row.dir);
    toast(`${row.dir} enabled`);
    state.reload();
  };

  const doRemove = async (row: Mount) => {
    if (!globalThis.confirm(`Delete mount ${row.dir}?`)) return;
    await remove.run(row.device, row.dir);
    toast(`${row.dir} deleted`);
    state.reload();
  };

  return (
    <Page
      title="Mounts"
      subtitle={meta?.mounts?.example}
      eyebrow="runtime"
      actions={<button type="button" class="btn primary" onClick={() => setEditing({ device: '', dir: '', type: meta?.mounts?.fs_types?.[0]?.value ?? 'nfs', options: meta?.mounts?.default_options ?? 'defaults', dump: meta?.mounts?.dump_default ?? '0', fsck: meta?.mounts?.fsck_default ?? '0', mounted: false })}><IconPlus /> New</button>}
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
                      <td class="mono">{row.device}</td>
                      <td class="mono">{row.dir}</td>
                      <td><span class="badge info" style="font-size:10px;padding:2px 7px;">{row.type}</span></td>
                      <td class="mono truncate" style="max-width:200px;">{row.options}</td>
                      <td><Badge kind={row.mounted ? 'ok' : 'off'}>{row.mounted ? 'mounted' : 'off'}</Badge></td>
                      <td class="right nowrap">
                        <button type="button" class="btn sm ghost" onClick={() => doEnable(row)}><IconPlay /> Enable</button>
                        <button type="button" class="btn sm ghost" onClick={() => setEditing(row)}><IconEdit /></button>
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
