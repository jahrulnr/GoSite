import { afterEach, describe, expect, it, vi } from 'vitest';
import { observability, plugins } from './endpoints';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('observability endpoints', () => {
  it('includes the current query in live tail URLs', () => {
    const url = observability.tailUrl({
      source: 'all',
      q: 'curl',
      from: '2026-06-15T00:00:00Z',
    });

    expect(url).toContain('/api/v1/query/tail?');
    expect(url).toContain('source=all');
    expect(url).toContain('q=curl');
    expect(url).toContain('from=2026-06-15T00%3A00%3A00Z');
  });
});

describe('plugin endpoints', () => {
  it('addresses namespaced plugin ids through vendor/name route segments', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ plugin: { id: 1, plugin_id: 'acme/logger' } }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );

    await plugins.enable('acme/logger', '1.0.0');

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/plugins/acme/logger/enable', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ version: '1.0.0' }),
    }));
  });

  it('uses multipart upload for artifact installs', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ plugin: { id: 1, plugin_id: 'acme/logger' } }), {
        status: 201,
        headers: { 'content-type': 'application/json' },
      }),
    );
    const file = new File(['{}'], 'manifest.json', { type: 'application/json' });

    await plugins.installFile(file, 'abc123');

    const [, init] = fetchMock.mock.calls[0];
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/plugins/install');
    expect(init).toEqual(expect.objectContaining({ method: 'POST' }));
    expect(init?.body).toBeInstanceOf(FormData);
  });

  it('fetches remote install settings', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({
        remote_install_enabled: true,
        trust_mode: 'strict',
        allowed_hosts: ['github.com'],
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );

    const settings = await plugins.installSettings();

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/plugins/install/settings', expect.objectContaining({ method: 'GET' }));
    expect(settings.remote_install_enabled).toBe(true);
    expect(settings.trust_mode).toBe('strict');
  });

  it('resolves remote install source', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({
        preview: { plugin_id: 'acme/foo', version: '1.0.0', tier: 1, signed: true, sha256: 'abc', size: 100, url: 'https://x', source_type: 'url', source_ref: 'x', install_path: 'release', permissions: [], hooks: [] },
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );

    const result = await plugins.resolveInstall({ type: 'url', url: 'https://example.com/p.tgz', sha256: 'abc' });

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/plugins/install/resolve', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ source: { type: 'url', url: 'https://example.com/p.tgz', sha256: 'abc' } }),
    }));
    expect(result.preview.plugin_id).toBe('acme/foo');
  });

  it('installs from remote source with permissions ack', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ plugin: { id: 2, plugin_id: 'acme/foo' } }), {
        status: 201,
        headers: { 'content-type': 'application/json' },
      }),
    );

    await plugins.installRemote({ type: 'github-release', repo: 'acme/foo', tag: 'v1.0.0' }, true, 'tok123');

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/plugins/install', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({
        source: { type: 'github-release', repo: 'acme/foo', tag: 'v1.0.0' },
        permissions_ack: true,
        resolveToken: 'tok123',
      }),
    }));
  });

  it('lists and revokes keyring entries', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ keys: [{ vendor: 'acme', keyId: 'k1', publicKey: 'abc' }] }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const keys = await plugins.listKeyring();
    expect(keys).toHaveLength(1);
    await plugins.revokeKeyringEntry('acme', 'k1');

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/plugins/keyring', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/plugins/keyring?vendor=acme&keyId=k1', expect.objectContaining({
      method: 'DELETE',
    }));
  });
});
