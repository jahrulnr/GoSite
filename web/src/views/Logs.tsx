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
// Hard cap for the rendered event list. Larger buffers make the Preact diff
// walk the whole array on every prepend during Live Tail, which freezes the
// main thread. 200 keeps the list scannable without blowing the budget.
const MAX_LIVE_EVENTS = 200;

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

  // Monotonic epoch bumped on every cleanup(). Any async callback that
  // captured the previous epoch must skip its state update, otherwise a
  // late-arriving SSE event or rAF flush can overwrite the fresh empty list
  // after the user has already kicked off a new query.
  const liveEpochRef = useRef(0);

  // Batched live-tail buffer: events that arrive between animation frames are
  // accumulated and committed in a single setState. This keeps the Preact diff
  // cost at O(MAX_LIVE_EVENTS) per frame instead of O(N) per individual event,
  // which is the difference between a smooth scroll and a frozen tab.
  const liveQueueRef = useRef<QueryEvent[]>([]);
  const liveFlushRef = useRef<number | null>(null);
  const flushLiveQueue = useCallback(() => {
    liveFlushRef.current = null;
    if (liveQueueRef.current.length === 0) return;
    const epoch = liveEpochRef.current;
    const queued = liveQueueRef.current;
    liveQueueRef.current = [];
    // Defer the state update to the next microtask so we can check whether
    // the tail session is still the current one. If cleanup() ran, the epoch
    // will have advanced and we drop the queued events on the floor.
    queueMicrotask(() => {
      if (liveEpochRef.current !== epoch) return;
      setEvents((current) => {
        const merged = queued.concat(current);
        return merged.length > MAX_LIVE_EVENTS ? merged.slice(0, MAX_LIVE_EVENTS) : merged;
      });
      setTotalHits((total) => total + queued.length);
    });
  }, []);

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
    if (liveFlushRef.current !== null) {
      if (typeof cancelAnimationFrame === 'function') {
        cancelAnimationFrame(liveFlushRef.current);
      } else {
        clearTimeout(liveFlushRef.current);
      }
      liveFlushRef.current = null;
    }
    liveQueueRef.current = [];
    // Invalidate any in-flight live flush so a late rAF cannot clobber the
    // empty list we set when the next run kicks off.
    liveEpochRef.current += 1;
  }, []);

  useEffect(() => cleanup, [cleanup]);

  const runHistoryInternal = useCallback(
    async (signal: AbortSignal, opts: { from?: string } = {}): Promise<QueryEvent[]> => {
      if (!source) return [];
      const normalizedQuery = query.trim() === '*' ? '' : query;
      const res = await observability.query(
        {
          source: source.query.source,
          q: normalizedQuery,
          site: source.query.site,
          from: opts.from,
        },
        signal,
      );
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
    // Clear stale events immediately so the user does not see the previous
    // result while the new query is in flight. The empty-state placeholder
    // (loading=true, events=[]) renders the "Scanning…" indicator.
    setEvents([]);
    setTotalHits(0);
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
    // Reset the displayed list so the user does not see stale events from a
    // prior history query while the live seed is in flight.
    setEvents([]);
    setTotalHits(0);
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
          liveQueueRef.current.push(event);
          if (liveFlushRef.current === null && typeof requestAnimationFrame === 'function') {
            liveFlushRef.current = requestAnimationFrame(flushLiveQueue);
          } else if (liveFlushRef.current === null) {
            // SSR / non-browser fallback: flush on next microtask.
            liveFlushRef.current = setTimeout(flushLiveQueue, 16) as unknown as number;
          }
        },
        // Only invoked when the browser has given up reconnecting (the
        // transient-error case is handled silently by EventSource's built-in
        // retry, and must not flip the Stop button back to Run).
        () => {
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
