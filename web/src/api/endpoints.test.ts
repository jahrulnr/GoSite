import { describe, expect, it } from 'vitest';
import { observability } from './endpoints';

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
