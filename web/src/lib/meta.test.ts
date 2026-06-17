import { describe, expect, it } from 'vitest';
import { fileRootsFromMeta, mergePanelMeta } from './meta';
import type { UiMetaResponse } from '../api/types';

const UI_META: UiMetaResponse = {
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

describe('mergePanelMeta', () => {
  it('keeps files.roots from ui/meta when present', () => {
    const merged = mergePanelMeta(UI_META, { lockscreen_enabled: true, basic_auth_enabled: true, lock_after_seconds: 300 });
    expect(merged?.files?.roots).toHaveLength(2);
    expect(merged?.auth?.lock_after_seconds).toBe(300);
  });

  it('fills files.roots from auth.file_roots when ui/meta is missing (post-login regression)', () => {
    const merged = mergePanelMeta(undefined, {
      lockscreen_enabled: true,
      basic_auth_enabled: true,
      lock_after_seconds: 300,
      file_roots: [
        { path: '/www', label: 'Websites' },
        { path: '/storage', label: 'Storage' },
        { path: '/tmp', label: 'Temp' },
      ],
    });
    expect(merged?.files?.roots).toHaveLength(3);
    expect(merged?.files?.roots?.[0]?.path).toBe('/www');
  });
});

describe('fileRootsFromMeta', () => {
  it('prefers files.roots', () => {
    expect(fileRootsFromMeta(UI_META)).toHaveLength(2);
  });

  it('falls back to auth.file_roots', () => {
    const meta = {
      ...UI_META,
      files: { roots: [], actions: [] },
      auth: {
        ...UI_META.auth,
        file_roots: [{ path: '/storage', label: 'Storage' }],
      },
    } as UiMetaResponse;
    expect(fileRootsFromMeta(meta)).toEqual([{ path: '/storage', label: 'Storage' }]);
  });
});
