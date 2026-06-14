// Flat, monospace event list for the Splunk-style /logs view.
// - Smart format highlights `key=value` / `key="value"` pairs in the message body.
// - Source badge color depends on the event's `source` (audit/access/error/job).
// - Row click toggles an inline detail panel with the full event `meta` JSON.
// - Empty / loading / error states are localized to the list area.

import { useState } from 'preact/hooks';
import type { JSX } from 'preact';
import type { QueryEvent } from '../api/types';
import type { LogFormat } from './LogsSearch';

export interface Props {
  events: QueryEvent[];
  loading: boolean;
  error?: Error;
  format: LogFormat;
  totalHits: number;
}

const FIELD_REGEX = /([\w.-]+)=(?:"([^"]*)"|(\S+))/g;

function formatTimestamp(value: string): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function statusClass(code: unknown): string {
  if (typeof code !== 'number' || Number.isNaN(code)) return '';
  if (code >= 500) return 's5';
  if (code >= 400) return 's4';
  if (code >= 300) return 's3';
  if (code >= 200) return 's2';
  return '';
}

function sourceBadgeClass(source: string): string {
  const known = new Set(['audit', 'access', 'error', 'job']);
  return known.has(source) ? `evt-source-${source}` : '';
}

function renderSmartMessage(message: string): JSX.Element | string {
  if (!message) return '';
  const parts: Array<string | JSX.Element> = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = 0;
  FIELD_REGEX.lastIndex = 0;
  while ((match = FIELD_REGEX.exec(message)) !== null) {
    if (match.index > lastIndex) {
      const bare = message.slice(lastIndex, match.index);
      parts.push(
        <span class="bare" key={`bare-${key++}`}>
          {bare}
        </span>,
      );
    }
    const field = match[1];
    const value = match[2] ?? match[3] ?? '';
    parts.push(
      <span class="evt-key" key={`key-${key++}`}>
        {field}
      </span>,
    );
    parts.push(
      <span class="evt-eq" key={`eq-${key++}`}>
        =
      </span>,
    );
    parts.push(
      <span class="evt-val" key={`val-${key++}`}>
        {value}
      </span>,
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < message.length) {
    const bare = message.slice(lastIndex);
    parts.push(
      <span class="bare" key={`bare-${key++}`}>
        {bare}
      </span>,
    );
  }
  if (parts.length === 0) return message;
  return <>{parts}</>;
}

function getStatusCode(meta: Record<string, unknown>): number | undefined {
  const candidate = meta.status_code ?? meta.status ?? meta.code;
  if (typeof candidate === 'number') return candidate;
  if (typeof candidate === 'string' && /^\d+$/.test(candidate)) return Number(candidate);
  return undefined;
}

function formatJson(value: unknown, indent = 2): string {
  try {
    return JSON.stringify(value, null, indent);
  } catch {
    return String(value);
  }
}

export function EventStream({ events, loading, error, format, totalHits }: Readonly<Props>) {
  // Track the expanded row by a stable key (ts+index at insert time) so that
  // prepending a new event in live-tail mode doesn't shift the open row.
  const [expanded, setExpanded] = useState<string | null>(null);
  const isSmart = format === 'smart';

  const meta = (
    <div class="evt-meta">
      <span class={`dot${loading ? '' : ' idle'}`} aria-hidden="true" />
      <span>
        {events.length.toLocaleString()} event{events.length === 1 ? '' : 's'} shown
        <span class="sep"> · </span>
        {totalHits.toLocaleString()} total
      </span>
    </div>
  );

  if (error && events.length === 0) {
    return (
      <div>
        {meta}
        <div class="evt-state">
          <span class="scan-dot" style="background:var(--danger);box-shadow:0 0 8px oklch(64% 0.23 22 / 0.6);animation:none;" />
          <strong>Query failed</strong>
          <span>{error.message || 'Something went wrong while running the search.'}</span>
        </div>
      </div>
    );
  }

  if (loading && events.length === 0) {
    return (
      <div>
        {meta}
        <div class="evt-state">
          <span class="scan-dot" />
          <strong>Scanning…</strong>
          <span>Querying the audit, log, and job tables.</span>
        </div>
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div>
        {meta}
        <div class="evt-state">
          <strong>No events matched.</strong>
          <span>Try a different time range, a broader query, or switch to Live Tail.</span>
        </div>
      </div>
    );
  }

  return (
    <div>
      {meta}
      <div class="evt-list" role="list">
        {events.map((event, index) => {
          const key = `${event.ts}-${index}`;
          const isOpen = expanded === key;
          const code = getStatusCode(event.meta);
          const badgeClass = sourceBadgeClass(event.source);
          return (
            <div key={key}>
              <div
                class="evt-row"
                role="listitem"
                tabIndex={0}
                aria-expanded={isOpen}
                onClick={() => setExpanded(isOpen ? null : key)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    setExpanded(isOpen ? null : key);
                  }
                }}
              >
                <span class="ts">{formatTimestamp(event.ts)}</span>
                <span class={`src ${badgeClass}`}>{event.source || '—'}</span>
                <span class="msg">{isSmart ? renderSmartMessage(event.message) : event.message}</span>
                {event.user ? <span class="user" title={event.user}>@{event.user}</span> : <span />}
                {code !== undefined ? (
                  <span class={`status ${statusClass(code)}`}>{code}</span>
                ) : (
                  <span />
                )}
              </div>
              {isOpen && (
                <div class="evt-detail">
                  <pre>{formatJson(event.meta)}</pre>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
