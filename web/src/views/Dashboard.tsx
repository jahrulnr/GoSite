import { dashboard, system } from '../api/endpoints';
import type { TrafficSite } from '../api/types';
import { AsyncView, Badge, EmptyState } from '../components/Ui';
import { Card, Page, Stat, siteTraffic } from '../components/Layout';
import { formatBytes, formatDate, formatKiB, formatNumber, formatPercent, formatDiskSectors, formatRate } from '../lib/format';
import { useAsync, useInterval } from '../lib/hooks';

function MetricTile({
  label,
  value,
  sub,
  percent,
  tone,
}: Readonly<{
  label: string;
  value: string;
  sub?: string;
  percent?: number;
  tone?: 'default' | 'warn' | 'danger' | 'info';
}>) {
  return (
    <div class={`stat${tone && tone !== 'default' ? ` ${tone}` : ''}`}>
      <div class="label">{label}</div>
      <div class="value">{value}</div>
      {sub && <div class="sub">{sub}</div>}
      {percent !== undefined && (
        <div class={`meter ${tone && tone !== 'default' ? tone : ''}`}>
          <span style={{ width: `${Math.max(0, Math.min(100, percent))}%` }} />
        </div>
      )}
    </div>
  );
}

export function DashboardView() {
  const state = useAsync(() => dashboard.get());
  const network = useAsync(() => system.network());
  const diskIO = useAsync(() => system.diskIO());
  useInterval(state.reload, 5000);

  return (
    <Page title="Dashboard" subtitle="Fresh backend snapshot, refreshed every 15 seconds" eyebrow="mission">
      <AsyncView state={state}>
        {(data) => {
          const memory = data.system.memory?.[0];
          const storage = data.system.storage;
          const memoryPercent = memory?.total ? (memory.used / memory.total) * 100 : undefined;
          const storagePercent = storage?.size ? (storage.used / storage.size) * 100 : undefined;
          const traffic = data.traffic_summary as { total?: TrafficSite; sites?: Record<string, TrafficSite>; requests?: number; bytes?: number };
          const netIn = Object.values(network.data?.in ?? {}).reduce((sum, v) => sum + v, 0);
          const netOut = Object.values(network.data?.out ?? {}).reduce((sum, v) => sum + v, 0);
          const cpuTone = (data.system.cpu ?? 0) > 80 ? 'danger' : (data.system.cpu ?? 0) > 60 ? 'warn' : 'default';
          const memTone = (memoryPercent ?? 0) > 85 ? 'danger' : (memoryPercent ?? 0) > 65 ? 'warn' : 'default';
          const diskTone = (storagePercent ?? 0) > 90 ? 'danger' : (storagePercent ?? 0) > 70 ? 'warn' : 'default';
          const ranked = siteTraffic(traffic);

          return (
            <div class="grid">
              <div class="grid cols-4">
                <MetricTile label="CPU" value={formatPercent(data.system.cpu)} sub="Current system load" percent={data.system.cpu} tone={cpuTone} />
                <MetricTile label="Memory" value={memory ? formatKiB(memory.used) : '—'} sub={memory ? `${formatKiB(memory.free)} free` : 'No sample'} percent={memoryPercent} tone={memTone} />
                <MetricTile label="Storage" value={storage ? formatKiB(storage.used) : '—'} sub={storage ? `${formatKiB(storage.available)} available` : 'No disk sample'} percent={storagePercent} tone={diskTone} />
                <MetricTile label="Requests · 1h" value={formatNumber(traffic.total?.requests ?? traffic.requests)} sub={formatBytes(traffic.total?.bytes ?? traffic.bytes)} />
              </div>

              {data.nginx_status?.available && (
                <div class="grid cols-4">
                  <MetricTile label="Nginx active" value={formatNumber(data.nginx_status.active)} sub={`${formatRate(data.nginx_status.request_rate_per_sec)} req/s`} tone="info" />
                  <MetricTile label="Reading" value={formatNumber(data.nginx_status.reading)} sub="Receiving request data" />
                  <MetricTile label="Writing" value={formatNumber(data.nginx_status.writing)} sub="Sending responses" />
                  <MetricTile label="Waiting" value={formatNumber(data.nginx_status.waiting)} sub="Idle connections" />
                  {data.nginx_status.dropped_connections > 0 && (
                    <MetricTile label="Dropped" value={formatNumber(data.nginx_status.dropped_connections)} sub="Failed to handle" tone="danger" />
                  )}
                </div>
              )}

              <div class="grid cols-3">
                <Card title="Network I/O">
                  <div class="col">
                    <KeyRow label="Inbound" value={formatBytes(netIn)} accent />
                    <KeyRow label="Outbound" value={formatBytes(netOut)} />
                    <KeyRow label="Interfaces" value={String(Object.keys(network.data?.in ?? {}).length || 0)} muted />
                  </div>
                </Card>
                <Card title="Disk I/O">
                  <div class="col">
                    <KeyRow label="Read" value={formatDiskSectors(diskIO.data?.read)} accent />
                    <KeyRow label="Write" value={formatDiskSectors(diskIO.data?.write)} />
                    <KeyRow label="Block size" value="512 bytes" muted />
                  </div>
                </Card>
                <Card
                  title="SSL watch"
                  actions={
                    <Badge kind={data.ssl_expiring.length > 0 ? 'warn' : 'ok'}>
                      {data.ssl_expiring.length > 0 ? `${data.ssl_expiring.length} expiring` : 'all clear'}
                    </Badge>
                  }
                >
                  {data.ssl_expiring.length === 0 ? (
                    <EmptyState title="No expiring certificates" />
                  ) : (
                    <div class="col" style="gap:6px;">
                      {data.ssl_expiring.slice(0, 4).map((cert) => (
                        <div key={cert.website_id} class="row wrap" style="justify-content:space-between;padding:8px 10px;border:1px solid var(--border-soft);border-radius:8px;background:var(--inset);">
                          <div style="min-width:0;">
                            <div style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">{cert.domain}</div>
                            <div class="dim mono" style="font-size:11px;">{formatDate(cert.expires_at)}</div>
                          </div>
                          <Badge kind={cert.expired ? 'danger' : 'warn'}>{cert.days_left}d</Badge>
                        </div>
                      ))}
                    </div>
                  )}
                </Card>
              </div>

              <div class="grid cols-2">
                <Card title="Top sites" actions={<span class="dim mono" style="font-size:11px;">last 1h</span>}>
                  {ranked.length === 0 ? (
                    (traffic.total?.requests ?? traffic.requests ?? 0) > 0 ? (
                      <EmptyState
                        title="No per-site breakdown yet"
                        hint="Total requests are counted. Site rows appear once traffic is attributed to domains."
                      />
                    ) : (
                      <EmptyState title="No traffic yet" hint="Website visits will show here after nginx logs are collected." />
                    )
                  ) : (
                    <div class="card-body flush">
                      <div class="table-wrap">
                        <table class="table">
                          <tbody>
                            {ranked.slice(0, 6).map(([domain, row], i) => {
                              const max = ranked[0]?.[1]?.requests || 1;
                              const pct = (row.requests / max) * 100;
                              return (
                                <tr key={domain}>
                                  <td style="width:24px;color:var(--text-faint);font-family:var(--mono);font-size:11px;">{String(i + 1).padStart(2, '0')}</td>
                                  <td>
                                    <div class="mono">{domain}</div>
                                    <div style="margin-top:6px;height:4px;background:var(--inset);border-radius:99px;overflow:hidden;">
                                      <span style={`display:block;width:${pct}%;height:100%;background:linear-gradient(90deg,var(--accent),var(--cobalt-400));`} />
                                    </div>
                                  </td>
                                  <td class="right nowrap">
                                    <div>{formatNumber(row.requests)}</div>
                                    <div class="dim mono" style="font-size:11px;">{formatBytes(row.bytes)}</div>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )}
                </Card>
                <Card title="Recent audit" actions={<span class="dim mono" style="font-size:11px;">{data.recent_audit.length} events</span>}>
                  {data.recent_audit.length === 0 ? (
                    <EmptyState title="No audit events" />
                  ) : (
                    <div class="col" style="gap:6px;max-height:280px;overflow-y:auto;">
                      {data.recent_audit.slice(0, 8).map((event, i) => (
                        <div key={i} class="row" style="align-items:flex-start;padding:8px 10px;border:1px solid var(--border-soft);border-radius:8px;background:var(--inset);">
                          <span class="badge info" style="font-size:10px;padding:2px 7px;flex:0 0 auto;">{event.source}</span>
                          <div style="min-width:0;flex:1;">
                            <div class="mono" style="font-size:11.5px;color:var(--text-dim);">{formatDate(event.ts)}</div>
                            <div style="font-size:13px;word-break:break-word;">{event.message}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </Card>
              </div>
            </div>
          );
        }}
      </AsyncView>
    </Page>
  );
}

function KeyRow({ label, value, accent, muted }: Readonly<{ label: string; value: string; accent?: boolean; muted?: boolean }>) {
  return (
    <div class="row" style="justify-content:space-between;padding:8px 10px;border:1px solid var(--border-soft);border-radius:8px;background:var(--inset);">
      <span class="dim mono" style="font-size:11px;text-transform:uppercase;letter-spacing:0.06em;">{label}</span>
      <span class="mono" style={`font-size:13px;color:${accent ? 'var(--accent)' : muted ? 'var(--text-dim)' : 'var(--text)'};`}>{value}</span>
    </div>
  );
}

// Re-export Stat for backward compatibility (other views may import it).
export { Stat };
