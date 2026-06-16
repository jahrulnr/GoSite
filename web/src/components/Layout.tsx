import type { ComponentChildren, ComponentType } from 'preact';
import { useEffect, useMemo, useState } from 'preact/hooks';
import { auth, rootHealth, ui } from '../api/endpoints';
import type { TrafficSite, UiMetaResponse, UiOption } from '../api/types';
import { useIdleLock } from '../lib/idleLock';
import {
  IconChart,
  IconClock,
  IconBookmark,
  IconDashboard,
  IconDatabase,
  IconDisk,
  IconDocker,
  IconExternal,
  IconFolder,
  IconGlobe,
  IconLock,
  IconLogout,
  IconPlug,
  IconSearch,
  IconServer,
  IconSettings,
  IconTerminal,
} from './Icons';
import { ErrorState, Loading, Toasts } from './Ui';
import { initials, envLabel } from '../lib/format';
import { mergePanelMeta } from '../lib/meta';
import { useAction, useAsync } from '../lib/hooks';
import { navigate, useRoute } from '../lib/router';
import { useStore } from '../lib/store';
import { useTerminalStore } from '../lib/terminalStore';
import { TerminalWindow } from './TerminalWindow';
import { Lockscreen, Login } from '../views/Login';
import { CronView } from '../views/Cron';
import { DashboardView } from '../views/Dashboard';
import { DatabaseView } from '../views/Database';
import { DockerView } from '../views/Docker';
import { FilesView } from '../views/Files';
import { LogsView } from '../views/Logs';
import { MetricsView } from '../views/Metrics';
import { MountsView } from '../views/Mounts';
import { NginxView } from '../views/Nginx';
import { PluginContributionView, PluginsView } from '../views/Plugins';
import { SettingsView } from '../views/Settings';
import { WebsitesView } from '../views/Websites';

type Icon = ComponentType<{ width?: number; height?: number }>;

const iconByKey: Record<string, Icon> = {
  dashboard: IconDashboard,
  globe: IconGlobe,
  chart: IconChart,
  search: IconSearch,
  folder: IconFolder,
  database: IconDatabase,
  docker: IconDocker,
  clock: IconClock,
  disk: IconDisk,
  plug: IconPlug,
  server: IconServer,
  settings: IconSettings,
};

interface NavItem {
  path: string;
  label: string;
  group: string;
  icon: Icon;
  meta?: string;
}

/** Fallback when /ui/meta has not loaded yet. */
export const navItems: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard', group: 'Observe', icon: IconDashboard, meta: 'live' },
  { path: '/metrics', label: 'Traffic', group: 'Observe', icon: IconChart, meta: 'metrics' },
  { path: '/logs', label: 'Logs', group: 'Observe', icon: IconSearch, meta: 'query' },
  { path: '/websites', label: 'Websites', group: 'Operate', icon: IconGlobe, meta: 'sites' },
  { path: '/nginx', label: 'Nginx', group: 'Config', icon: IconServer, meta: 'config' },
  { path: '/settings', label: 'Settings', group: 'Config', icon: IconSettings, meta: 'account' },
  { path: '/files', label: 'Files', group: 'Storage', icon: IconFolder, meta: 'storage' },
  { path: '/database', label: 'Database', group: 'Storage', icon: IconDatabase, meta: 'sqlite' },
  { path: '/docker', label: 'Docker', group: 'Runtime', icon: IconDocker, meta: 'containers' },
  { path: '/cron', label: 'Cron', group: 'Runtime', icon: IconClock, meta: 'jobs' },
  { path: '/mounts', label: 'Mounts', group: 'Runtime', icon: IconDisk, meta: 'fstab' },
  { path: '/plugins', label: 'Plugins', group: 'Runtime', icon: IconPlug, meta: 'extensions' },
];

const GOSITE_WIKI_URL = 'https://github.com/jahrulnr/GoSite/wiki';

function navFromMeta(meta: UiMetaResponse | undefined): NavItem[] {
  const items = meta?.navigation;
  if (!items?.length) return navItems;
  return items.map((item) => ({
    path: item.path,
    label: item.label,
    group: item.group,
    icon: iconByKey[item.icon] ?? IconDashboard,
  }));
}

export function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}

export function siteTraffic(summary?: { sites?: Record<string, TrafficSite>; total?: TrafficSite }) {
  return Object.entries(summary?.sites ?? {}).sort((a, b) => b[1].requests - a[1].requests);
}

export function optionLabel(options: UiOption[] | undefined, value: string) {
  return options?.find((item) => item.value === value)?.label ?? value;
}

export function Page({
  title,
  subtitle,
  actions,
  children,
  eyebrow,
}: Readonly<{
  title: string;
  subtitle?: string;
  actions?: ComponentChildren;
  children: ComponentChildren;
  eyebrow?: string;
}>) {
  return (
    <>
      <div class="page-head row wrap">
        <div>
          <h1>
            {eyebrow ? (
              <>
                <span class="accent" style="font-family:var(--mono);font-weight:500;">//</span>
                <span class="dim" style="font-family:var(--mono);font-weight:500;margin:0 6px 0 4px;">{eyebrow}</span>
                <span style="color:var(--text-faint);margin-right:8px;">·</span>
              </>
            ) : null}
            {title}
          </h1>
          {subtitle && <p>{subtitle}</p>}
        </div>
        <div class="spacer" />
        {actions}
      </div>
      {children}
    </>
  );
}

export function Stat({
  label,
  value,
  sub,
  percent,
  tone,
}: Readonly<{
  label: string;
  value: ComponentChildren;
  sub?: ComponentChildren;
  percent?: number;
  tone?: 'default' | 'warn' | 'danger' | 'info';
}>) {
  const toneClass = tone && tone !== 'default' ? ` ${tone}` : '';
  return (
    <div class={`stat${toneClass}`}>
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

export function Card({
  title,
  actions,
  children,
  flush,
  tight,
}: Readonly<{ title: string; actions?: ComponentChildren; children: ComponentChildren; flush?: boolean; tight?: boolean }>) {
  return (
    <section class="card">
      <div class="card-head">
        <h3>{title}</h3>
        {actions && <div class="actions">{actions}</div>}
      </div>
      <div class={`card-body${flush ? ' flush' : ''}${tight ? ' tight' : ''}`}>{children}</div>
    </section>
  );
}

export function SimpleRows({ rows }: Readonly<{ rows: ComponentChildren[][] }>) {
  if (rows.length === 0) return <div class="empty"><strong>No data</strong></div>;
  return (
    <div class="table-wrap">
      <table class="table">
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function BootGate({ children }: Readonly<{ children: ComponentChildren }>) {
  const { user, locked, setUser, setMeta, setLocked } = useStore();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error>();

  useEffect(() => {
    let active = true;
    Promise.allSettled([ui.meta(), auth.metadata(), auth.me(), auth.lockscreen()])
      .then(([metaRes, authMetaRes, meRes, lockRes]) => {
        if (!active) return;
        if (authMetaRes.status === 'fulfilled') {
          const metaValue = metaRes.status === 'fulfilled' ? metaRes.value : undefined;
          setMeta(mergePanelMeta(metaValue, authMetaRes.value));
        } else if (metaRes.status === 'fulfilled') {
          setMeta(metaRes.value);
        }
        if (meRes.status === 'fulfilled') {
          const value = meRes.value;
          setUser('user' in value ? value.user : value);
        }
        if (lockRes.status === 'fulfilled') setLocked(Boolean(lockRes.value.locked));
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (!active) return;
        setError(asError(err));
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [setLocked, setMeta, setUser]);

  if (loading) return <Loading label="Opening GoSite" />;
  if (error) return <ErrorState error={error} />;
  if (locked) return <Lockscreen />;
  if (!user) return <Login />;
  return <>{children}</>;
}

function RouteView({ path, params }: Readonly<{ path: string; params: Record<string, string> }>) {
  if (path === '/plugins/keyring') {
    return <PluginsView tab="keyring" />;
  }
  if (path.startsWith('/plugins/')) {
    return <PluginContributionView path={path} />;
  }
  switch (path) {
    case '/websites':
      return <WebsitesView />;
    case '/metrics':
      return <MetricsView />;
    case '/logs':
      return <LogsView />;
    case '/files':
      return <FilesView path={params.path ?? ''} />;
    case '/database':
      return <DatabaseView table={params.table} />;
    case '/docker':
      return <DockerView />;
    case '/cron':
      return <CronView />;
    case '/mounts':
      return <MountsView />;
    case '/nginx':
      return <NginxView />;
    case '/settings':
      return <SettingsView />;
    case '/plugins':
      return <PluginsView />;
    default:
      return <DashboardView />;
  }
}

function TopbarCrumb({ path }: Readonly<{ path: string }>) {
  const item = path.startsWith('/plugins/') ? navItems.find((it) => it.path === '/plugins') : navItems.find((it) => it.path === path);
  if (!item) {
    return (
      <div class="topbar-crumb">
        <span>GoSite</span>
        <span class="sep">/</span>
        <span class="current">{path.replace(/^\//, '') || 'home'}</span>
      </div>
    );
  }
  return (
    <div class="topbar-crumb">
      <span>GoSite</span>
      <span class="sep">/</span>
      <span>{item.group}</span>
      <span class="sep">/</span>
      <span class="current">{item.label}</span>
    </div>
  );
}

function HealthPill() {
  const health = useAsync(() =>
    rootHealth()
      .then((res) => res)
      .catch(() => undefined),
  );
  const data = health.data as { status?: string } | undefined;
  const ok = data?.status === 'ok';
  return (
    <div class={`health-pill ${ok ? '' : 'warn'}`} title="Backend /health">
      <span class="dot" />
      <span>API · {ok ? 'healthy' : (data?.status ?? '—')}</span>
    </div>
  );
}

export function Shell() {
  const route = useRoute();
  const { meta, user, setUser, setLocked, toast } = useStore();
  const { run: runLogout, loading: loggingOut } = useAction(auth.logout);
  const { run: runLock, loading: locking } = useAction(auth.lock);
  useIdleLock();
  const grouped = useMemo(() => {
    const map = new Map<string, NavItem[]>();
    for (const item of navFromMeta(meta)) map.set(item.group, [...(map.get(item.group) ?? []), item]);
    return [...map.entries()];
  }, [meta]);

  const logout = async () => {
    await runLogout();
    setUser(undefined);
    toast('Signed out');
  };

  const lock = async () => {
    await runLock();
    setLocked(true);
  };

  return (
    <div class="shell">
      <div class="app-bg-glow" />
      <aside class="sidebar">
        <div class="sidebar-brand">
          <div class="brand-logo">{meta?.app?.logo_letter ?? 'G'}</div>
          <div class="brand-meta">
            <div class="brand-name">{meta?.app?.name ?? 'GoSite'}</div>
            <div class="brand-env">{envLabel(meta?.app?.env)}</div>
          </div>
        </div>
        <nav class="nav" aria-label="Primary">
          {grouped.map(([group, items]) => (
            <div key={group} class="nav-group">
              <div class="nav-group-label">{group}</div>
              {items.map((item) => {
                const IconCmp = item.icon;
                const active = route.path === item.path || (item.path === '/plugins' && route.path.startsWith('/plugins/'));
                return (
                  <button
                    type="button"
                    key={item.path}
                    class={`nav-item ${active ? 'active' : ''}`}
                    onClick={() => navigate(item.path)}
                  >
                    <span class="ico"><IconCmp /></span>
                    {item.label}
                  </button>
                );
              })}
            </div>
          ))}
        </nav>
        <div class="sidebar-nav-footer">
          <a
            class="nav-item nav-item--external"
            href={GOSITE_WIKI_URL}
            target="_blank"
            rel="noopener noreferrer"
          >
            <span class="ico"><IconBookmark /></span>
            <span class="nav-item-label">Docs</span>
            <span class="nav-external" aria-hidden="true"><IconExternal /></span>
          </a>
        </div>
        <div class="sidebar-footer">
          <div class="avatar">{initials(user?.name)}</div>
          <div class="truncate">
            <div>{user?.name}</div>
            <div class="dim mono">{user?.email}</div>
          </div>
        </div>
      </aside>
      <main class="main">
        <header class="topbar">
          <TopbarCrumb path={route.path} />
          <div class="topbar-spacer" />
          <div class="row">
            <HealthPill />
            <TerminalButton />
            {meta?.auth?.lockscreen_enabled && (
              <button type="button" class="btn ghost" disabled={locking} onClick={lock}>
                {locking ? '…' : <><IconLock /> Lock</>}
              </button>
            )}
            <button type="button" class="btn ghost" disabled={loggingOut} onClick={logout}>
              {loggingOut ? '…' : <><IconLogout /> Logout</>}
            </button>
          </div>
        </header>
        <div class="content">
          <RouteView path={route.path} params={route.params} />
        </div>
      </main>
      <TerminalWindow />
      <Toasts />
    </div>
  );
}

function TerminalButton() {
  const terminal = useTerminalStore();
  const active = terminal.open;
  return (
    <button
      type="button"
      class="btn ghost topbar-icon-btn--terminal"
      data-active={active}
      title={active ? 'Hide terminal' : 'Open terminal'}
      onClick={() => terminal.toggleTerminal()}
    >
      <IconTerminal /> Terminal
    </button>
  );
}
