import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, cleanup, waitFor, fireEvent } from '@testing-library/preact';
import { AppProvider, useStore } from '../lib/store';
import type { ComponentChildren } from 'preact';
import { useEffect } from 'preact/hooks';
import type { UiMetaResponse } from '../api/types';
import { WebsitesView } from './Websites';

vi.mock('../components/JobStream', () => ({ JobStreamModal: () => null }));
vi.mock('../components/Layout', () => ({
  Page: ({ children }: { children: ComponentChildren }) => <div>{children}</div>,
  optionLabel: (o: { value: string; label: string }) => o.label,
}));

const META: UiMetaResponse = {
  app: { name: 'GoSite', env: 'production', logo_letter: 'G' },
  auth: { login_hint: '', remember_me: false, basic_auth_enabled: true, lockscreen_enabled: false, lock_after_seconds: 0 },
  navigation: [],
  files: { roots: [], actions: [] },
  logs: { tail_kinds: [] },
  nginx: { test: { enabled: true }, reload: { enabled: true }, stub_status: { enabled: false }, vts: { enabled: false } },
  cron: { run_every_options: [], manual_run: { enabled: true } },
  mounts: { default_options: '', dump_default: '', fs_types: [], example: '' },
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

const CONFIG = `# Per-site reverse-proxy vhost.\nserver {\n    listen 80;\n}\n`;

describe('WebsiteConfigModal', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders fetched nginx config in the editor', async () => {
    // CodeMirror's placeholder widget may call layout APIs that jsdom lacks.
    if (!('getClientRects' in Range.prototype)) {
      Range.prototype.getClientRects = () => ({ length: 0, item: () => null }) as DOMRectList;
    }
    if (!('getBoundingClientRect' in Range.prototype)) {
      Range.prototype.getBoundingClientRect = () => new DOMRect();
    }


    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input.url;
      if (url.includes('/api/v1/websites') && url.includes('/nginx-config')) {
        return new Response(JSON.stringify({ config: CONFIG }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        });
      }
      return new Response(JSON.stringify({ websites: [{ id: 4, name: 'PJKR', domain: 'pjkruntagcirebon.org', type: 'proxy', upstream: 'http://web-wp', active: true, ssl: true }] }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      });
    });

    const { container } = render(
      <AppProvider>
        <SetMeta meta={META} />
        <WebsitesView />
      </AppProvider>,
    );

    await waitFor(() => expect(container.querySelector('table')).toBeTruthy());

    const configButton = Array.from(container.querySelectorAll('button')).find((b) => b.getAttribute('aria-label') === 'Nginx config');
    expect(configButton).toBeTruthy();
    fireEvent.click(configButton as HTMLButtonElement);

    await waitFor(() => {
      const content = container.querySelector('.cm-content');
      expect(content?.textContent).toContain('Per-site reverse-proxy vhost');
    });
  });
});
