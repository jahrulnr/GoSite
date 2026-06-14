import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/preact';
import { AppProvider, useStore } from '../lib/store';
import type { ComponentChildren } from 'preact';
import { useEffect } from 'preact/hooks';
import type { UiMetaResponse } from '../api/types';
import { WebsiteModal } from './Websites';

vi.mock('../components/JobStream', () => ({ JobStreamModal: () => null }));
vi.mock('../components/Layout', () => ({
  Page: ({ children }: { children: ComponentChildren }) => <div>{children}</div>,
  optionLabel: (o: { value: string; label: string }) => o.label,
}));

const META: UiMetaResponse = {
  app: { name: 'GoSite', env: 'production', logo_letter: 'G' },
  auth: {
    login_hint: '',
    remember_me: false,
    basic_auth_enabled: true,
    lockscreen_enabled: false,
    lock_after_seconds: 0,
  },
  navigation: [],
  files: { roots: [], actions: [] },
  logs: { tail_kinds: [] },
  nginx: { test: { enabled: true }, reload: { enabled: true } },
  cron: { run_every_options: [], manual_run: { enabled: true } },
  mounts: { default_options: '', dump_default: '', fsck_default: '', fs_types: [], example: '' },
  traffic: { ranges: [] },
  docker: { restart: { enabled: true }, stop: { enabled: true }, logs: { enabled: true } },
  websites: {
    types: [
      { value: 'static', label: 'Static site', hint: 'Serve files from a folder.' },
      { value: 'proxy', label: 'Reverse proxy', hint: 'Forward traffic to an upstream service.' },
    ],
    web_root: '/www',
    static_path_hint: '/www/example-site',
    proxy_upstream_hint: 'http://127.0.0.1:3000',
  },
};

function SetMeta({ meta }: Readonly<{ meta: UiMetaResponse | undefined }>): ComponentChildren {
  const { setMeta } = useStore();
  useEffect(() => {
    setMeta(meta);
  }, [meta, setMeta]);
  return null;
}

const SAMPLE_SITE = {
  id: 0,
  name: 'BangunInfo',
  domain: 'banguninfo.com',
  path: '',
  type: 'proxy' as const,
  upstream: 'http://127.0.0.1:3000',
  active: false,
  ssl: false,
  config: '',
  created_at: '',
  updated_at: '',
};

describe('WebsiteModal select', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders type select with both options as visible <option> elements', () => {
    const { container } = render(
      <AppProvider>
        <SetMeta meta={META} />
        <WebsiteModal site={SAMPLE_SITE} onClose={vi.fn()} onSaved={vi.fn()} />
      </AppProvider>,
    );
    // jsdom may not open <dialog> as a top-layer; query inside the rendered tree.
    const select = container.querySelector('select.select') as HTMLSelectElement | null;
    expect(select).toBeTruthy();
    const options = Array.from(select!.options);
    expect(options).toHaveLength(2);
    // Regression: option text must equal the label, not the value, and must NOT
    // be empty (some browsers paint a focus halo over the option text if the
    // value attribute is missing or label is empty).
    expect(options[0]?.text).toBe('Static site');
    expect(options[0]?.value).toBe('static');
    expect(options[1]?.text).toBe('Reverse proxy');
    expect(options[1]?.value).toBe('proxy');
    // Reverse proxy is selected by default in this fixture.
    expect(select!.value).toBe('proxy');
  });

  it('preserves Upstream field (not Path) when type is proxy', () => {
    const { container } = render(
      <AppProvider>
        <SetMeta meta={META} />
        <WebsiteModal site={SAMPLE_SITE} onClose={vi.fn()} onSaved={vi.fn()} />
      </AppProvider>,
    );
    const labels = Array.from(container.querySelectorAll('.field > span:first-child')).map(
      (el) => el.textContent,
    );
    // The 3rd field label is dynamic: "Upstream" for proxy, "Path" for static.
    expect(labels).toContain('Upstream');
    expect(labels).not.toContain('Path');
  });});
