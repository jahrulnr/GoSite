// One typed function per OpenAPI operation. Views call these — never fetch directly.
import { http, rootRequest, API_BASE } from './client';
import type {
  AuthMetadata,
  Cronjob,
  DashboardResponse,
  DatabaseTableData,
  DatabaseTablesResponse,
  DockerContainer,
  FileEntry,
  JobAcceptedResponse,
  LockscreenResponse,
  LoginResponse,
  LogSite,
  LogTailResponse,
  Mount,
  NetworkTraffic,
  QueryEvent,
  QueryMetaResponse,
  QueryResponse,
  SavedQuery,
  SavedQueryUpdateRequest,
  SslStatus,
  StatusCodesResponse,
  SystemInfo,
  TopSiteEntry,
  TrafficMetricsSummary,
  TrafficSeriesResponse,
  UiMetaResponse,
  User,
  Website,
  WebsiteCreateRequest,
  WebsiteToggleResponse,
  WebsiteValidateResponse,
} from './types';

// ---- Auth ----
export const auth = {
  metadata: () => http.get<AuthMetadata>('/auth/login'),
  login: (email: string, password: string, remember = false) =>
    http.post<LoginResponse>('/auth/login', { email, password, remember }),
  logout: () => http.post<void>('/auth/logout'),
  me: () => http.get<{ user: User } | User>('/auth/me'),
  lockscreen: () => http.get<LockscreenResponse>('/auth/lockscreen'),
  lock: () => http.post<void>('/auth/lock'),
  unlock: (password: string) => http.post<LoginResponse>('/auth/unlock', { password }),
};

// ---- UI meta ----
export const ui = {
  meta: () => http.get<UiMetaResponse>('/ui/meta'),
};

// ---- Dashboard / System ----
export const dashboard = {
  get: () => http.get<DashboardResponse>('/dashboard'),
};

export const system = {
  info: () => http.get<SystemInfo>('/system/info'),
  network: () => http.get<NetworkTraffic>('/system/network'),
  diskIO: () => http.get<{ read?: string; write?: string }>('/system/disk-io'),
  nginxTraffic: () => http.get<Record<string, unknown>>('/system/nginx-traffic'),
};

// ---- Settings ----
export const settings = {
  updateProfile: (payload: { name?: string; email?: string; password?: string }) =>
    http.put<{ user: User }>('/settings/profile', payload),
};

// ---- Websites ----
export const websites = {
  list: () => http.get<{ websites: Website[] }>('/websites').then((r) => r.websites ?? []),
  get: (id: number) => http.get<Website>(`/websites/${id}`),
  create: (body: WebsiteCreateRequest) => http.post<Website>('/websites', body),
  update: (id: number, body: Partial<WebsiteCreateRequest>) =>
    http.put<{ message: string }>(`/websites/${id}`, body),
  remove: (id: number, clean = false) =>
    http.del<{ message: string }>(`/websites/${id}`, { clean }),
  toggle: (id: number) => http.patch<WebsiteToggleResponse>(`/websites/${id}/toggle`),
  validate: (domain: string, path: string) =>
    http.post<WebsiteValidateResponse>('/websites/validate', { domain, path }),
  nginxConfig: (id: number) =>
    http.get<{ config: string }>(`/websites/${id}/nginx-config`).then((r) => r.config),
  updateNginxConfig: (id: number, config: string) =>
    http.put<{ message: string }>(`/websites/${id}/nginx-config`, { config }),
};

// ---- SSL ----
export const ssl = {
  status: (id: number) => http.get<SslStatus>(`/websites/${id}/ssl`),
  uploadManual: (id: number, pub: string, priv: string) =>
    http.put<{ message: string }>(`/websites/${id}/ssl/manual`, { public: pub, private: priv }),
  startCertbot: (id: number) => http.post<JobAcceptedResponse>(`/websites/${id}/ssl/certbot`),
  certbotStreamUrl: (id: number, jobId: number) =>
    `${API_BASE}/websites/${id}/ssl/certbot/stream?job_id=${jobId}`,
};

// ---- Nginx ----
export const nginx = {
  getDefault: () => http.get<{ config: string }>('/nginx/default').then((r) => r.config),
  updateDefault: (config: string) =>
    http.put<{ message: string }>('/nginx/default', { config }),
  getGlobal: () => http.get<{ config: string }>('/nginx/global').then((r) => r.config),
  updateGlobal: (content: string) =>
    http.put<{ message: string }>('/nginx/global', { content }),
  test: (config?: string) => http.post<{ ok: boolean; output?: string }>('/nginx/test', { config }),
  reload: () => http.post<{ message: string }>('/nginx/reload'),
};

// ---- Docker ----
export const docker = {
  list: () =>
    http.get<{ containers: DockerContainer[] }>('/docker/containers').then((r) => r.containers ?? []),
  restart: (id: string) => http.post<{ message: string }>(`/docker/containers/${id}/restart`),
  stop: (id: string) => http.post<{ message: string }>(`/docker/containers/${id}/stop`),
  logs: (id: string) =>
    http.get<{ lines: string[] }>(`/docker/containers/${id}/logs`).then((r) => r.lines ?? []),
};

// ---- Files ----
export const files = {
  browse: (path: string) =>
    http.get<{ entries: FileEntry[] }>('/files', { path }).then((r) => r.entries ?? []),
  read: (path: string) =>
    http.get<{ content: string }>('/files/content', { path }).then((r) => r.content),
  createFile: (path: string, name: string, content = '') =>
    http.post<{ message: string }>('/files', { type: 'file', path, name, content }),
  createFolder: (path: string, name: string) =>
    http.post<{ message: string }>('/files', { type: 'directory', path, name }),
  upload: (path: string, file: File) => {
    const form = new FormData();
    form.append('path', path);
    form.append('file', file);
    return http.upload<{ message: string }>('/files', form);
  },
  remove: (path: string) => http.del<{ message: string }>('/files', { path }),
  action: (action: string, path: string, extra: Record<string, unknown> = {}) =>
    http.post<{ message: string }>('/files/actions', { action, path, ...extra }),
};

// ---- Mounts ----
export const mounts = {
  list: () => http.get<{ mounts: Mount[] }>('/mounts').then((r) => r.mounts ?? []),
  create: (body: { device: string; dir: string; type: string; options?: string; dump?: string; fsck?: string }) =>
    http.post<{ message: string }>('/mounts', body),
  update: (oldDevice: string, oldDir: string, entry: Mount) =>
    http.put<{ message: string }>('/mounts', { old_device: oldDevice, old_dir: oldDir, entry }),
  remove: (device: string, dir: string) => http.del<{ message: string }>('/mounts', { device, dir }),
  enable: (device: string, dir: string) =>
    http.post<{ message: string }>('/mounts/enable', { device, dir }),
};

// ---- Cron ----
export const cron = {
  list: () => http.get<{ cronjobs: Cronjob[] }>('/cronjobs').then((r) => r.cronjobs ?? []),
  create: (body: { name: string; payload: string; run_every: string }) =>
    http.post<Cronjob>('/cronjobs', body),
  update: (id: number, body: Partial<{ name: string; payload: string; run_every: string }>) =>
    http.put<Cronjob>(`/cronjobs/${id}`, body),
  remove: (id: number) => http.del<{ message: string }>(`/cronjobs/${id}`),
  run: (id: number) => http.post<JobAcceptedResponse>(`/cronjobs/${id}/run`),
  runStreamUrl: (id: number, jobId: number) =>
    `${API_BASE}/cronjobs/${id}/run/stream?job_id=${jobId}`,
};

// ---- Logs ----
export const logs = {
  sites: () => http.get<{ sites: LogSite[] }>('/logs/sites').then((r) => r.sites ?? []),
  tail: (domain: string, type: string, tail = 200) =>
    http.get<LogTailResponse>('/logs', { domain, type, tail }),
};

// ---- Database ----
export const database = {
  tables: () => http.get<DatabaseTablesResponse>('/database/tables'),
  table: (name: string, limit = 50, offset = 0) =>
    http.get<DatabaseTableData>(`/database/tables/${encodeURIComponent(name)}`, { limit, offset }),
};

export interface QueryPayload {
  source: string;
  q: string;
  site?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export interface QueryStreamFrame {
  type: 'ingesting' | 'meta' | 'event' | 'done' | 'error';
  hits?: number;
  event?: QueryEvent;
  error?: { code?: string; message?: string };
}

function queryUrl(path: string, params: QueryPayload & { stream?: string }) {
  const qs = new URLSearchParams();
  qs.set('source', params.source);
  if (params.q) qs.set('q', params.q);
  if (params.site) qs.set('site', params.site);
  if (params.from) qs.set('from', params.from);
  if (params.to) qs.set('to', params.to);
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.offset) qs.set('offset', String(params.offset));
  if (params.stream) qs.set('stream', params.stream);
  return `${API_BASE}${path}?${qs.toString()}`;
}

// ---- Observability (Splunk Lite) ----
export const observability = {
  queryMeta: () => http.get<QueryMetaResponse>('/query/meta'),
  query: (payload: QueryPayload, signal?: AbortSignal) => {
    const { source, q, site, from, to, limit, offset } = payload;
    return http.get<QueryResponse>('/query', { source, q, site, from, to, limit, offset }, signal);
  },
  queryPost: (payload: QueryPayload) => http.post<QueryResponse>('/query', payload),
  queryStreamUrl: (payload: QueryPayload, mode: 'sse' | 'ndjson' = 'sse') => queryUrl('/query', { ...payload, stream: mode }),
  tailUrl: (params: { source: string; site?: string; from?: string; to?: string }) => {
    const qs = new URLSearchParams();
    qs.set('source', params.source);
    if (params.site) qs.set('site', params.site);
    if (params.from) qs.set('from', params.from);
    if (params.to) qs.set('to', params.to);
    return `${API_BASE}/query/tail?${qs.toString()}`;
  },
  /**
   * Open a Server-Sent Events stream to /query/tail. Each `data:` line is a
   * JSON-encoded `QueryEvent`. EventSource does not support custom headers, so
   * the cookie session auth is used (withCredentials: true).
   *
   * The browser's EventSource auto-reconnects on transient network errors and
   * fires `onerror` while reconnecting. We only treat the stream as dead when
   * the readyState is `CLOSED` (server sent no `retry:` hint and gave up, or
   * we explicitly called `es.close()`). Mid-reconnect errors must NOT stop
   * the tail — otherwise the UI shows the Stop button flipping back to Run
   * while events are still flowing.
   * Returns a `stop()` function that closes the stream.
   */
  startTail: (url: string, onEvent: (e: QueryEvent) => void, onClosed?: () => void) => {
    const es = new EventSource(url, { withCredentials: true });
    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as QueryEvent;
        onEvent(data);
      } catch {
        // ignore malformed payload; backend will close the stream if it errors
      }
    };
    es.onerror = () => {
      // EventSource constants: 0 = CONNECTING (auto-reconnect in flight),
      // 2 = CLOSED (gave up). Only the latter is terminal.
      if (es.readyState === EventSource.CLOSED) {
        onClosed?.();
        es.close();
      }
      // Otherwise the browser is reconnecting; keep the tail alive.
    };
    return () => es.close();
  },
  startQueryStream: (payload: QueryPayload, onFrame: (frame: QueryStreamFrame) => void, onError?: (e: Event) => void) => {
    const es = new EventSource(observability.queryStreamUrl(payload, 'sse'), { withCredentials: true });
    es.onmessage = (event) => {
      try {
        onFrame(JSON.parse(event.data) as QueryStreamFrame);
      } catch {
        // ignore malformed frame
      }
    };
    es.addEventListener('error', (event) => {
      try {
        onFrame(JSON.parse((event as MessageEvent).data) as QueryStreamFrame);
      } catch {
        onError?.(event);
      }
    });
    return () => es.close();
  },
  savedQueries: () =>
    http.get<{ queries: SavedQuery[] }>('/query/saved').then((r) => r.queries ?? []),
  saveQuery: (name: string, source: string, q: string) =>
    http.post<SavedQuery>('/query/saved', { name, source, q }),
  updateSavedQuery: (id: number, body: SavedQueryUpdateRequest) =>
    http.patch<SavedQuery>(`/query/saved/${id}`, body),
  deleteSavedQuery: (id: number) => http.del<void>(`/query/saved/${id}`),
};

// ---- Metrics (Grafana Lite) ----
export const metrics = {
  series: (range: string) =>
    http.get<TrafficSeriesResponse>('/metrics/traffic/series', { range }),
  topSites: (range: string) =>
    http.get<{ sites: TopSiteEntry[] } | TopSiteEntry[]>('/metrics/traffic/top-sites', { range }),
  statusCodes: (range: string) =>
    http.get<StatusCodesResponse>('/metrics/traffic/status-codes', { range }),
  summary: (range: string) =>
    http.get<TrafficMetricsSummary>('/metrics/traffic/summary', { range }),
};

export const rootHealth = (signal?: AbortSignal) => rootRequest<unknown>('/health', signal);
export const health = rootHealth;
