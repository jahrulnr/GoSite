import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { files } from '../api/endpoints';
import type { FileEntry, FileListResponse } from '../api/types';
import {
  IconArrowUp,
  IconCopy,
  IconExtract,
  IconFile,
  IconFolder,
  IconMove,
  IconPlus,
  IconRefresh,
  IconSave,
  IconSettings,
  IconTrash,
  IconClose,
} from '../components/Icons';
import { AsyncView, EmptyState, ErrorState, Field, Modal, Spinner } from '../components/Ui';
import { CodeEditor } from '../components/CodeEditor';
import { Page } from '../components/Layout';
import { formatBytes, formatDate } from '../lib/format';
import { humanizeError } from '../lib/errors';
import { useAction, useAsync } from '../lib/hooks';
import { navigate } from '../lib/router';
import { fileRootsFromMeta } from '../lib/meta';
import { detectLanguage, displayKind } from '../lib/editorLanguage';
import { lintDiagnosticForFile, validatorForFile } from '../lib/fileValidate';
import { useStore } from '../lib/store';

type ModalAction = 'file' | 'folder' | 'upload' | 'copy' | 'move' | 'chmod' | 'extract';
type DraftMap = Record<string, { content: string; dirty: boolean; entry: FileEntry }>;
type Diagnostic = { kind: 'ok' | 'danger' | 'warn'; message: string };

const textKinds = new Set(['text', 'json', 'yaml', 'config']);

function parentPath(path: string) {
  if (!path || path === '/') return '/';
  const trimmed = path.replace(/\/+$/, '');
  const idx = trimmed.lastIndexOf('/');
  return idx <= 0 ? '/' : trimmed.slice(0, idx);
}

function joinPath(dir: string, name: string) {
  return `${dir.replace(/\/+$/, '')}/${name}`.replace(/^\/\//, '/');
}

function hasExternalFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files');
}

function entryIcon(entry: FileEntry) {
  return entry.is_dir ? <IconFolder /> : <IconFile />;
}

function entryTone(entry: FileEntry) {
  if (entry.is_dir) return 'folder';
  if (entry.archive) return 'archive';
  if (entry.kind === 'image') return 'image';
  if (textKinds.has(entry.kind)) return 'text';
  return 'binary';
}

function lintContent(entry: FileEntry | undefined, content: string): Diagnostic {
  if (!entry) return { kind: 'warn', message: 'No file selected.' };
  return lintDiagnosticForFile(entry, content);
}

function formatContent(entry: FileEntry | undefined, content: string) {
  if (!entry) return content;
  const lang = detectLanguage(entry.path, entry, content);
  if (lang === 'json') return `${JSON.stringify(JSON.parse(content), null, 2)}\n`;
  if (lang === 'yaml') {
    return content
      .split('\n')
      .map((line) => line.replace(/\s+$/g, ''))
      .join('\n')
      .replace(/\n*$/, '\n');
  }
  if (entry.extension === 'css') return content.replace(/\s*{\s*/g, ' {\n  ').replace(/;\s*/g, ';\n  ').replace(/\s*}\s*/g, '\n}\n');
  return content.replace(/[ \t]+$/gm, '').replace(/\n*$/, '\n');
}

function pathCrumbs(path: string) {
  const clean = path || '/';
  if (clean === '/') return [{ label: '/', path: '/' }];
  const parts = clean.split('/').filter(Boolean);
  const crumbs = [{ label: '/', path: '/' }];
  let current = '';
  for (const part of parts) {
    current += `/${part}`;
    crumbs.push({ label: part, path: current });
  }
  return crumbs;
}

function FileActionModal({
  action,
  dir,
  entry,
  onClose,
  onDone,
}: Readonly<{
  action: ModalAction;
  dir: string;
  entry: FileEntry | null;
  onClose: () => void;
  onDone: () => void;
}>) {
  const { toast } = useStore();
  const [name, setName] = useState('');
  const [content, setContent] = useState('');
  const [toPath, setToPath] = useState(entry ? joinPath(dir, entry.name) : dir);
  const [mode, setMode] = useState(entry?.mode.replace(/^[^0-7]*/, '').slice(-3) ?? '');
  const [uploadFiles, setUploadFiles] = useState<File[]>([]);
  const createPath = name ? joinPath(dir, name) : dir;
  const createLanguage = detectLanguage(createPath, undefined, content);
  const createValidator = validatorForFile({ path: createPath, kind: '', extension: name.split('.').pop() ?? '' }, content);
  const run = useAction(async () => {
    if (action === 'file') return files.createFile(dir, name, content);
    if (action === 'folder') return files.createFolder(dir, name);
    if (action === 'upload') return files.uploadMany(dir, uploadFiles);
    if (action === 'copy' && entry) return files.action('copy', entry.path, { to_path: toPath });
    if (action === 'move' && entry) return files.action('move', entry.path, { to_path: toPath });
    if (action === 'chmod' && entry) return files.action('chmod', entry.path, { mode });
    if (action === 'extract' && entry) return files.action('extract', entry.path, { to_path: toPath });
    return Promise.resolve({ message: 'ok' });
  });

  const submit = async (event: Event) => {
    event.preventDefault();
    try {
      await run.run();
      toast(action === 'upload' ? `${uploadFiles.length} file(s) uploaded` : 'File action completed');
      onDone();
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const title = {
    file: 'Create file',
    folder: 'Create folder',
    upload: 'Upload files',
    copy: 'Copy path',
    move: 'Move path',
    chmod: 'Change mode',
    extract: 'Extract archive',
  }[action];

  return (
    <Modal title={title} onClose={onClose} footer={<button type="submit" form="file-action" class="btn primary" disabled={run.loading}>{run.loading ? <Spinner /> : 'Apply'}</button>}>
      <form id="file-action" onSubmit={submit}>
        {(action === 'file' || action === 'folder') && <Field label="Name"><input class="input" value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} required /></Field>}
        {action === 'file' && (
          <Field label="Content">
            <CodeEditor
              value={content}
              onChange={setContent}
              language={createLanguage}
              minHeight="240px"
              placeholder="# file contents"
              onValidate={createValidator}
              validateDelayMs={createLanguage === 'json' || createLanguage === 'yaml' ? 400 : 800}
            />
          </Field>
        )}
        {action === 'upload' && (
          <Field label="Files">
            <input class="input" type="file" multiple onChange={(e) => setUploadFiles(Array.from((e.target as HTMLInputElement).files ?? []))} required />
          </Field>
        )}
        {(action === 'copy' || action === 'move' || action === 'extract') && (
          <Field label={action === 'extract' ? 'Destination directory' : 'Destination path'} hint={entry?.path}>
            <input class="input mono" value={toPath} onInput={(e) => setToPath((e.target as HTMLInputElement).value)} required />
          </Field>
        )}
        {action === 'chmod' && <Field label="Mode" hint={entry?.mode}><input class="input mono" value={mode} onInput={(e) => setMode((e.target as HTMLInputElement).value)} placeholder="0644" required /></Field>}
        {run.error && <ErrorState error={run.error} />}
      </form>
    </Modal>
  );
}

function FileViewerContent({
  selected,
  draft,
  onDraft,
  onFormat,
  onSave,
  saving,
}: Readonly<{
  selected: FileEntry | null;
  draft: DraftMap[string] | undefined;
  onDraft: (path: string, content: string, entry: FileEntry) => void;
  onFormat: () => void;
  onSave: () => void;
  saving: boolean;
}>) {
  const editorLanguage = useMemo(
    () => (selected ? detectLanguage(selected.path, selected, draft?.content ?? '') : 'text'),
    [selected, draft?.content],
  );
  const editorValidator = useMemo(
    () => (selected ? validatorForFile(selected, draft?.content ?? '') : undefined),
    [selected, draft?.content],
  );
  const diagnostic = useMemo(() => lintContent(selected ?? undefined, draft?.content ?? ''), [selected, draft?.content]);
  if (!selected) {
    return null;
  }
  if (selected.kind === 'image') {
    return (
      <div class="file-preview image-preview">
        <img src={files.rawUrl(selected.path)} alt={selected.name} />
      </div>
    );
  }
  if (!selected.editable) {
    return (
      <div class="file-preview">
        <div class={`file-kind ${entryTone(selected)}`}>{selected.kind}</div>
        <h3>{selected.name}</h3>
        <p>{selected.archive ? 'Archive ready to extract from the action bar.' : 'This file type is not editable in the browser.'}</p>
      </div>
    );
  }
  if (selected.editable && !draft) {
    return (
      <div class="file-preview">
        <Spinner />
      </div>
    );
  }
  return (
    <div class="file-editor">
      <div class="file-editor-head">
        <div>
          <strong>{selected.name}</strong>
          <span class="mono dim">{selected.path}</span>
        </div>
        <div class="row">
          <span class={`diagnostic ${diagnostic.kind}`}>{diagnostic.message}</span>
          <button type="button" class="btn sm" onClick={onFormat}>Format</button>
          <button type="button" class="btn sm primary" disabled={saving || !draft?.dirty} onClick={onSave}>{saving ? <Spinner /> : 'Save'}</button>
        </div>
      </div>
      <CodeEditor
        value={draft?.content ?? ''}
        onChange={(content) => onDraft(selected.path, content, selected)}
        language={editorLanguage}
        className="file-editor-text"
        minHeight="420px"
        onValidate={editorValidator}
        validateDelayMs={editorLanguage === 'json' || editorLanguage === 'yaml' ? 400 : 800}
      />
    </div>
  );
}

function FileViewerModal({
  entry,
  draft,
  onClose,
  onDraft,
  onFormat,
  onSave,
  saving,
}: Readonly<{
  entry: FileEntry;
  draft: DraftMap[string] | undefined;
  onClose: () => void;
  onDraft: (path: string, content: string, entry: FileEntry) => void;
  onFormat: () => void;
  onSave: () => void;
  saving: boolean;
}>) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (dialog && !dialog.open) dialog.showModal();
  }, []);

  return (
    <dialog
      ref={ref}
      class="modal lg file-viewer-modal"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <div class="modal-head">
        <h3>{entry.name}</h3>
        <button type="button" class="btn ghost sm" aria-label="Close" onClick={onClose}>
          <IconClose width={16} height={16} />
        </button>
      </div>
      <div class="modal-body file-viewer-body">
        <FileViewerContent
          selected={entry}
          draft={draft}
          onDraft={onDraft}
          onFormat={onFormat}
          onSave={onSave}
          saving={saving}
        />
      </div>
    </dialog>
  );
}

export function FilesView({ path }: Readonly<{ path: string }>) {
  const { meta, toast } = useStore();
  const roots = fileRootsFromMeta(meta);
  const rootPath = roots[0]?.path ?? '/';
  const currentPath = path || rootPath;
  const state = useAsync<FileListResponse>(
    () => (currentPath ? files.browse(currentPath) : Promise.resolve({ entries: [], tools: { unzip: false, tar: false, gzip: false } })),
    [currentPath],
  );
  const [typedPath, setTypedPath] = useState(currentPath);
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [viewer, setViewer] = useState<FileEntry | null>(null);
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const [drafts, setDrafts] = useState<DraftMap>({});
  const [modal, setModal] = useState<ModalAction | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [moveTarget, setMoveTarget] = useState('');
  const remove = useAction(files.remove);
  const batchDelete = useAction(files.batchDelete);
  const saveOne = useAction((item: { path: string; content: string }) => files.save(item.path, item.content));
  const saveBatch = useAction(files.batchSave);

  useEffect(() => {
    if (!path && rootPath) navigate('/files', { path: rootPath });
  }, [path, rootPath]);

  useEffect(() => {
    setTypedPath(currentPath);
    setChecked({});
  }, [currentPath]);

  const rows = state.data?.entries ?? [];
  const archiveTools = state.data?.tools ?? { unzip: false, tar: false, gzip: false };
  const selectedPaths = Object.entries(checked).filter(([, value]) => value).map(([key]) => key);
  const selectedEntries = rows.filter((entry) => checked[entry.path]);
  const dirtyDrafts = Object.values(drafts).filter((item) => item.dirty);

  const openEntry = async (entry: FileEntry) => {
    if (entry.is_dir) {
      navigate('/files', { path: entry.path });
      return;
    }
    setViewer(entry);
    if (entry.editable && !drafts[entry.path]) {
      const result = await files.read(entry.path);
      setDrafts((prev) => ({ ...prev, [entry.path]: { content: result.content, dirty: false, entry: result.entry } }));
    }
  };

  const deleteEntry = async (entry: FileEntry) => {
    if (!globalThis.confirm(`Delete ${entry.path}?`)) return;
    await remove.run(entry.path);
    toast(`${entry.name} deleted`);
    if (selected?.path === entry.path) setSelected(null);
    if (viewer?.path === entry.path) setViewer(null);
    state.reload();
  };

  const runBatchDelete = async () => {
    if (selectedPaths.length === 0) return;
    if (!globalThis.confirm(`Delete ${selectedPaths.length} selected path(s)?`)) return;
    await batchDelete.run(selectedPaths);
    toast(`${selectedPaths.length} path(s) deleted`);
    setChecked({});
    setSelected(null);
    setViewer(null);
    state.reload();
  };

  const runBatchSave = async () => {
    try {
      await saveBatch.run(dirtyDrafts.map((item) => ({ path: item.entry.path, content: item.content })));
      setDrafts((prev) => Object.fromEntries(Object.entries(prev).map(([key, item]) => [key, { ...item, dirty: false }])));
      toast(`${dirtyDrafts.length} draft(s) saved`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const copySelectedToParent = async () => {
    const destinationDir = parentPath(currentPath);
    for (const entry of selectedEntries) {
      await files.action('copy', entry.path, { to_path: joinPath(destinationDir, entry.name) });
    }
    toast(`${selectedEntries.length} path(s) copied to parent`);
    state.reload();
  };

  const moveSelectedToParent = async () => {
    const destinationDir = parentPath(currentPath);
    for (const entry of selectedEntries) {
      await files.action('move', entry.path, { to_path: joinPath(destinationDir, entry.name) });
    }
    toast(`${selectedEntries.length} path(s) moved to parent`);
    setChecked({});
    state.reload();
  };

  const saveSelected = async () => {
    if (!viewer) return;
    const draft = drafts[viewer.path];
    if (!draft) return;
    try {
      await saveOne.run({ path: viewer.path, content: draft.content });
      setDrafts((prev) => ({ ...prev, [viewer.path]: { ...draft, dirty: false } }));
      toast(`${viewer.name} saved`);
      state.reload();
    } catch (err) {
      toast(humanizeError(err as Error), 'error');
    }
  };

  const formatSelected = () => {
    if (!viewer) return;
    const draft = drafts[viewer.path];
    if (!draft) return;
    try {
      const content = formatContent(viewer, draft.content);
      setDrafts((prev) => ({ ...prev, [viewer.path]: { ...draft, content, dirty: true } }));
    } catch (error) {
      toast((error as Error).message, 'error');
    }
  };

  const moveDroppedPath = async (event: DragEvent, destinationDir: string) => {
    event.preventDefault();
    event.stopPropagation();
    setMoveTarget('');
    setDragOver(false);
    const movedPath = event.dataTransfer?.getData('text/gosite-path');
    if (!movedPath || movedPath === destinationDir || parentPath(movedPath) === destinationDir) return;
    await files.action('move', movedPath, { to_path: joinPath(destinationDir, movedPath.split('/').pop() ?? 'moved') });
    toast('Path moved');
    state.reload();
  };

  const uploadDropped = async (event: DragEvent) => {
    event.preventDefault();
    setDragOver(false);
    if (!hasExternalFiles(event)) return;
    const uploadFiles = Array.from(event.dataTransfer?.files ?? []);
    if (uploadFiles.length > 0) {
      await files.uploadMany(currentPath, uploadFiles);
      toast(`${uploadFiles.length} file(s) uploaded`);
      state.reload();
    }
  };

  const dropExternalOnDirectory = async (event: DragEvent, destinationDir: string) => {
    if (!hasExternalFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    setDragOver(false);
    const uploadFiles = Array.from(event.dataTransfer?.files ?? []);
    if (uploadFiles.length > 0) {
      await files.uploadMany(destinationDir, uploadFiles);
      toast(`${uploadFiles.length} file(s) uploaded`);
      state.reload();
    }
  };

  if (!meta) {
    return (
      <Page title="Files" subtitle="Loading folders..." eyebrow="storage">
        <EmptyState title="Loading file manager" hint="Reading folders from the server." />
      </Page>
    );
  }

  return (
    <Page
      title="Files"
      subtitle={currentPath}
      eyebrow="storage"
      actions={
        <>
          <button type="button" class="btn" onClick={() => setModal('folder')}><IconPlus /> Folder</button>
          <button type="button" class="btn" onClick={() => setModal('file')}><IconFile /> File</button>
          <button type="button" class="btn primary" onClick={() => setModal('upload')}><IconPlus /> Upload</button>
        </>
      }
    >
      <div
        class={`file-manager ${dragOver ? 'drag-over' : ''}`}
        onDragOver={(event) => {
          if (!hasExternalFiles(event)) return;
          event.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={uploadDropped}
      >
        <aside class="file-sidebar">
          <div class="file-path-box">
            <Field label="Path">
              <form onSubmit={(event) => { event.preventDefault(); navigate('/files', { path: typedPath || '/' }); }}>
                <input class="input mono" value={typedPath} onInput={(e) => setTypedPath((e.target as HTMLInputElement).value)} />
              </form>
            </Field>
          </div>
          <div class="file-roots">
            {roots.map((root) => (
              <button
                type="button"
                class={`${currentPath === root.path ? 'active' : ''} ${moveTarget === root.path ? 'drop-target' : ''}`}
                onClick={() => navigate('/files', { path: root.path })}
                onDragOver={(event) => {
                  if (hasExternalFiles(event) || event.dataTransfer?.types.includes('text/gosite-path')) {
                    event.preventDefault();
                    setMoveTarget(root.path);
                  }
                }}
                onDragLeave={() => setMoveTarget('')}
                onDrop={(event) => hasExternalFiles(event) ? void dropExternalOnDirectory(event, root.path) : void moveDroppedPath(event, root.path)}
                key={root.path}
              >
                <IconFolder /> <span>{root.label ?? root.path}</span><small>{root.path}</small>
              </button>
            ))}
          </div>
          <div class="file-tools">
            <span>Archive tools</span>
            <b class={archiveTools.unzip ? 'on' : ''}>zip</b>
            <b class={archiveTools.tar ? 'on' : ''}>tar</b>
            <b class={archiveTools.gzip ? 'on' : ''}>gz</b>
          </div>
        </aside>

        <section class="file-main">
          <div class="file-toolbar">
            <div class="crumbs">
              {pathCrumbs(currentPath).map((crumb, index) => (
                <>
                  {index > 0 && <span class="sep">/</span>}
                  <a
                    key={crumb.path}
                    class={moveTarget === crumb.path ? 'drop-target' : ''}
                    onClick={() => navigate('/files', { path: crumb.path })}
                    onDragOver={(event) => {
                      if (event.dataTransfer?.types.includes('text/gosite-path')) {
                        event.preventDefault();
                        setMoveTarget(crumb.path);
                      }
                    }}
                    onDragLeave={() => setMoveTarget('')}
                    onDrop={(event) => void moveDroppedPath(event, crumb.path)}
                  >
                    {crumb.label}
                  </a>
                </>
              ))}
            </div>
            <span class="grow" />
            <button type="button" class="btn sm" disabled={currentPath === '/'} onClick={() => navigate('/files', { path: parentPath(currentPath) })} title="Up to parent"><IconArrowUp /></button>
            <button type="button" class="btn sm" onClick={state.reload} title="Refresh"><IconRefresh /></button>
            <button type="button" class="btn sm" disabled={selectedEntries.length === 0 || currentPath === '/'} onClick={copySelectedToParent} title="Copy selected to parent"><IconCopy /></button>
            <button type="button" class="btn sm" disabled={selectedEntries.length === 0 || currentPath === '/'} onClick={moveSelectedToParent} title="Move selected to parent"><IconMove /></button>
            <button type="button" class="btn sm" disabled={dirtyDrafts.length === 0 || saveBatch.loading} onClick={runBatchSave} title="Save all drafts"><IconSave /></button>
            <button type="button" class="btn sm danger" disabled={selectedPaths.length === 0 || batchDelete.loading} onClick={runBatchDelete} title="Delete selected"><IconTrash /></button>
          </div>

          <AsyncView
            state={state}
            isEmpty={(data) => data.entries.length === 0}
            empty={<EmptyState title="This folder is empty" hint="Drop files here or create a new folder." />}
          >
            {(data) => (
              <div class="file-grid">
                <div class="file-table-wrap">
                  <table class="table file-table">
                    <thead><tr><th><input type="checkbox" checked={rows.length > 0 && selectedPaths.length === rows.length} onChange={(e) => {
                      const next = (e.target as HTMLInputElement).checked;
                      setChecked(Object.fromEntries(rows.map((entry) => [entry.path, next])));
                    }} /></th><th>Name</th><th>Kind</th><th>Mode</th><th>Modified</th><th class="right">Size</th><th class="right">Actions</th></tr></thead>
                    <tbody>
                      {data.entries.map((entry) => (
                        <tr
                          key={entry.path}
                          class={`file-row ${viewer?.path === entry.path ? 'selected' : ''} ${moveTarget === entry.path ? 'drop-target' : ''}`}
                          draggable
                          onDragStart={(event) => {
                            event.dataTransfer?.setData('text/gosite-path', entry.path);
                            event.dataTransfer?.setData('text/plain', entry.path);
                          }}
                          onDragOver={(event) => {
                            if (!entry.is_dir) return;
                            if (hasExternalFiles(event) || event.dataTransfer?.types.includes('text/gosite-path')) {
                              event.preventDefault();
                              setMoveTarget(entry.path);
                            }
                          }}
                          onDragLeave={() => setMoveTarget('')}
                          onDrop={(event) => {
                            if (!entry.is_dir) return;
                            if (hasExternalFiles(event)) void dropExternalOnDirectory(event, entry.path);
                            else void moveDroppedPath(event, entry.path);
                          }}
                          onDblClick={() => void openEntry(entry)}
                        >
                          <td><input type="checkbox" checked={Boolean(checked[entry.path])} onChange={(e) => setChecked((prev) => ({ ...prev, [entry.path]: (e.target as HTMLInputElement).checked }))} /></td>
                          <td>
                            <button type="button" class={`file-name ${entryTone(entry)}`} onClick={() => void openEntry(entry)}>
                              {entryIcon(entry)} <span>{entry.name}</span>
                            </button>
                            <div class="mono dim file-path">{entry.symlink ? `${entry.path} -> ${entry.target}` : entry.path}</div>
                          </td>
                          <td><span class={`file-kind ${entryTone(entry)}`}>{displayKind(entry)}</span></td>
                          <td class="mono">{entry.mode}</td>
                          <td>{formatDate(entry.mod_time)}</td>
                          <td class="right">{entry.is_dir ? <span class="dim mono">dir</span> : formatBytes(entry.size)}</td>
                          <td class="right nowrap">
                            <button type="button" class="btn sm ghost" onClick={() => void openEntry(entry)} title={entry.is_dir ? 'Open folder' : 'Open file'}>
                              {entry.is_dir ? <IconFolder /> : <IconFile />}
                            </button>
                            <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setModal('copy'); }} title="Copy"><IconCopy /></button>
                            <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setModal('move'); }} title="Move"><IconMove /></button>
                            {entry.archive && (
                              <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setModal('extract'); }} title="Extract"><IconExtract /></button>
                            )}
                            <button type="button" class="btn sm ghost" onClick={() => { setSelected(entry); setModal('chmod'); }} title="Change permissions"><IconSettings /></button>
                            <button type="button" class="btn sm danger" onClick={() => void deleteEntry(entry)} title="Delete"><IconTrash /></button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </AsyncView>
          {(remove.error || batchDelete.error || saveBatch.error || saveOne.error) && <ErrorState error={(remove.error ?? batchDelete.error ?? saveBatch.error ?? saveOne.error) as Error} />}
        </section>
      </div>

      {modal && (
        <FileActionModal
          action={modal}
          dir={currentPath}
          entry={selected}
          onClose={() => setModal(null)}
          onDone={() => { setModal(null); state.reload(); }}
        />
      )}
      {viewer && (
        <FileViewerModal
          entry={viewer}
          draft={drafts[viewer.path]}
          onClose={() => setViewer(null)}
          onDraft={(draftPath, content, entry) => setDrafts((prev) => ({ ...prev, [draftPath]: { content, dirty: true, entry } }))}
          onFormat={formatSelected}
          onSave={saveSelected}
          saving={saveOne.loading}
        />
      )}
    </Page>
  );
}
