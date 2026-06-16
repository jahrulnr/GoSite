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
});
