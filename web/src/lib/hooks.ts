// Small stateless data-fetching helpers built on Preact hooks.
import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { ApiError } from '../api/client';

export interface AsyncState<T> {
  data: T | undefined;
  error: ApiError | Error | undefined;
  loading: boolean;
  reload: () => void;
}

/**
 * Run an async loader on mount (and when deps change). Always fetches fresh —
 * no caching, keeping the UI stateless against the backend.
 */
export function useAsync<T>(loader: (signal: AbortSignal) => Promise<T>, deps: unknown[] = []): AsyncState<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<ApiError | Error>();
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);
  const loaderRef = useRef(loader);
  loaderRef.current = loader;

  useEffect(() => {
    const ctrl = new AbortController();
    let active = true;
    setLoading(true);
    setError(undefined);
    loaderRef
      .current(ctrl.signal)
      .then((res) => {
        if (active) {
          setData(res);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (active && err?.name !== 'AbortError') {
          setError(err);
          setLoading(false);
        }
      });
    return () => {
      active = false;
      ctrl.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, error, loading, reload };
}

/** Imperative async action with loading + error tracking (for forms, buttons). */
export function useAction<TArgs extends unknown[], TResult>(
  fn: (...args: TArgs) => Promise<TResult>,
) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | Error>();
  const run = useCallback(
    async (...args: TArgs): Promise<TResult | undefined> => {
      setLoading(true);
      setError(undefined);
      try {
        return await fn(...args);
      } catch (err) {
        setError(err as Error);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  return { run, loading, error };
}

/** Poll a loader at a fixed interval; pauses when the tab is hidden. */
export function useInterval(callback: () => void, ms: number) {
  const cb = useRef(callback);
  cb.current = callback;
  useEffect(() => {
    let id = 0;
    const start = () => {
      stop();
      id = globalThis.setInterval(() => {
        if (!document.hidden) cb.current();
      }, ms);
    };
    const stop = () => {
      if (id) globalThis.clearInterval(id);
      id = 0;
    };
    start();
    return stop;
  }, [ms]);
}
