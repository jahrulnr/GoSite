import { useEffect, useState } from 'preact/hooks';
import { metrics } from '../api/endpoints';
import { AsyncView, EmptyState } from '../components/Ui';
import { Card, Page, SimpleRows, Stat } from '../components/Layout';
import { Sparkline } from '../components/Sparkline';
import { formatBytes, formatNumber } from '../lib/format';
import { useAsync, useInterval } from '../lib/hooks';
import { useStore } from '../lib/store';

type MetricsTab = 'traffic' | 'nginx';

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

function TabSelect({ value, onChange, showNginx }: Readonly<{ value: MetricsTab; onChange: (value: MetricsTab) => void; showNginx: boolean }>) {
  return (
    <div class="row" style="gap:8px;">
      <button type="button" class={`btn ghost compact${value === 'traffic' ? ' active' : ''}`} onClick={() => onChange('traffic')}>Traffic</button>
      {showNginx && (
        <button type="button" class={`btn ghost compact${value === 'nginx' ? ' active' : ''}`} onClick={() => onChange('nginx')}>Nginx</button>
      )}
    </div>
  );
}

function TrafficPanel({ range }: Readonly<{ range: string }>) {
  const summary = useAsync(() => metrics.summary(range), [range]);
  const top = useAsync(() => metrics.topSites(range), [range]);
  const codes = useAsync(() => metrics.statusCodes(range), [range]);
  const series = useAsync(() => metrics.series(range), [range]);
  const requestSeries = Object.entries(series.data?.requests ?? {});
  const byteSeries = Object.entries(series.data?.bytes ?? {});

  return (
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
  );
}

function NginxPanel({ range }: Readonly<{ range: string }>) {
  const current = useAsync(() => metrics.nginxCurrent(), [range]);
  const series = useAsync(() => metrics.nginxSeries(range), [range]);
  const vts = useAsync(() => metrics.nginxVTSStatus(), []);
  const vtsServers = useAsync(() => (vts.data?.enabled ? metrics.nginxVTSServers() : Promise.resolve({ servers: [] })), [vts.data?.enabled]);
  const vtsUpstreams = useAsync(() => (vts.data?.enabled ? metrics.nginxVTSUpstreams() : Promise.resolve({ upstreams: [] })), [vts.data?.enabled]);
  useInterval(current.reload, 15000);

  return (
    <div class="grid">
      <AsyncView state={current}>
        {(data) => (
          data.available ? (
            <div class="col" style="gap:12px;">
              {data.counter_reset && (
                <div class="dim mono" style="font-size:11px;">Counter reset detected — nginx likely restarted; rates omitted for this sample.</div>
              )}
              <div class="grid cols-4">
                <Stat label="Active" value={formatNumber(data.active)} tone="info" sub="Connections now" />
                <Stat label="Reading" value={formatNumber(data.reading)} sub="Reading request" />
                <Stat label="Writing" value={formatNumber(data.writing)} sub="Writing response" />
                <Stat label="Waiting" value={formatNumber(data.waiting)} sub="Keep-alive idle" />
              </div>
              <div class="grid cols-4">
                <Stat label="Req/s" value={data.request_rate_per_sec != null ? data.request_rate_per_sec.toFixed(2) : '—'} tone="warn" sub="Δrequests / Δtime" />
                <Stat label="Accept/s" value={data.accept_rate_per_sec != null ? data.accept_rate_per_sec.toFixed(2) : '—'} sub="Δaccepts / Δtime" />
                <Stat label="Handled/s" value={data.handled_rate_per_sec != null ? data.handled_rate_per_sec.toFixed(2) : '—'} sub="Δhandled / Δtime" />
                <Stat label="Dropped" value={formatNumber(data.dropped_connections)} tone={data.dropped_connections > 0 ? 'danger' : undefined} sub="accepts − handled" />
              </div>
            </div>
          ) : (
            <EmptyState title="No nginx samples yet" hint="stub_status collector runs every 30 seconds when nginx is reachable on localhost." />
          )
        )}
      </AsyncView>
      <div class="grid cols-2">
        <Card title="Active connections">
          <AsyncView state={series}>
            {(data) => (
              (data.active?.length ?? 0) === 0 ? (
                <EmptyState title="No connection series" />
              ) : (
                <Sparkline label="active" points={data.active} />
              )
            )}
          </AsyncView>
        </Card>
        <Card title="Request rate (req/s)">
          <AsyncView state={current}>
            {(data) => data.available && (
              <div class="col" style="gap:8px;margin-bottom:12px;">
                <div class="mono" style="font-size:28px;color:var(--accent);">
                  {data.request_rate_per_sec != null ? data.request_rate_per_sec.toFixed(2) : '—'}
                </div>
                <div class="dim mono" style="font-size:11px;">from stub_status cumulative counter</div>
              </div>
            )}
          </AsyncView>
          <AsyncView state={series}>
            {(data) => (
              (data.request_rate?.length ?? 0) === 0 ? (
                <EmptyState title="No rate series" />
              ) : (
                <Sparkline label="req/s" points={data.request_rate} stroke="var(--warn)" />
              )
            )}
          </AsyncView>
        </Card>
      </div>
      <Card title="Connection states">
        <AsyncView state={series}>
          {(data) => (
            (data.reading?.length ?? 0) === 0 ? (
              <EmptyState title="No state series" />
            ) : (
              <div class="grid cols-3">
                <Sparkline label="reading" points={data.reading} stroke="var(--info)" />
                <Sparkline label="writing" points={data.writing} stroke="var(--accent)" />
                <Sparkline label="waiting" points={data.waiting} stroke="var(--text-dim)" />
              </div>
            )
          )}
        </AsyncView>
      </Card>
      <Card title="VTS server zones">
        <AsyncView state={vts}>
          {(status) => (
            !status.enabled ? (
              <EmptyState title="VTS not enabled" hint={status.hint ?? 'Requires custom nginx build with nginx-module-vts.'} />
            ) : (
              <AsyncView state={vtsServers}>
                {(data) => (
                  data.servers.length === 0 ? (
                    <EmptyState title="No VTS server samples" hint="Enable vhost_traffic_status on site vhosts and wait for collector ticks." />
                  ) : (
                    <SimpleRows rows={data.servers.map((row) => [
                      <span class="mono">{row.server_name}</span>,
                      formatNumber(row.requests),
                      formatBytes(row.out_bytes),
                      `${row.request_msec.toFixed(1)} ms`,
                    ])} />
                  )
                )}
              </AsyncView>
            )
          )}
        </AsyncView>
      </Card>
      <Card title="VTS upstream peers">
        <AsyncView state={vts}>
          {(status) => (
            !status.enabled ? (
              <EmptyState title="VTS not enabled" hint={status.hint} />
            ) : (
              <AsyncView state={vtsUpstreams}>
                {(data) => (
                  data.upstreams.length === 0 ? (
                    <EmptyState title="No upstream peers" hint="Named upstream blocks appear here once proxy traffic flows." />
                  ) : (
                    <SimpleRows rows={data.upstreams.map((row) => [
                      <span class="mono">{row.upstream_name}</span>,
                      <span class="mono dim">{row.server_addr}</span>,
                      formatNumber(row.requests),
                      `${row.response_msec.toFixed(1)} ms`,
                      row.down ? 'down' : 'up',
                    ])} />
                  )
                )}
              </AsyncView>
            )
          )}
        </AsyncView>
      </Card>
    </div>
  );
}

export function MetricsView() {
  const { meta } = useStore();
  const [range, setRange] = useState(meta?.traffic?.ranges?.[0]?.value ?? '1h');
  const showNginx = meta?.nginx?.stub_status?.enabled ?? false;
  const [tab, setTab] = useState<MetricsTab>('traffic');

  useEffect(() => {
    if (!meta?.traffic?.ranges?.some((item) => item.value === range) && meta?.traffic?.ranges?.[0]) {
      setRange(meta.traffic.ranges[0].value);
    }
  }, [meta, range]);

  useEffect(() => {
    if (tab === 'nginx' && !showNginx) setTab('traffic');
  }, [tab, showNginx]);

  return (
    <Page
      title="Traffic"
      subtitle={tab === 'nginx' ? 'Real-time nginx stub_status on the same host' : 'Traffic for the selected time range (may differ from the dashboard snapshot)'}
      eyebrow="metrics"
      actions={(
        <div class="row" style="gap:10px;">
          <TabSelect value={tab} onChange={setTab} showNginx={showNginx} />
          <RangeSelect value={range} onChange={setRange} />
        </div>
      )}
    >
      {tab === 'nginx' ? <NginxPanel range={range} /> : <TrafficPanel range={range} />}
    </Page>
  );
}
