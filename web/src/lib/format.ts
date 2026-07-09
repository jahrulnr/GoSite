// Pure formatting helpers. No app data is hardcoded here — just display utilities.

export function formatBytes(bytes: number | undefined | null, digits = 1): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return '—';
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.min(Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** i;
  return `${value.toFixed(i === 0 ? 0 : digits)} ${units[i]}`;
}

/** Format a value expressed in KiB (e.g. /proc/meminfo, `df` 1K-blocks). */
export function formatKiB(kib: number | undefined | null, digits = 1): string {
  if (kib === undefined || kib === null || Number.isNaN(kib)) return '—';
  return formatBytes(kib * 1024, digits);
}

export function formatNumber(n: number | undefined | null): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—';
  return n.toLocaleString();
}

/** Compact numeric display for rates and small decimals (req/s, etc.). */
export function formatRate(n: number | undefined | null, digits = 2): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—';
  if (n === 0) return '0';
  const abs = Math.abs(n);
  if (abs >= 100) return n.toFixed(0);
  if (abs >= 10) return n.toFixed(1);
  return n.toFixed(digits);
}

/** Short time label for chart axes. */
export function formatAxisTime(value: string | number | Date | undefined | null): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

export function formatPercent(n: number | undefined | null, digits = 0): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—';
  return `${n.toFixed(digits)}%`;
}

export function formatDate(value: string | number | Date | undefined | null): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatRelative(value: string | number | Date | undefined | null): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  const diff = Date.now() - d.getTime();
  const sec = Math.round(diff / 1000);
  if (Math.abs(sec) < 60) return 'just now';
  const min = Math.round(sec / 60);
  if (Math.abs(min) < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (Math.abs(hr) < 24) return `${hr}h ago`;
  const day = Math.round(hr / 24);
  return `${day}d ago`;
}

export function initials(name: string | undefined): string {
  if (!name) return '?';
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  const last = parts[parts.length - 1] ?? '';
  return (parts[0][0] + last[0]).toUpperCase();
}

/** Convert backend query meta time-range token to ISO `from` timestamp. */
export function queryRangeFrom(
  value: string | undefined,
  ranges?: Array<{ value: string; offset_ms?: number }>,
): string | undefined {
  if (!value || value === 'all') return undefined;
  const match = ranges?.find((item) => item.value === value);
  if (match?.offset_ms) {
    return new Date(Date.now() - match.offset_ms).toISOString();
  }
  const offsets: Record<string, number> = {
    '1h': 3_600_000,
    '6h': 21_600_000,
    '24h': 86_400_000,
    '1d': 86_400_000,
    '7d': 604_800_000,
    '30d': 2_592_000_000,
  };
  const ms = offsets[value];
  if (!ms) return undefined;
  return new Date(Date.now() - ms).toISOString();
}

/** Show a path relative to web root when possible. */
export function displayPath(webRoot: string | undefined, path: string | undefined): string {
  if (!path) return '—';
  if (webRoot && path.startsWith(webRoot)) {
    const rel = path.slice(webRoot.length).replace(/^\//, '');
    return rel || 'web root';
  }
  return path;
}

/** Format disk I/O counters (often reported as sector counts). */
export function formatDiskSectors(value: string | number | undefined | null): string {
  if (value === undefined || value === null || value === '') return '—';
  const raw = String(value).trim();
  const digits = raw.replace(/\D/g, '');
  if (!digits) return raw;
  const sectors = Number(digits);
  if (Number.isNaN(sectors)) return raw;
  return formatBytes(sectors * 512);
}

export function envLabel(env: string | undefined): string {
  if (!env) return 'Panel';
  return env.charAt(0).toUpperCase() + env.slice(1);
}
