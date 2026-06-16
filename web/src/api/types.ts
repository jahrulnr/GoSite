// Types mirror the GoSite OpenAPI contract (api/openapi.yaml) and handler envelopes.
// Everything here is data-shape only; no values are hardcoded for the UI.

export interface ApiErrorBody {
  error: { code: string; message: string };
}

export interface User {
  id: number;
  name: string;
  email: string;
}

export interface LoginResponse {
  token?: string;
  user: User;
}

export interface AuthMetadata {
  lockscreen_enabled: boolean;
  basic_auth_enabled: boolean;
  lock_after_seconds: number;
  web_root?: string;
  file_roots?: Array<{ path: string; label?: string }>;
}

export interface LockscreenResponse {
  locked: boolean;
  user?: User;
}

// ---- System / Dashboard ----

export interface MemoryStat {
  label: string;
  total: number;
  used: number;
  free: number;
}

export interface StorageStat {
  system: string;
  size: number;
  used: number;
  available: number;
}

export interface SystemInfo {
  cpu?: number;
  memory?: MemoryStat[];
  storage?: StorageStat;
}

export interface TrafficSite {
  requests: number;
  bytes: number;
}

export interface TrafficSummary {
  sites: Record<string, TrafficSite>;
  total: TrafficSite;
}

export interface ExpiringCert {
  website_id: number;
  domain: string;
  expires_at: string;
  days_left: number;
  expired: boolean;
}

export interface AuditEvent {
  ts: string;
  source: string;
  action: string;
  user: string;
  message: string;
  meta: Record<string, unknown>;
}

export interface DashboardResponse {
  system: SystemInfo;
  traffic_summary: TrafficSummary;
  ssl_expiring: ExpiringCert[];
  recent_audit: AuditEvent[];
}

export interface NetworkTraffic {
  in?: Record<string, number>;
  out?: Record<string, number>;
  [k: string]: unknown;
}

// ---- Websites ----

export interface Website {
  id: number;
  name: string;
  domain: string;
  path: string;
  type: string;
  upstream?: string;
  ssl: boolean;
  active: boolean;
  config?: string;
}

export interface WebsiteCreateRequest {
  name: string;
  domain: string;
  path: string;
  type: string;
  upstream?: string;
  active?: boolean;
}

export interface WebsiteValidateResponse {
  valid: boolean;
  reason?: string;
  message?: string;
  [k: string]: unknown;
}

export interface WebsiteToggleResponse {
  id: number;
  active: boolean;
  message: string;
}

// ---- SSL ----

export interface SslStatus {
  enabled: boolean;
  expired: boolean;
  expires_at?: string;
  issuer?: string;
  [k: string]: unknown;
}

// ---- Docker ----

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  status: string;
}

// ---- Files ----

export interface FileEntry {
  name: string;
  path: string;
  size: number;
  mode: string;
  is_dir: boolean;
  mod_time: string;
  kind: string;
  mime_type: string;
  extension: string;
  editable: boolean;
  viewable: boolean;
  archive: boolean;
  symlink: boolean;
  target?: string;
}

export interface FileTools {
  unzip: boolean;
  tar: boolean;
  gzip: boolean;
}

export interface FileListResponse {
  entries: FileEntry[];
  tools: FileTools;
}

export interface FileContentResponse {
  content: string;
  entry: FileEntry;
  encoding: string;
}

// ---- Mounts ----

export interface MountS3Config {
  bucket?: string;
  endpoint?: string;
  region?: string;
  access_key?: string;
  secret_key?: string;
  path_style?: boolean;
}

export interface Mount {
  device: string;
  dir: string;
  type: string;
  options: string;
  dump: string;
  fsck: string;
  mounted: boolean;
  s3?: MountS3Config;
}

// ---- Cron ----

export interface Cronjob {
  id: number;
  name: string;
  payload: string;
  run_every: string;
  executed_at?: string;
}

export interface JobAcceptedResponse {
  job_id: number;
  message: string;
}

// ---- Plugins ----

export type PluginState =
  | 'installing'
  | 'installed'
  | 'install_failed'
  | 'enabling'
  | 'enabled'
  | 'enable_failed'
  | 'disabling'
  | 'uninstalling'
  | 'uninstalled';

export type PluginFailureClass =
  | 'validate_timeout'
  | 'start_failed'
  | 'hook_refresh_failed'
  | 'db_failed'
  | 'compensation_failed'
  | 'stop_failed'
  | 'fs_delete_failed'
  | 'config_migration_failed'
  | 'unknown'
  | '';

export interface PluginCapabilities {
  hooks?: string[];
  hookIsolation?: 'sequential' | 'independent' | string;
  uiSidebar?: boolean;
  configSchema?: boolean;
  loggingSink?: boolean;
  rulesAndRoles?: string;
  [k: string]: unknown;
}

export interface PluginSidebarEntry {
  label?: string;
  route?: string;
}

export interface PluginUIContribution {
  sidebar?: PluginSidebarEntry[];
  configSchema?: Record<string, unknown>;
  [k: string]: unknown;
}

export interface PluginManifest {
  id?: string;
  name?: string;
  version?: string;
  tier?: number;
  apiVersion?: string;
  minGoSiteVersion?: string;
  rpcVersion?: string;
  configVersion?: string;
  capabilities?: PluginCapabilities;
  permissions?: string[];
  entrypoints?: Record<string, { type?: string; command?: string }>;
  artifact?: { sha256?: string };
  signatures?: Array<{ keyId?: string; sig?: string }>;
  ui?: PluginUIContribution;
  [k: string]: unknown;
}

export interface PluginInstallSource {
  type: 'url' | 'github-release';
  url?: string;
  sha256?: string;
  repo?: string;
  tag?: string;
}

export interface PluginResolvePreview {
  plugin_id: string;
  version: string;
  tier: number;
  signed: boolean;
  keyId?: string;
  sha256: string;
  size: number;
  url: string;
  minGoSiteVersion?: string;
  source_type: string;
  source_ref: string;
  install_path: string;
  sourceCommit?: string;
  sourceRepository?: string;
  permissions: string[];
  hooks: string[];
  resolveToken?: string;
  resolveExpiresAt?: string;
}

export interface PluginInstallSettings {
  remote_install_enabled: boolean;
  trust_mode: 'strict' | 'community' | 'dev';
  allowed_hosts: string[];
  allow_unsigned: boolean;
  github_token_configured: boolean;
  gitlab_token_configured: boolean;
}

export interface PluginKeyringEntry {
  vendor: string;
  keyId: string;
  publicKey: string;
  createdAt?: string;
  revokedAt?: string;
}

export interface PluginInstallLogStep {
  step: string;
  at: string;
  status: 'ok' | 'failed';
  failure_class?: string;
  detail?: string;
}

export interface PluginVersion {
  id: number;
  plugin_id: string;
  version: string;
  name: string;
  tier: number;
  api_version: string;
  min_gosite_version?: string;
  rpc_version?: string;
  config_version?: string;
  artifact_digest: string;
  state: PluginState;
  failure_class?: PluginFailureClass;
  failure_message?: string;
  failure_at?: string;
  manifest: PluginManifest;
  capabilities: PluginCapabilities;
  ui: PluginUIContribution;
  source_type?: string;
  source_ref?: string;
  resolved_url?: string;
  install_path?: string;
  source_commit?: string;
  permissions_ack_at?: string;
  install_log?: PluginInstallLogStep[];
  created_at: string;
  updated_at: string;
}

// ---- Logs ----

export interface LogSite {
  domain: string;
  name?: string;
}

export interface LogTailResponse {
  domain: string;
  type: string;
  path: string;
  lines: string[];
  line_count?: number;
}

// ---- Database ----

export interface DatabaseTablesResponse {
  path: string;
  tables: string[];
}

export interface DatabaseTableData {
  name: string;
  columns: string[];
  rows: Array<Record<string, unknown> | unknown[]>;
  limit: number;
  offset: number;
  count: number;
}

// ---- Observability ----

export interface QueryEvent {
  /** Stable composite key (source|ts|message) used for dedup. */
  id: string;
  ts: string;
  source: string;
  action: string;
  user: string;
  message: string;
  meta: Record<string, unknown>;
}

export interface QueryOption {
  value?: string;
  label?: string;
  [k: string]: unknown;
}

export interface QuerySourceMeta {
  id: string;
  label: string;
  group: string;
  description: string;
  query: { source: string; site?: string };
  fields: QueryOption[];
  quick_filters: Array<{ label: string; value: string }>;
  examples: string[];
}

export interface QueryMetaResponse {
  syntax_hint: string;
  time_ranges: Array<{ value: string; label: string; offset_ms?: number }>;
  sources: QuerySourceMeta[];
}

export interface QueryResponse {
  hits: number;
  events: QueryEvent[];
}

export interface SavedQuery {
  id: number;
  name: string;
  source: string;
  query: string;
  created_at: string;
  updated_at: string;
}

export interface SavedQueryUpdateRequest {
  name?: string;
  source?: string;
  q?: string;
}

// ---- Metrics ----

export type SeriesPoint = [string, number];

export interface TrafficSeriesResponse {
  step: string;
  requests: Record<string, SeriesPoint[]>;
  bytes: Record<string, SeriesPoint[]>;
}

export interface TopSiteEntry {
  site: string;
  requests: number;
  bytes: number;
}

export interface StatusCodesResponse {
  s2xx: number;
  s3xx: number;
  s4xx: number;
  s5xx: number;
}

export interface TrafficMetricsSummary {
  range: string;
  requests: number;
  bytes: number;
}

// ---- UI meta (backend-owned options) ----

export interface UiOption {
  value: string;
  label: string;
  hint?: string;
}

export interface UiCapability {
  enabled: boolean;
  mode?: string;
  label?: string;
  hint?: string;
}

export interface UiMetaResponse {
  app: { name: string; env: string; logo_letter: string };
  auth: {
    login_hint: string;
    login_email_placeholder?: string;
    remember_me: boolean;
    basic_auth_enabled: boolean;
    lockscreen_enabled: boolean;
    lock_after_seconds: number;
  };
  navigation?: Array<{ path: string; label: string; group: string; icon: string }>;
  files: { roots: Array<{ path: string; label?: string }>; actions: UiOption[] };
  logs?: { tail_kinds: UiOption[] };
  nginx: { test: UiCapability; reload: UiCapability };
  cron: { run_every_options: UiOption[]; manual_run: UiCapability };
  mounts: {
    default_options: string;
    dump_default: string;
    fsck_default: string;
    fs_types: UiOption[];
    example: string;
  };
  traffic: { ranges: UiOption[] };
  docker: { restart: UiCapability; stop: UiCapability; logs: UiCapability };
  websites: {
    types: UiOption[];
    web_root: string;
    static_path_hint: string;
    proxy_upstream_hint: string;
  };
}
