import { watch, type FSWatcher } from 'node:fs';
import { open, stat } from 'node:fs/promises';
import { join } from 'node:path';

import type { BenderEvent, SessionState } from './schema.ts';
import { parseEventsJSONL, readState, sessionsRoot } from './sessions.ts';

/**
 * Tails events.jsonl in a session directory. On each change event, reads
 * from the last-known byte offset to EOF, parses any complete JSON lines,
 * and emits them in order via `onEvent`. State writes (state.json rewrites)
 * trigger `onState`.
 *
 * Returns a stop function that closes the watcher.
 */
export function tailSession(
  dir: string,
  onEvent: (ev: BenderEvent) => void,
  onState: (state: SessionState) => void,
  onError?: (err: unknown) => void,
): () => void {
  let offset = 0;
  let reading = false;
  let pendingRead = false;
  let carry = '';

  const eventsPath = join(dir, 'events.jsonl');

  async function drain() {
    if (reading) { pendingRead = true; return; }
    reading = true;
    try {
      let info;
      try {
        info = await stat(eventsPath);
      } catch (err) {
        if ((err as NodeJS.ErrnoException).code === 'ENOENT') return;
        throw err;
      }
      if (info.size < offset) {
        // File was truncated / recreated. Restart from zero.
        offset = 0;
        carry = '';
      }
      if (info.size <= offset) return;

      const fh = await open(eventsPath, 'r');
      try {
        const buf = Buffer.alloc(info.size - offset);
        await fh.read(buf, 0, buf.length, offset);
        offset = info.size;
        const chunk = carry + buf.toString('utf8');
        const lastNewline = chunk.lastIndexOf('\n');
        const complete = lastNewline === -1 ? '' : chunk.slice(0, lastNewline + 1);
        carry = lastNewline === -1 ? chunk : chunk.slice(lastNewline + 1);
        if (complete) {
          for (const ev of parseEventsJSONL(complete)) onEvent(ev);
        }
      } finally {
        await fh.close();
      }
    } catch (err) {
      onError?.(err);
    } finally {
      reading = false;
      if (pendingRead) {
        pendingRead = false;
        void drain();
      }
    }
  }

  async function readStateSafe() {
    try {
      const s = await readState(dir);
      onState(s);
    } catch (err) {
      onError?.(err);
    }
  }

  // Initial drain so clients get current tail position if they subscribe
  // after some events were already written.
  void drain();
  void readStateSafe();

  let watcher: FSWatcher | null = null;
  try {
    watcher = watch(dir, { persistent: true }, (_evt, filename) => {
      if (!filename) { void drain(); void readStateSafe(); return; }
      if (filename === 'events.jsonl') void drain();
      else if (filename === 'state.json') void readStateSafe();
    });
  } catch (err) {
    onError?.(err);
  }

  return () => {
    try { watcher?.close(); } catch { /* noop */ }
  };
}

/**
 * Watches the sessions root for newly created session directories. Emits the
 * new directory id via `onAdded`.
 */
export function watchSessionsRoot(
  projectRoot: string,
  onAdded: (id: string) => void,
  onError?: (err: unknown) => void,
): () => void {
  const root = sessionsRoot(projectRoot);
  const seen = new Set<string>();

  let watcher: FSWatcher | null = null;
  try {
    watcher = watch(root, { persistent: true }, async (_evt, filename) => {
      if (!filename) return;
      const id = String(filename);
      if (seen.has(id)) return;
      try {
        const info = await stat(join(root, id));
        if (info.isDirectory()) {
          seen.add(id);
          onAdded(id);
        }
      } catch (err) {
        onError?.(err);
      }
    });
  } catch (err) {
    onError?.(err);
  }

  return () => {
    try { watcher?.close(); } catch { /* noop */ }
  };
}
