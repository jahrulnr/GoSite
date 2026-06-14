import { useEffect, useState } from 'preact/hooks';
import { files } from '../api/endpoints';
import type { FileEntry } from '../api/types';
import { IconEdit, IconFile, IconFolder, IconPlus, IconSettings, IconTrash } from '../components/Icons';
import { AsyncView, EmptyState, ErrorState, Field, Modal, Spinner } from '../components/Ui';
import { Page } from '../components/Layout';
import { formatBytes, formatDate } from '../lib/format';
import { useAction, useAsync } from '../lib/hooks';
import { navigate } from '../lib/router';
import { useStore } from '../lib/store';

function FileActionModal({
  action,
  dir,
  entry,
  onClose,
  onDone,
}: Readonly<{
  action: 'file' | 'folder' | 'upload' | 'read' | 'copy' | 'chmod';
  dir: string;
  entry: FileEntry | null;
  onClose: () => void;
  onDone: () => void;
}>) {
  const { toast } = useStore();
  const [name, setName] = useState('');
  const [content, setContent] = useState('');
  const [toPath, setToPath] = useState('');
  const [mode, setMode] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [readContent, setReadContent] = useState<string>();
  const run = useAction(async () => {
    if (action === 'file') return files.createFile(dir, name, content);
    if (action === 'folder') return files.createFolder(dir, name);
    if (action === 'upload' && file) return files.upload(dir, file);
    if (action === 'copy' && entry) return files.action('copy', entry.path, { to_path: toPath });
    if (action === 'chmod' && entry) return files.action('chmod', entry.path, { mode });
    return Promise.resolve({ message: 'ok' });
  });
  const read = useAsync(() => (action === 'read' && entry ? files.read(entry.path) : Promise.resolve('')), [action, entry?.path]);

  useEffect(() => {
    if (read.data !== undefined) setReadContent(read.data);
  }, [read.data]);

  const submit = async (event: Event) => {
    event.preventDefault();
    await run.run();
    toast('File action completed');
    onDone();
  };

  if (action === 'read') {
    return (
      <Modal title={entry?.path ?? 'File'} onClose={onClose} wide>
        <AsyncView state={read}>{() => <textarea class="textarea" readOnly value={readContent ?? ''} />}</AsyncView>
      </Modal>
    );
  }

  return (
    <Modal title={action === 'file' ? 'Create file' : action === 'folder' ? 'Create folder' : action === 'upload' ? 'Upload file' : action === 'copy' ? 'Copy path' : 'Change mode'} onClose={onClose} footer={<button type="submit" form="file-action" class="btn primary" disabled={run.loading}>{run.loading ? <Spinner /> : 'Apply'}</button>}>
      <form id="file-action" onSubmit={submit}>
        {(action === 'file' || action === 'folder') && <Field label="Name"><input class="input" value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} required /></Field>}
        {action === 'file' && <Field label="Content"><textarea class="textarea" value={content} onInput={(e) => setContent((e.target as HTMLTextAreaElement).value)} /></Field>}
        {action === 'upload' && <Field label="File"><input class="input" type="file" onChange={(e) => setFile((e.target as HTMLInputElement).files?.[0] ?? null)} required /></Field>}
        {action === 'copy' && <Field label="Destination path" hint={entry?.path}><input class="input mono" value={toPath} onInput={(e) => setToPath((e.target as HTMLInputElement).value)} required /></Field>}
        {action === 'chmod' && <Field label="Mode" hint={entry?.mode}><input class="input mono" value={mode} onInput={(e) => setMode((e.target as HTMLInputElement).value)} placeholder="0644" required /></Field>}
        {run.error && <ErrorState error={run.error} />}
      </form>
    </Modal>
  );
}

function Crumbs({ path, roots }: Readonly<{ path: string; roots: Array<{ path: string; label?: string }> }>) {
  const current = roots.find((r) => r.path === path) ?? { path, label: path };
  return (
    <div class="crumbs">
      {roots.map((root) => (
        <a key={root.path} onClick={() => navigate('/files', { path: root.path })}>{root.label ?? 'Root'}</a>
      ))}
      {current.path !== path && (
        <>
          <span class="sep">/</span>
          <span>{path.split('/').pop()}</span>
        </>
      )}
    </div>
  );
}

export function FilesView({ path }: Readonly<{ path: string }>) {
  const { meta, toast } = useStore();
  const roots = meta?.files?.roots ?? [];
  const rootPath = roots[0]?.path ?? '';
  const currentPath = path || rootPath;
  const currentLabel = roots.find((r) => r.path === currentPath)?.label ?? currentPath;
  const state = useAsync(
    () => (currentPath ? files.browse(currentPath) : Promise.resolve([])),
    [currentPath],
  );
  const [action, setAction] = useState<'file' | 'folder' | 'upload' | 'read' | 'copy' | 'chmod' | null>(null);
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const remove = useAction(files.remove);

  useEffect(() => {
    if (!path && rootPath) navigate('/files', { path: rootPath });
  }, [path, rootPath]);

  const open = (entry: FileEntry) => {
    if (entry.is_dir) navigate('/files', { path: entry.path });
    else {
      setSelected(entry);
      setAction('read');
    }
  };

  const deleteEntry = async (entry: FileEntry) => {
    if (!globalThis.confirm(`Delete ${entry.path}?`)) return;
    await remove.run(entry.path);
    toast(`${entry.name} deleted`);
    state.reload();
  };

  if (!meta) {
    return (
      <Page title="Files" subtitle="Loading folders…" eyebrow="storage">
        <EmptyState title="Loading file manager" hint="Reading allowed folders from the server." />
      </Page>
    );
  }

  return (
    <Page
      title="Files"
      subtitle={currentLabel}
      eyebrow="storage"
      actions={
        <>
          <button type="button" class="btn" disabled={!currentPath} onClick={() => setAction('folder')}><IconPlus /> Folder</button>
          <button type="button" class="btn" disabled={!currentPath} onClick={() => setAction('file')}><IconFile /> File</button>
          <button type="button" class="btn primary" disabled={!currentPath} onClick={() => setAction('upload')}><IconPlus /> Upload</button>
        </>
      }
    >
      <div class="toolbar" style="margin-bottom:14px;">
        {roots.map((root) => (
          <button
            type="button"
            key={root.path}
            class={`btn ${currentPath === root.path ? 'active' : ''}`}
            onClick={() => navigate('/files', { path: root.path })}
          >
            <IconFolder /> {root.label ?? 'Root'}
          </button>
        ))}
        <span class="spacer" />
        <Crumbs path={currentPath} roots={roots} />
      </div>
      <AsyncView
        state={state}
        isEmpty={(rows) => rows.length === 0}
        empty={
          <EmptyState
            title="This folder is empty"
            hint={currentPath ? 'Create a folder or upload a file to get started.' : 'Pick a folder above to browse files.'}
          />
        }
      >
        {(rows) => (
          <div class="card">
            <div class="table-wrap">
              <table class="table">
                <thead><tr><th>Name</th><th>Mode</th><th>Modified</th><th class="right">Size</th><th class="right">Actions</th></tr></thead>
                <tbody>
                  {rows.map((entry) => (
                    <tr key={entry.path} class="file-row">
                      <td>
                        <span class="row" style="gap:8px;">
                          <span style={`color:${entry.is_dir ? 'var(--accent)' : 'var(--text-dim)'};`}>
                            {entry.is_dir ? <IconFolder /> : <IconFile />}
                          </span>
                          <span>{entry.name}</span>
                        </span>
                        <div class="mono dim" style="font-size:11px;margin-top:2px;padding-left:26px;">{entry.path}</div>
                      </td>
                      <td class="mono">{entry.mode}</td>
                      <td>{formatDate(entry.mod_time)}</td>
                      <td class="right">{entry.is_dir ? <span class="dim mono">dir</span> : formatBytes(entry.size)}</td>
                      <td class="right nowrap">
                        <button type="button" class="btn sm ghost" onClick={() => open(entry)}>{entry.is_dir ? <IconFolder /> : <IconFile />} Open</button>
                        <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setAction('copy'); }}><IconEdit /> Copy</button>
                        <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setAction('chmod'); }}><IconSettings /> Mode</button>
                        <button type="button" class="btn sm danger" onClick={() => deleteEntry(entry)}><IconTrash /></button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AsyncView>
      {action && currentPath && (
        <FileActionModal
          action={action}
          dir={currentPath}
          entry={selected}
          onClose={() => { setAction(null); setSelected(null); }}
          onDone={() => { setAction(null); setSelected(null); state.reload(); }}
        />
      )}
    </Page>
  );
}
