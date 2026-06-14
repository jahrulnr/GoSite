import { useState } from 'preact/hooks';
import { docker } from '../api/endpoints';
import type { DockerContainer } from '../api/types';
import { IconList, IconRefresh, IconTrash } from '../components/Icons';
import { AsyncView, Badge, EmptyState, Modal } from '../components/Ui';
import { Page } from '../components/Layout';
import { useAction, useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function statusTone(status: string): 'ok' | 'warn' | 'off' | 'danger' {
  const s = status.toLowerCase();
  if (s.includes('up') && !s.includes('unhealthy') && !s.includes('restarting')) return 'ok';
  if (s.includes('unhealthy') || s.includes('dead') || s.includes('exited')) return 'danger';
  if (s.includes('restarting') || s.includes('paused') || s.includes('starting')) return 'warn';
  return 'off';
}

export function DockerView() {
  const { meta, toast } = useStore();
  const state = useAsync(() => docker.list());
  const restart = useAction(docker.restart);
  const stop = useAction(docker.stop);
  const [logId, setLogId] = useState<string>();
  const logState = useAsync(() => (logId ? docker.logs(logId) : Promise.resolve([])), [logId]);

  const doRestart = async (row: DockerContainer) => {
    await restart.run(row.id);
    toast(`${row.name} restarted`);
    state.reload();
  };

  const doStop = async (row: DockerContainer) => {
    await stop.run(row.id);
    toast(`${row.name} stopped`);
    state.reload();
  };

  return (
    <Page title="Docker" subtitle="Containers visible to the panel" eyebrow="runtime">
      <AsyncView state={state} isEmpty={(rows) => rows.length === 0} empty={<EmptyState title="No containers" hint="No containers visible to the panel. Check Docker socket access on the host." />}>
        {(rows) => (
          <div class="card">
            <div class="table-wrap">
              <table class="table">
                <thead><tr><th>Name</th><th>Image</th><th>Status</th><th class="right">Actions</th></tr></thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={row.id}>
                      <td>
                        <div>{row.name}</div>
                        <div class="mono dim" style="font-size:11px;">{row.id.slice(0, 12)}</div>
                      </td>
                      <td class="mono">{row.image}</td>
                      <td><Badge kind={statusTone(row.status)}>{row.status}</Badge></td>
                      <td class="right nowrap">
                        {meta?.docker?.logs?.enabled && <button type="button" class="btn sm ghost" onClick={() => setLogId(row.id)}><IconList /> Logs</button>}
                        {meta?.docker?.restart?.enabled && <button type="button" class="btn sm ghost" onClick={() => doRestart(row)}><IconRefresh /> Restart</button>}
                        {meta?.docker?.stop?.enabled && <button type="button" class="btn sm danger" onClick={() => doStop(row)}><IconTrash /> Stop</button>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AsyncView>
      {logId && <Modal title={`Container logs · ${logId.slice(0, 12)}`} onClose={() => setLogId(undefined)} wide><AsyncView state={logState}>{(lines) => <pre class="logbox">{lines.join('\n')}</pre>}</AsyncView></Modal>}
    </Page>
  );
}
