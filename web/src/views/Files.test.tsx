import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/preact';
import type { ComponentChildren } from 'preact';
import { useEffect } from 'preact/hooks';
import { AppProvider, useStore } from '../lib/store';
import type { UiMetaResponse } from '../api/types';
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
  nginx: { test: { enabled: true }, reload: { enabled: true } },
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

const BROWSE_BY_PATH: Record<string, Array<{ name: string; path: string; size: number; mode: string; is_dir: boolean; mod_time: string }>> = {
  '/www': [{ name: 'default', path: '/www/default', size: 4096, mode: 'drwxr-xr-x', is_dir: true, mod_time: '2026-06-14T15:15:11Z' }],
  '/storage': [{ name: 'db.sqlite', path: '/storage/db.sqlite', size: 106496, mode: '-rw-r--r--', is_dir: false, mod_time: '2026-06-14T16:02:14Z' }],
  '/tmp': [],
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

  beforeEach(async () => {
    browse = vi.fn(async (p: string) => BROWSE_BY_PATH[p] ?? []);
    const endpoints = await import('../api/endpoints');
    (endpoints as unknown as { files: { browse: typeof browse } }).files.browse = browse;
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
});
