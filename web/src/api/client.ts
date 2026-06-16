// Thin, stateless fetch wrapper for the GoSite API.
// - Always sends cookies (session auth).
// - Surfaces the backend error envelope `{ error: { code, message } }`.
// - No data is cached; callers fetch fresh on demand.

import type { ApiErrorBody } from './types';

export const API_BASE = '/api/v1';

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export interface RequestOptions {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
  /** When true, send `body` as FormData instead of JSON. */
  formData?: FormData;
  /** Query string params; undefined/null values are skipped. */
  query?: Record<string, string | number | boolean | undefined | null>;
}

function buildUrl(path: string, query?: RequestOptions['query']): string {
  const url = path.startsWith('http') ? path : `${API_BASE}${path}`;
  if (!query) return url;
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null) continue;
    qs.append(k, String(v));
  }
  const s = qs.toString();
  return s ? `${url}?${s}` : url;
}

async function parseError(res: Response): Promise<ApiError> {
  let code = 'http_error';
  let message = `Request failed (${res.status})`;
  try {
    const data = (await res.json()) as Partial<ApiErrorBody>;
    if (data?.error) {
      code = data.error.code ?? code;
      message = data.error.message ?? message;
    }
  } catch {
    // non-JSON error body; keep defaults
  }
  return new ApiError(res.status, code, message);
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {};
  let body: BodyInit | undefined;

  if (opts.formData) {
    body = opts.formData;
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }

  const url = buildUrl(path, opts.query);
  let res: Response;
  try {
    res = await fetch(url, {
      method: opts.method ?? 'GET',
      headers,
      body,
      credentials: 'include',
      signal: opts.signal,
    });
  } catch (err) {
    if ((err as Error).name === 'AbortError') throw err;
    const protocol = globalThis.location?.protocol === 'https:' ? 'HTTPS' : 'HTTP';
    throw new ApiError(
      0,
      'network_error',
      `Network request failed before the server returned a response (${protocol} ${url}). Check DevTools Network for CORS/TLS/proxy errors.`,
    );
  }

  if (res.status === 401) {
    throw await parseError(res);
  }
  if (!res.ok) {
    throw await parseError(res);
  }
  if (res.status === 204) {
    return undefined as unknown as T;
  }
  const ctype = res.headers.get('content-type') ?? '';
  if (!ctype.includes('application/json')) {
    return (await res.text()) as unknown as T;
  }
  return (await res.json()) as T;
}

export const http = {
  get: <T>(path: string, query?: RequestOptions['query'], signal?: AbortSignal) =>
    request<T>(path, { method: 'GET', query, signal }),
  post: <T>(path: string, body?: unknown, signal?: AbortSignal) =>
    request<T>(path, { method: 'POST', body, signal }),
  put: <T>(path: string, body?: unknown, signal?: AbortSignal) =>
    request<T>(path, { method: 'PUT', body, signal }),
  patch: <T>(path: string, body?: unknown, signal?: AbortSignal) =>
    request<T>(path, { method: 'PATCH', body, signal }),
  del: <T>(path: string, query?: RequestOptions['query'], signal?: AbortSignal) =>
    request<T>(path, { method: 'DELETE', query, signal }),
  upload: <T>(path: string, form: FormData) =>
    request<T>(path, { method: 'POST', formData: form }),
};

export function rootRequest<T>(path: string, signal?: AbortSignal): Promise<T> {
  return request<T>(path.startsWith('/') ? path.replace(/^\//, `${globalThis.location.origin}/`) : path, {
    method: 'GET',
    signal,
  });
}
