import { useState } from 'preact/hooks';
import { cron } from '../api/endpoints';
import type { Cronjob } from '../api/types';
import { JobStreamModal } from '../components/JobStream';
import { IconEdit, IconPlay, IconPlus, IconTrash } from '../components/Icons';
import { AsyncView, EmptyState, ErrorState, Field, Modal, Spinner } from '../components/Ui';
import { Page, optionLabel } from '../components/Layout';
import { formatDate } from '../lib/format';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function CronModal({ job, onClose, onSaved }: Readonly<{ job: Cronjob; onClose: () => void; onSaved: () => void }>) {
  const { meta, toast } = useStore();
  const [form, setForm] = useState({ name: job.name, payload: job.payload, run_every: job.run_every });
  const save = useAction((payload: { name: string; payload: string; run_every: string }) =>
    job.id ? cron.update(job.id, payload) : cron.create(payload),
  );

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      await save.run(form);
      toast(`${form.name} saved`);
      onSaved();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Modal title={job.id ? `Edit ${job.name}` : 'New cronjob'} onClose={onClose} footer={<button type="submit" form="cron-form" class="btn primary" disabled={save.loading}>{save.loading ? <Spinner /> : 'Save'}</button>}>
      <form id="cron-form" onSubmit={submit}>
        <Field label="Name"><input class="input" value={form.name} onInput={(e) => setForm({ ...form, name: (e.target as HTMLInputElement).value })} required /></Field>
        <Field label="Schedule" hint={meta?.cron?.run_every_options?.find((it) => it.value === form.run_every)?.hint}>
          <select class="select" value={form.run_every} onChange={(e) => setForm({ ...form, run_every: (e.target as HTMLSelectElement).value })}>
            {(meta?.cron?.run_every_options ?? [{ value: form.run_every, label: form.run_every }]).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </Field>
        <Field label="Payload"><textarea class="textarea" value={form.payload} onInput={(e) => setForm({ ...form, payload: (e.target as HTMLTextAreaElement).value })} required /></Field>
        {save.error && <ErrorState error={save.error} />}
      </form>
    </Modal>
  );
}

export function CronView() {
  const { meta, toast } = useStore();
  const state = useAsync(() => cron.list());
  const runJob = useAction(cron.run);
  const remove = useAction(cron.remove);
  const [editing, setEditing] = useState<Cronjob | null>(null);
  const [stream, setStream] = useState<{ id: number; name: string; url: string }>();

  const doRun = async (job: Cronjob) => {
    try {
      const res = await runJob.run(job.id);
      if (res?.job_id) {
        setStream({ id: job.id, name: job.name, url: cron.runStreamUrl(job.id, res.job_id) });
      }
      toast(`${job.name} queued`);
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  const doRemove = async (job: Cronjob) => {
    if (!globalThis.confirm(`Delete ${job.name}?`)) return;
    try {
      await remove.run(job.id);
      toast(`${job.name} deleted`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error, meta), 'error');
    }
  };

  return (
    <Page
      title="Cron"
      subtitle="Jobs from backend cron APIs"
      eyebrow="runtime"
      actions={<button type="button" class="btn primary" onClick={() => setEditing({ id: 0, name: '', payload: '', run_every: meta?.cron?.run_every_options?.[0]?.value ?? 'hour' })}><IconPlus /> New</button>}
    >
      <AsyncView state={state} isEmpty={(rows) => rows.length === 0} empty={<EmptyState title="No cronjobs" />}>
        {(rows) => (
          <div class="card">
            <div class="table-wrap">
              <table class="table">
                <thead><tr><th>Name</th><th>Schedule</th><th>Payload</th><th>Last run</th><th class="right">Actions</th></tr></thead>
                <tbody>
                  {rows.map((job) => (
                    <tr key={job.id}>
                      <td>{job.name}</td>
                      <td><span class="badge info">{optionLabel(meta?.cron?.run_every_options, job.run_every)}</span></td>
                      <td class="mono truncate" style="max-width:300px;">{job.payload}</td>
                      <td>{formatDate(job.executed_at)}</td>
                      <td class="right nowrap">
                        {meta?.cron?.manual_run?.enabled && <button type="button" class="btn sm ghost" onClick={() => doRun(job)}><IconPlay /> Run</button>}
                        <button type="button" class="btn sm ghost" onClick={() => setEditing(job)}><IconEdit /></button>
                        <button type="button" class="btn sm danger" onClick={() => doRemove(job)}><IconTrash /></button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AsyncView>
      {editing && <CronModal job={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); state.reload(); }} />}
      {stream && <JobStreamModal title={`Run ${stream.name}`} streamUrl={stream.url} onClose={() => setStream(undefined)} />}
    </Page>
  );
}
