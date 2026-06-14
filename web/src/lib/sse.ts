// Stateless SSE consumer for job output streams (certbot, cron run).
// Uses EventSource with credentials so session cookies are sent.

export interface JobStreamHandlers {
  onLine: (line: string) => void;
  onDone?: (status: string) => void;
  onError?: (message: string) => void;
}

export function openJobStream(url: string, handlers: JobStreamHandlers): () => void {
  const es = new EventSource(url, { withCredentials: true });

  es.onmessage = (event) => {
    const text = String(event.data ?? '');
    if (text.startsWith('status=')) {
      const status = text.slice('status='.length);
      if (status === 'done' || status === 'failed' || status === 'ok') {
        handlers.onDone?.(status);
        es.close();
      }
      return;
    }
    for (const line of text.split(/\r?\n/)) {
      if (line !== '') handlers.onLine(line);
    }
  };

  es.addEventListener('done', (event) => {
    const status = String((event as MessageEvent).data ?? 'done');
    handlers.onDone?.(status);
    es.close();
  });

  es.onerror = () => {
    handlers.onError?.('Stream disconnected');
    es.close();
  };

  return () => es.close();
}
