import { cleanup, fireEvent, render, waitFor } from '@testing-library/preact';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ComponentChildren } from 'preact';
import { DatabaseView } from './Database';

vi.mock('../components/Layout', () => ({
  Page: ({ title, subtitle, children }: {
    title: string;
    subtitle?: string;
    children: ComponentChildren;
  }) => (
    <div data-testid="page">
      <h1>{title}</h1>
      {subtitle ? <p>{subtitle}</p> : null}
      {children}
    </div>
  ),
  Card: ({ title, actions, children }: {
    title: ComponentChildren;
    actions?: ComponentChildren;
    children: ComponentChildren;
  }) => (
    <section>
      <h2>{title}</h2>
      {actions}
      {children}
    </section>
  ),
}));

vi.mock('../lib/router', () => ({
  navigate: vi.fn(),
}));

describe('DatabaseView', () => {
  let table: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    table = vi.fn(async (name: string) => {
      if (name === 'audit_logs') {
        return {
          name,
          columns: ['id', 'message'],
          rows: [{ id: 1, message: 'audit row' }],
          limit: 50,
          offset: 0,
          count: 1,
        };
      }
      return {
        name,
        columns: ['id', 'name'],
        rows: [{ id: 2, name: 'cron row' }],
        limit: 50,
        offset: 0,
        count: 1,
      };
    });
    const endpoints = await import('../api/endpoints');
    (endpoints as unknown as {
      database: {
        tables: () => Promise<{ path: string; tables: string[] }>;
        table: typeof table;
      };
    }).database.tables = async () => ({ path: '/storage/db.sqlite', tables: ['audit_logs', 'cronjobs'] });
    (endpoints as unknown as { database: { table: typeof table } }).database.table = table;
  });

  afterEach(() => {
    cleanup();
  });

  it('clears stale rows while switching from audit_logs to cronjobs', async () => {
    const { rerender } = render(<DatabaseView table="audit_logs" />);

    await waitFor(() => expect(document.body.textContent).toContain('audit row'));

    rerender(<DatabaseView table="cronjobs" />);

    expect(document.body.textContent).not.toContain('audit row');
    expect(document.body.textContent).toContain('Loading cronjobs');
    await waitFor(() => expect(document.body.textContent).toContain('cron row'));
    expect(document.body.textContent).not.toContain('audit row');
  });

  it('handles null rows from API without crashing', async () => {
    table.mockResolvedValueOnce({
      name: 'audit_logs',
      columns: ['id', 'message'],
      rows: null,
      limit: 50,
      offset: 0,
      count: 0,
    });

    render(<DatabaseView table="audit_logs" />);

    await waitFor(() => expect(document.body.textContent).toContain('No rows'));
    expect(document.body.textContent).not.toContain('audit row');
  });

  it('navigates to the clicked table', async () => {
    const { navigate } = await import('../lib/router');
    render(<DatabaseView />);

    await waitFor(() => expect(document.body.textContent).toContain('cronjobs'));
    fireEvent.click(document.body.querySelectorAll('.file-row')[1]);

    expect(navigate).toHaveBeenCalledWith('/database', { table: 'cronjobs' });
  });
});
