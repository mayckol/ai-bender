export type SSEHandler = (event: MessageEvent) => void;

export interface SSEOptions {
  onOpen?: () => void;
  onError?: (err: Event) => void;
  handlers: Record<string, SSEHandler>;
}

/**
 * Thin EventSource wrapper that attaches typed handlers and exposes a close
 * helper. The browser's EventSource already auto-reconnects.
 */
export function subscribeSSE(url: string, opts: SSEOptions): () => void {
  const es = new EventSource(url);
  if (opts.onOpen) es.addEventListener('open', opts.onOpen);
  if (opts.onError) es.addEventListener('error', opts.onError);
  for (const [name, handler] of Object.entries(opts.handlers)) {
    es.addEventListener(name, handler);
  }
  return () => es.close();
}
