import { useEffect, useState } from 'preact/hooks';
import { metrics } from '../api/endpoints';
import { AsyncView, EmptyState } from '../components/Ui';
import { Card, Page, SimpleRows, Stat } from '../components/Layout';
import { Sparkline } from '../components/Sparkline';
import { formatBytes, formatNumber } from '../lib/format';
import { useAsync } from '../lib/hooks';
import { useStore } from '../lib/store';

function RangeSelect({ value, onChange }: Readonly<{ value: string; onChange: (value: string) => void }>) {
  const { meta } = useStore();
  return (
    <select class="select compact" value={value} onChange={(e) => onChange((e.target as HTMLSelectElement).value)}>
      {(meta?.traffic?.ranges ?? [{ value, label: value }]).map((item) => (
        <option key={item.value} value={item.value}>{item.label}</option>
      ))}
    </select>
  );
}

export function MetricsView() {
  const { meta } = useStore();
  const [range, setRange] = useState(meta?.traffic?.ranges?.[0]?.value ?? '1h');
  const summary = useAsync(() => metrics.summary(range), [range]);
  const top = useAsync(() => metrics.topSites(range), [range]);
  const codes = useAsync(() => metrics.statusCodes(range), [range]);
  const series = useAsync(() => metrics.series(range), [range]);

  useEffect(() => {
    if (!meta?.traffic?.ranges?.some((item) => item.value === range) && meta?.traffic?.ranges?.[0]) {
      setRange(meta.traffic.ranges[0].value);
    }
  }, [meta, range]);

  const requestSeries = Object.entries(series.data?.requests ?? {});
  const byteSeries = Object.entries(series.data?.bytes ?? {});

  return (
    <Page
      title="Traffic"
      subtitle="Traffic for the selected time range (may differ from the dashboard snapshot)"
      eyebrow="metrics"
      actions={<RangeSelect value={range} onChange={setRange} />}
    >
      <div class="grid">
        <AsyncView state={summary}>
          {(data) => (
            <div class="grid cols-3">
              <Stat label="Range" value={data.range} sub="Time window" />
              <Stat label="Requests" value={formatNumber(data.requests)} tone="info" sub="Total hits" />
              <Stat label="Bytes" value={formatBytes(data.bytes)} sub="Total transfer" />
            </div>
          )}
        </AsyncView>
        <div class="grid cols-2">
          <Card title="Top sites">
            <AsyncView state={top}>
              {(data) => {
                const rows = Array.isArray(data) ? data : data.sites;
                if (rows.length === 0) return <EmptyState title="No traffic in this period" hint="Try a longer range or check back after site activity." />;
                return <SimpleRows rows={rows.map((row) => [<span class="mono">{row.site}</span>, formatNumber(row.requests), formatBytes(row.bytes)])} />;
              }}
            </AsyncView>
          </Card>
          <Card title="Status codes">
            <AsyncView state={codes}>
              {(data) => {
                const labelMap: Record<string, string> = { s2xx: '2xx Success', s3xx: '3xx Redirect', s4xx: '4xx Client error', s5xx: '5xx Server error' };
                return (
                  <div class="col" style="gap:8px;">
                    {Object.entries(data).map(([k, v]) => {
                      const total = Object.values(data).reduce((sum, n) => sum + (typeof n === 'number' ? n : 0), 0) || 1;
                      const pct = ((Number(v) / total) * 100);
                      const tone = k === 's5xx' ? 'danger' : k === 's4xx' ? 'warn' : k === 's3xx' ? 'info' : 'ok';
                      return (
                        <div key={k} style="display:flex;flex-direction:column;gap:6px;padding:10px 12px;border:1px solid var(--border-soft);border-radius:8px;background:var(--inset);">
                          <div class="row" style="justify-content:space-between;">
                            <span class="mono" style="font-size:12px;">{labelMap[k] ?? k}</span>
                            <span class="mono" style={`color:var(--${tone === 'ok' ? 'accent' : tone});`}>{formatNumber(Number(v))}</span>
                          </div>
                          <div style="height:4px;background:oklch(0% 0 0 / 0.3);border-radius:99px;overflow:hidden;">
                            <span style={`display:block;width:${pct}%;height:100%;background:var(--${tone === 'ok' ? 'accent' : tone});transition:width var(--d-slow) var(--ease-out);`} />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                );
              }}
            </AsyncView>
          </Card>
        </div>
        <Card title="Request series">
          <AsyncView state={series}>
            {() => (
              requestSeries.length === 0 ? (
                <EmptyState title="No request series" hint="No per-site request data for this time range." />
              ) : (
                <div class="grid cols-2">
                  {requestSeries.map(([site, points]) => (
                    <Sparkline key={site} label={site} points={points} />
                  ))}
                </div>
              )
            )}
          </AsyncView>
        </Card>
        <Card title="Bytes series">
          <AsyncView state={series}>
            {() => (
              byteSeries.length === 0 ? (
                <EmptyState title="No bytes series" hint="No per-site bandwidth data for this time range." />
              ) : (
                <div class="grid cols-2">
                  {byteSeries.map(([site, points]) => (
                    <Sparkline key={site} label={site} points={points} stroke="var(--info)" />
                  ))}
                </div>
              )
            )}
          </AsyncView>
        </Card>
      </div>
    </Page>
  );
}
