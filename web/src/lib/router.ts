// Tiny hash-based router. Stateless: the URL hash is the single source of truth.
import { useEffect, useState } from 'preact/hooks';

export interface Route {
  path: string; // e.g. "/websites"
  params: Record<string, string>; // parsed query (?key=value)
}

function parseHash(): Route {
  const raw = globalThis.location.hash.replace(/^#/, '') || '/dashboard';
  const [path, queryStr] = raw.split('?');
  const params: Record<string, string> = {};
  if (queryStr) {
    for (const [k, v] of new URLSearchParams(queryStr)) params[k] = v;
  }
  return { path: path || '/dashboard', params };
}

export function navigate(path: string, params?: Record<string, string | number | undefined>) {
  let hash = path;
  if (params) {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') qs.append(k, String(v));
    }
    const s = qs.toString();
    if (s) hash += `?${s}`;
  }
  globalThis.location.hash = hash;
}

export function useRoute(): Route {
  const [route, setRoute] = useState<Route>(parseHash());
  useEffect(() => {
    const onChange = () => setRoute(parseHash());
    globalThis.addEventListener('hashchange', onChange);
    return () => globalThis.removeEventListener('hashchange', onChange);
  }, []);
  return route;
}
