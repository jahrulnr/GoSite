import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/preact';
import type { ComponentChildren } from 'preact';
import { useEffect } from 'preact/hooks';
import { AppProvider, useStore } from '../lib/store';
import type { FileEntry, FileListResponse, UiMetaResponse } from '../api/types';
import { FilesView } from './Files';

vi.mock('../components/Layout', () => ({
  Page: ({ title, subtitle, children, actions }: {
    title: string;
    subtitle?: string;
    children: ComponentChildren;
    actions?: ComponentChildren;
  }) => (
    <div data-testid="page">
      <h1>{title}</h1>
      {subtitle ? <p data-testid="subtitle">{subtitle}</p> : null}
      <div data-testid="actions">{actions}</div>
      {children}
    </div>
  ),
}));

const FULL_META: UiMetaResponse = {
  app: { name: 'GoSite', env: 'production', logo_letter: 'G' },
  auth: {
    login_hint: '',
    remember_me: false,
    basic_auth_enabled: true,
    lockscreen_enabled: false,
    lock_after_seconds: 0,
  },
  navigation: [],
  files: {
    roots: [
      { path: '/www', label: 'Websites' },
      { path: '/storage', label: 'Storage' },
      { path: '/tmp', label: 'Temp' },
    ],
    actions: [],
  },
  logs: { tail_kinds: [] },
  nginx: { test: { enabled: true }, reload: { enabled: true }, stub_status: { enabled: false }, vts: { enabled: false } },
  cron: { run_every_options: [], manual_run: { enabled: true } },
  mounts: { default_options: '', dump_default: '', fsck_default: '', fs_types: [], example: '' },
  traffic: { ranges: [] },
  docker: { restart: { enabled: true }, stop: { enabled: true }, logs: { enabled: true } },
  websites: { types: [], web_root: '/www', static_path_hint: '', proxy_upstream_hint: '' },
};

/** Simulates BootGate state after login when ui/meta failed pre-session. */
const PARTIAL_META = {
  ...FULL_META,
  files: { roots: [], actions: [] },
  auth: {
    ...FULL_META.auth,
    file_roots: [
      { path: '/www', label: 'Websites' },
      { path: '/storage', label: 'Storage' },
      { path: '/tmp', label: 'Temp' },
    ],
  },
} as UiMetaResponse;

function entry(partial: Partial<FileEntry> & Pick<FileEntry, 'name' | 'path' | 'is_dir'>): FileEntry {
  return {
    size: partial.is_dir ? 4096 : 106496,
    mode: partial.is_dir ? 'drwxr-xr-x' : '-rw-r--r--',
    mod_time: '2026-06-14T15:15:11Z',
    kind: partial.is_dir ? 'directory' : 'text',
    mime_type: partial.is_dir ? 'inode/directory' : 'text/plain',
    extension: '',
    editable: !partial.is_dir,
    viewable: !partial.is_dir,
    archive: false,
    symlink: false,
    ...partial,
  };
}

const BROWSE_BY_PATH: Record<string, FileListResponse> = {
  '/www': { entries: [entry({ name: 'default', path: '/www/default', is_dir: true })], tools: { unzip: true, tar: true, gzip: true } },
  '/storage': { entries: [entry({ name: 'db.sqlite', path: '/storage/db.sqlite', is_dir: false })], tools: { unzip: true, tar: true, gzip: true } },
  '/www/default': { entries: [entry({ name: 'index.txt', path: '/www/default/index.txt', is_dir: false })], tools: { unzip: true, tar: true, gzip: true } },
  '/tmp': { entries: [], tools: { unzip: true, tar: true, gzip: true } },
};

function SetMeta({ meta }: Readonly<{ meta: UiMetaResponse | undefined }>): ComponentChildren {
  const { setMeta } = useStore();
  useEffect(() => {
    setMeta(meta);
  }, [meta, setMeta]);
  return null;
}

describe('FilesView', () => {
  let browse: ReturnType<typeof vi.fn>;
  let action: ReturnType<typeof vi.fn>;
  let uploadMany: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    browse = vi.fn(async (p: string) => BROWSE_BY_PATH[p] ?? { entries: [], tools: { unzip: false, tar: false, gzip: false } });
    action = vi.fn(async () => ({ message: 'ok' }));
    uploadMany = vi.fn(async () => ({ message: 'uploaded' }));
    const endpoints = await import('../api/endpoints');
    const fileApi = (endpoints as unknown as { files: { browse: typeof browse; action: typeof action; uploadMany: typeof uploadMany } }).files;
    fileApi.browse = browse;
    fileApi.action = action;
    fileApi.uploadMany = uploadMany;
  });

  afterEach(() => {
    cleanup();
  });

  it('lists files when path is set explicitly', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="/storage" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/storage'));
    expect(document.body.textContent).toContain('db.sqlite');
  });

  it('lists files when path is empty but meta has files.roots', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/www'));
    expect(document.body.textContent).toContain('default');
    expect(document.body.textContent).not.toContain('Pick a folder above');
  });

  it('lists files when only auth.file_roots is present (post-login regression)', async () => {
    render(
      <AppProvider>
        <SetMeta meta={PARTIAL_META} />
        <FilesView path="" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/www'));
    expect(document.body.textContent).toContain('default');
    expect(document.body.textContent).not.toContain('Pick a folder above');
  });

  it('shows empty state when the directory has no entries', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="/tmp" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/tmp'));
    expect(document.body.textContent).toContain('This folder is empty');
  });

  it('does not move a dragged internal path when dropped on the broad browser surface', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="/www" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/www'));
    const panel = document.querySelector('.file-manager');
    expect(panel).toBeTruthy();
    fireEvent.drop(panel as Element, {
      dataTransfer: {
        types: ['text/gosite-path'],
        files: [],
        getData: (key: string) => key === 'text/gosite-path' ? '/www/default' : '',
      },
    });
    expect(action).not.toHaveBeenCalled();
    expect(uploadMany).not.toHaveBeenCalled();
  });

  it('moves a dragged internal path only when dropped on an explicit folder target', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="/www" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/www'));
    const row = screen.getByText('default').closest('tr');
    expect(row).toBeTruthy();
    fireEvent.drop(row as Element, {
      dataTransfer: {
        types: ['text/gosite-path'],
        files: [],
        getData: (key: string) => key === 'text/gosite-path' ? '/storage/db.sqlite' : '',
      },
    });
    await waitFor(() => expect(action).toHaveBeenCalledWith('move', '/storage/db.sqlite', { to_path: '/www/default/db.sqlite' }));
  });

  it('copies and moves selected entries to the parent directory', async () => {
    render(
      <AppProvider>
        <SetMeta meta={FULL_META} />
        <FilesView path="/www/default" />
      </AppProvider>,
    );
    await waitFor(() => expect(browse).toHaveBeenCalledWith('/www/default'));
    const rowCheckbox = document.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')[1];
    fireEvent.click(rowCheckbox);
    fireEvent.click(screen.getByTitle('Copy selected to parent'));
    await waitFor(() => expect(action).toHaveBeenCalledWith('copy', '/www/default/index.txt', { to_path: '/www/index.txt' }));
    fireEvent.click(screen.getByTitle('Move selected to parent'));
    await waitFor(() => expect(action).toHaveBeenCalledWith('move', '/www/default/index.txt', { to_path: '/www/index.txt' }));
  });
});
