import { stat } from 'node:fs/promises';
import { resolve } from 'node:path';

import {
  exportSession,
  listSessions,
  reportPath,
  sessionDir,
  sessionsRoot,
} from './sessions.ts';
import { sseStream } from './sse.ts';
import { tailSession, watchSessionsRoot } from './watcher.ts';

interface CliArgs {
  project?: string;
  port?: number;
}

function parseArgs(argv: string[]): CliArgs {
  const out: CliArgs = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a) continue;
    if (a === '--project' && argv[i + 1]) { out.project = argv[++i]; continue; }
    if (a.startsWith('--project=')) { out.project = a.slice('--project='.length); continue; }
    if (a === '--port' && argv[i + 1]) { out.port = Number(argv[++i]); continue; }
    if (a.startsWith('--port=')) { out.port = Number(a.slice('--port='.length)); continue; }
  }
  return out;
}

const args = parseArgs(Bun.argv.slice(2));
const projectRoot = resolve(
  args.project || process.env.BENDER_UI_PROJECT || process.cwd(),
);
const port = args.port ?? Number(process.env.BENDER_UI_PORT ?? 4317);

const CLIENT_DIR = resolve(import.meta.dir, '..', 'client');
const INDEX_HTML = resolve(CLIENT_DIR, 'index.html');
const STYLES_CSS = resolve(CLIENT_DIR, 'styles.css');
const CLIENT_ENTRY = resolve(CLIENT_DIR, 'main.tsx');

// Bundle the Preact client on startup. Kept in-memory; no watch rebuild in v1.
async function bundleClient() {
  const built = await Bun.build({
    entrypoints: [CLIENT_ENTRY],
    target: 'browser',
    minify: process.env.NODE_ENV === 'production',
  });
  if (!built.success || built.outputs.length === 0) {
    throw new Error(`client bundle failed: ${built.logs.map((l) => l.message).join('; ')}`);
  }
  return await built.outputs[0].text();
}

const clientBundle = await bundleClient();

const server = Bun.serve({
  port,
  idleTimeout: 0, // long-lived SSE
  async fetch(req) {
    const url = new URL(req.url);
    const path = url.pathname;

    if (path === '/api/sessions') {
      const list = await listSessions(projectRoot);
      return Response.json(list);
    }
    if (path === '/api/sessions/stream') {
      return sessionListStream(req);
    }

    const sessionMatch = /^\/api\/sessions\/([^/]+)(?:\/(stream|report))?$/.exec(path);
    if (sessionMatch) {
      const id = sessionMatch[1];
      const suffix = sessionMatch[2];
      const dir = sessionDir(projectRoot, id);
      try {
        const info = await stat(dir);
        if (!info.isDirectory()) throw new Error('not a directory');
      } catch {
        return new Response(`session not found: ${id}`, { status: 404 });
      }

      if (suffix === 'stream') {
        return sessionLiveStream(req, id, dir);
      }
      if (suffix === 'report') {
        const p = reportPath(projectRoot, id);
        const file = Bun.file(p);
        if (!(await file.exists())) {
          return new Response('report not found', { status: 404 });
        }
        return new Response(file, { headers: { 'content-type': 'text/markdown; charset=utf-8' } });
      }

      const snapshot = await exportSession(dir);
      return Response.json(snapshot);
    }

    if (path === '/client.js') {
      return new Response(clientBundle, {
        headers: { 'content-type': 'application/javascript; charset=utf-8' },
      });
    }
    if (path === '/styles.css') {
      return new Response(Bun.file(STYLES_CSS), {
        headers: { 'content-type': 'text/css; charset=utf-8' },
      });
    }

    // SPA shell — every other path serves the client entry HTML.
    return new Response(Bun.file(INDEX_HTML), {
      headers: { 'content-type': 'text/html; charset=utf-8' },
    });
  },
});

console.log(`bender-ui → http://localhost:${server.port}`);
console.log(`project   → ${projectRoot}`);
console.log(`sessions  → ${sessionsRoot(projectRoot)}`);

function sessionLiveStream(req: Request, id: string, dir: string): Response {
  const stream = sseStream();

  (async () => {
    try {
      const snapshot = await exportSession(dir);
      stream.write('snapshot', snapshot);
    } catch (err) {
      stream.write('error', { message: `failed to load session: ${String(err)}` });
      stream.close();
      return;
    }

    const stop = tailSession(
      dir,
      (ev) => {
        if (!stream.closed) stream.write('event', ev);
      },
      (state) => {
        if (!stream.closed) stream.write('state-patch', state);
      },
      (err) => {
        if (!stream.closed) stream.write('error', { message: String(err) });
      },
    );

    req.signal.addEventListener('abort', () => {
      stop();
      stream.close();
    });
  })();

  return new Response(stream.readable, {
    headers: sseHeaders(),
  });
}

function sessionListStream(req: Request): Response {
  const stream = sseStream();

  (async () => {
    try {
      stream.write('snapshot', await listSessions(projectRoot));
    } catch (err) {
      stream.write('error', { message: String(err) });
      stream.close();
      return;
    }

    const stop = watchSessionsRoot(
      projectRoot,
      (newId) => {
        if (!stream.closed) stream.write('session-added', { id: newId });
      },
      (err) => {
        if (!stream.closed) stream.write('error', { message: String(err) });
      },
    );

    req.signal.addEventListener('abort', () => {
      stop();
      stream.close();
    });
  })();

  return new Response(stream.readable, {
    headers: sseHeaders(),
  });
}

function sseHeaders(): HeadersInit {
  return {
    'content-type': 'text/event-stream; charset=utf-8',
    'cache-control': 'no-cache, no-transform',
    'connection': 'keep-alive',
    'x-accel-buffering': 'no',
  };
}
