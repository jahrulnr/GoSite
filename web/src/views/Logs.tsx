// Splunk-style /logs view: a single sticky search bar (LogsSearch) and a
// monospace event stream (EventStream). History runs POST /query, Live Tail
// seeds with /query and then opens a Server-Sent Events stream on /query/tail
// so new events arrive in real time. State is persisted to localStorage.

import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { observability } from '../api/endpoints';
import type { QueryEvent, QuerySourceMeta, SavedQuery } from '../api/types';
import { EventStream } from '../components/EventStream';
import {
  LogsSearch,
  type LogFormat,
  type LogMode,
} from '../components/LogsSearch';
import { Page } from '../components/Layout';
import { humanizeError } from '../lib/errors';
import { queryRangeFrom } from '../lib/format';
import { useStore } from '../lib/store';

const STORAGE_KEY = 'gosite:logs:search-state';
const MAX_LIVE_EVENTS = 500;

interface SearchState {
  sourceId?: string;
  query?: string;
  timeRange?: string;
  mode?: LogMode;
  format?: LogFormat;
  savedId?: number;
}

function readPersistedState(): SearchState {
  if (typeof window === 'undefined') return {};
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as SearchState;
    if (parsed && typeof parsed === 'object') return parsed;
  } catch {
    // ignore corrupt entry
  }
  return {};
}

function writePersistedState(state: SearchState) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // ignore quota / private-mode failures
  }
}

export function LogsView() {
  const { toast } = useStore();
  const [meta, setMeta] = useState<Awaited<ReturnType<typeof observability.queryMeta>> | undefined>();
  const [metaError, setMetaError] = useState<Error | undefined>();
  const [savedQueries, setSavedQueries] = useState<SavedQuery[]>([]);
  const [activeSavedId, setActiveSavedId] = useState<number | undefined>();

  const persisted = useRef<SearchState>(readPersistedState());

  const [source, setSource] = useState<QuerySourceMeta | undefined>();
  const [query, setQuery] = useState<string>(persisted.current.query ?? '');
  const [timeRange, setTimeRange] = useState<string>(persisted.current.timeRange ?? '');
  const [mode, setMode] = useState<LogMode>(persisted.current.mode ?? 'history');
  const [format, setFormat] = useState<LogFormat>(persisted.current.format ?? 'smart');

  const [running, setRunning] = useState(false);
  const [events, setEvents] = useState<QueryEvent[]>([]);
  const [totalHits, setTotalHits] = useState(0);
  const [error, setError] = useState<Error | undefined>();

  const abortRef = useRef<AbortController | undefined>(undefined);
  const stopTailRef = useRef<(() => void) | undefined>(undefined);

  // Persist state on change.
  useEffect(() => {
    const state: SearchState = {
      sourceId: source?.id ?? persisted.current.sourceId,
      query,
      timeRange,
      mode,
      format,
      savedId: activeSavedId,
    };
    writePersistedState(state);
  }, [source?.id, query, timeRange, mode, format, activeSavedId]);

  // Initial metadata load.
  useEffect(() => {
    let active = true;
    observability
      .queryMeta()
      .then((data) => {
        if (!active) return;
        setMeta(data);
      })
      .catch((err: Error) => {
        if (!active) return;
        setMetaError(err);
      });
    return () => {
      active = false;
    };
  }, []);

  // Saved queries.
  const refetchSaved = useCallback(() => {
    observability
      .savedQueries()
      .then((items) => setSavedQueries(items ?? []))
      .catch((err: Error) => toast(humanizeError(err), 'error'));
  }, [toast]);

  useEffect(() => {
    refetchSaved();
  }, [refetchSaved]);

  // After metadata loads, apply defaults / restore from storage.
  useEffect(() => {
    if (!meta) return;
    if (source) return;
    const sources = meta.sources ?? [];
    if (sources.length === 0) return;
    const persistedId = persisted.current.sourceId;
    const restored = persistedId ? sources.find((s) => s.id === persistedId) : undefined;
    const initial = restored ?? sources[0];
    if (!initial) return;
    setSource(initial);
    if (!timeRange && meta.time_ranges?.[0]) {
      setTimeRange(String(meta.time_ranges[0].value ?? ''));
    }
    if (!query) {
      setQuery(initial.examples?.[0] ?? '');
    }
  }, [meta, source, query, timeRange]);

  const cleanup = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = undefined;
    if (stopTailRef.current) {
      stopTailRef.current();
      stopTailRef.current = undefined;
    }
  }, []);

  useEffect(() => cleanup, [cleanup]);

  const runHistoryInternal = useCallback(
    async (signal: AbortSignal, opts: { from?: string } = {}): Promise<QueryEvent[]> => {
      if (!source) return [];
      const res = await observability.query({
        source: source.query.source,
        q: query,
        site: source.query.site,
        from: opts.from,
      });
      if (signal.aborted) return [];
      setTotalHits(res.hits);
      return res.events;
    },
    [source, query],
  );

  const runHistory = useCallback(async () => {
    if (!source) return;
    cleanup();
    setError(undefined);
    setRunning(true);
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    try {
      const from = queryRangeFrom(timeRange, meta?.time_ranges);
      const list = await runHistoryInternal(ctrl.signal, { from });
      if (!ctrl.signal.aborted) {
        setEvents(list);
      }
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      const e = err as Error;
      setError(e);
      toast(humanizeError(e), 'error');
    } finally {
      if (abortRef.current === ctrl) abortRef.current = undefined;
      setRunning(false);
    }
  }, [source, timeRange, meta?.time_ranges, runHistoryInternal, cleanup, toast]);

  const startLive = useCallback(async () => {
    if (!source) return;
    cleanup();
    setError(undefined);
    setRunning(true);
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    try {
      const from = queryRangeFrom(timeRange, meta?.time_ranges);
      const seed = await runHistoryInternal(ctrl.signal, { from });
      if (ctrl.signal.aborted) return;
      setEvents(seed);

      const url = observability.tailUrl({
        source: source.query.source,
        site: source.query.site,
        from,
      });
      const stop = observability.startTail(
        url,
        (event) => {
          setEvents((current) => {
            const next = [event, ...current];
            return next.length > MAX_LIVE_EVENTS ? next.slice(0, MAX_LIVE_EVENTS) : next;
          });
          setTotalHits((total) => total + 1);
        },
        () => {
          // Stream closed (network error or backend close). Stop the tail.
          if (stopTailRef.current === stop) {
            stopTailRef.current = undefined;
            setRunning(false);
          }
        },
      );
      stopTailRef.current = stop;
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      const e = err as Error;
      setError(e);
      toast(humanizeError(e), 'error');
      setRunning(false);
    } finally {
      if (abortRef.current === ctrl) abortRef.current = undefined;
    }
  }, [source, timeRange, meta?.time_ranges, runHistoryInternal, cleanup, toast]);

  const handleRun = useCallback(() => {
    if (mode === 'live') void startLive();
    else void runHistory();
  }, [mode, startLive, runHistory]);

  const handleStop = useCallback(() => {
    cleanup();
    setRunning(false);
  }, [cleanup]);

  // When the user toggles modes, abort any in-flight work.
  useEffect(() => {
    cleanup();
    setRunning(false);
  }, [mode, cleanup]);

  // Saved-query handlers.
  const handleSaveCurrent = useCallback(
    async (name: string) => {
      if (!source) return;
      try {
        const created = await observability.saveQuery(name, source.query.source, query);
        setActiveSavedId(created.id);
        refetchSaved();
        toast(`Saved “${created.name}”`);
      } catch (err) {
        toast(humanizeError(err as Error), 'error');
      }
    },
    [source, query, refetchSaved, toast],
  );

  const handleRenameSaved = useCallback(
    async (id: number, name: string) => {
      try {
        await observability.updateSavedQuery(id, { name });
        refetchSaved();
        toast('Renamed');
      } catch (err) {
        toast(humanizeError(err as Error), 'error');
      }
    },
    [refetchSaved, toast],
  );

  const handleDeleteSaved = useCallback(
    async (id: number) => {
      try {
        await observability.deleteSavedQuery(id);
        setSavedQueries((current) => current.filter((item) => item.id !== id));
        if (activeSavedId === id) setActiveSavedId(undefined);
        toast('Deleted');
      } catch (err) {
        toast(humanizeError(err as Error), 'error');
      }
    },
    [activeSavedId, toast],
  );

  const handleLoadSaved = useCallback(
    (item: SavedQuery) => {
      const match = meta?.sources.find((s) => s.query.source === item.source);
      if (match) setSource(match);
      setQuery(item.query);
      setActiveSavedId(item.id);
      toast(`Loaded “${item.name}”`);
    },
    [meta?.sources, toast],
  );

  if (metaError && !meta) {
    return (
      <Page title="Logs" subtitle="Splunk-style search across audit, jobs, and nginx access/error logs" eyebrow="observability">
        <div class="error-box">
          <strong>Could not load query metadata</strong>
          <span>{humanizeError(metaError)}</span>
        </div>
      </Page>
    );
  }

  if (!meta) {
    return (
      <Page title="Logs" subtitle="Splunk-style search across audit, jobs, and nginx access/error logs" eyebrow="observability">
        <div class="loading">
          <output class="spinner" />
          <span>Loading search metadata…</span>
        </div>
      </Page>
    );
  }

  const sources = meta.sources ?? [];
  const timeRanges = (meta.time_ranges ?? []).map((item) => ({
    value: String(item.value ?? ''),
    label: String(item.label ?? item.value ?? ''),
  }));
  const syntaxHint = meta.syntax_hint ?? '';

  return (
    <Page
      title="Logs"
      subtitle="Splunk-style search across audit, jobs, and nginx access/error logs"
      eyebrow="observability"
    >
      <LogsSearch
        source={source}
        sources={sources}
        onSourceChange={(next) => {
          cleanup();
          setSource(next);
          setActiveSavedId(undefined);
        }}
        query={query}
        onQueryChange={setQuery}
        timeRange={timeRange}
        timeRanges={timeRanges}
        onTimeRangeChange={setTimeRange}
        mode={mode}
        onModeChange={setMode}
        running={running}
        onRun={handleRun}
        onStop={handleStop}
        saved={savedQueries}
        onSaveCurrent={handleSaveCurrent}
        onLoadSaved={handleLoadSaved}
        onRenameSaved={handleRenameSaved}
        onDeleteSaved={handleDeleteSaved}
        format={format}
        onFormatChange={setFormat}
        syntaxHint={syntaxHint}
      />
      <EventStream events={events} loading={running} error={error} format={format} totalHits={totalHits} />
    </Page>
  );
}
