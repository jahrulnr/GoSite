import { database } from '../api/endpoints';
import type { DatabaseTableData } from '../api/types';
import { AsyncView, EmptyState } from '../components/Ui';
import { Card, Page } from '../components/Layout';
import { useAsync } from '../lib/hooks';
import { navigate } from '../lib/router';
import { formatNumber } from '../lib/format';

function DataTable({ data }: Readonly<{ data: DatabaseTableData }>) {
  return (
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            {data.columns.map((col, i) => (
              <th key={col}>
                <span class="row" style="gap:6px;">
                  <span style="color:var(--text-faint);font-family:var(--mono);font-weight:500;font-size:10px;">{String(i).padStart(2, '0')}</span>
                  {col}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.rows.map((row, i) => (
            <tr key={i}>
              {data.columns.map((col, colIndex) => (
                <td key={col}>{String(Array.isArray(row) ? row[colIndex] ?? '' : row[col] ?? '')}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function DatabaseView({ table }: Readonly<{ table?: string }>) {
  const tables = useAsync(() => database.tables());
  const data = useAsync(() => (table ? database.table(table) : Promise.resolve(undefined)), [table]);

  return (
    <Page title="Database" subtitle="Browse tables in the panel database" eyebrow="storage">
      <div class="grid cols-2">
        <Card title="Tables">
          <AsyncView state={tables}>
            {(res) => (
              <div class="table-wrap">
                <table class="table">
                  <tbody>
                    {res.tables.map((name) => (
                      <tr key={name} class="file-row" onClick={() => navigate('/database', { table: name })}>
                        <td>
                          <span class="row" style="gap:8px;">
                            <span class="badge info" style="font-size:10px;padding:2px 7px;">tbl</span>
                            <span class="mono" style={name === table ? 'color:var(--accent);' : ''}>{name}</span>
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </AsyncView>
        </Card>
        <Card
          title={table ?? 'Rows'}
          actions={data.data ? <span class="dim mono" style="font-size:11px;">{formatNumber(data.data.count)} rows · {data.data.limit} shown</span> : undefined}
        >
          {!table ? (
            <EmptyState title="Select a table" hint="Pick one on the left to view rows." />
          ) : (
            <AsyncView state={data} loadingLabel={`Loading ${table}`}>
              {(res) => res ? <DataTable data={res} /> : <EmptyState title="No rows" />}
            </AsyncView>
          )}
        </Card>
      </div>
    </Page>
  );
}
