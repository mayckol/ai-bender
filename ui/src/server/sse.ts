// Minimal SSE helpers. One writer per HTTP request; the broadcaster logic
// (fan-out from a single file tailer to N subscribers) lives in the route
// handler, not here.

export interface SSEWriter {
  readable: ReadableStream<Uint8Array>;
  write(event: string, data: unknown): void;
  close(): void;
  readonly closed: boolean;
}

const encoder = new TextEncoder();

export function sseEncode(event: string, data: unknown): Uint8Array {
  const serialized = typeof data === 'string' ? data : JSON.stringify(data);
  return encoder.encode(`event: ${event}\ndata: ${serialized}\n\n`);
}

export function sseStream(): SSEWriter {
  let controller: ReadableStreamDefaultController<Uint8Array> | undefined;
  let closed = false;

  const readable = new ReadableStream<Uint8Array>({
    start(c) { controller = c; },
    cancel() { closed = true; controller = undefined; },
  });

  return {
    readable,
    write(event, data) {
      if (closed || !controller) return;
      try {
        controller.enqueue(sseEncode(event, data));
      } catch {
        closed = true;
      }
    },
    close() {
      if (closed) return;
      closed = true;
      try { controller?.close(); } catch { /* already closed */ }
    },
    get closed() { return closed; },
  };
}
