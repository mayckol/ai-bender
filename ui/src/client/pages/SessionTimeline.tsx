import { useEffect, useMemo, useState } from 'preact/hooks';

import { EventRow } from '../components/EventRow.tsx';
import { FindingsPanel } from '../components/FindingsPanel.tsx';
import { Layout } from '../components/Layout.tsx';
import {
  fetchSession,
  reportUrl,
  type BenderEvent,
  type SessionExport,
  type SessionState,
} from '../lib/api.ts';
import { subscribeSSE } from '../lib/sse.ts';

interface Props { params: { id: string }; }

export function SessionTimeline({ params }: Props) {
  const id = params.id;
  const [state, setState] = useState<SessionState | null>(null);
  const [events, setEvents] = useState<BenderEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  const frozen = state?.status === 'completed' || state?.status === 'failed';
  const liveClass = frozen ? 'frozen' : connected ? 'connected' : '';

  useEffect(() => {
    let mounted = true;
    fetchSession(id)
      .then((snap: SessionExport) => {
        if (!mounted) return;
        setState(snap.state);
        setEvents(snap.events);
      })
      .catch((err) => { if (mounted) setError(String(err)); });

    const stop = subscribeSSE(`/api/sessions/${encodeURIComponent(id)}/stream`, {
      onOpen: () => setConnected(true),
      onError: () => setConnected(false),
      handlers: {
        snapshot: (ev) => {
          try {
            const snap = JSON.parse(ev.data) as SessionExport;
            setState(snap.state);
            setEvents(snap.events);
          } catch (err) { setError(String(err)); }
        },
        event: (ev) => {
          try {
            const parsed = JSON.parse(ev.data) as BenderEvent;
            setEvents((prev) => [...prev, parsed]);
          } catch (err) { setError(String(err)); }
        },
        'state-patch': (ev) => {
          try { setState(JSON.parse(ev.data) as SessionState); }
          catch (err) { setError(String(err)); }
        },
        error: (ev) => {
          try { setError((JSON.parse(ev.data) as { message: string }).message); }
          catch { setError(ev.data); }
        },
      },
    });

    return () => { mounted = false; stop(); };
  }, [id]);

  const reportAvailable = useMemo(
    () => events.some((e) => e.type === 'artifact_written' &&
      String((e.payload as any)?.path ?? '').endsWith('-report.md')),
    [events],
  );

  return (
    <Layout
      title={id}
      breadcrumb={state?.command ?? '/?'}
      right={
        <span class={`live-indicator ${liveClass}`}>
          <span class="dot" />
          {frozen ? 'final' : connected ? 'live' : 'offline'}
        </span>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <a class="btn" href="/">← all sessions</a>
        {reportAvailable && (
          <a class="btn" href={reportUrl(id)} target="_blank" rel="noopener" style={{ marginLeft: 8 }}>
            Open report ↗
          </a>
        )}
      </div>

      {error && (
        <div class="card" style={{ color: 'var(--err)' }}>Error: {error}</div>
      )}

      <div class="layout">
        <div>
          {state && (
            <div class="card">
              <h2>Session</h2>
              <dl class="meta-grid">
                <dt>Command</dt><dd>{state.command}</dd>
                <dt>Status</dt><dd><span class={`status-pill ${state.status}`}>{state.status}</span></dd>
                <dt>Started</dt><dd>{state.started_at}</dd>
                {state.completed_at && <><dt>Completed</dt><dd>{state.completed_at}</dd></>}
                {state.source_artifacts && state.source_artifacts.length > 0 && (
                  <>
                    <dt>Sources</dt>
                    <dd>{state.source_artifacts.join(', ')}</dd>
                  </>
                )}
                <dt>Files changed</dt><dd>{state.files_changed ?? 0}</dd>
                <dt>Findings</dt><dd>{state.findings_count ?? 0}</dd>
              </dl>
            </div>
          )}

          <div class="card">
            <h2>Timeline ({events.length} events)</h2>
            <div class="event-log">
              {events.length === 0 ? (
                <div class="empty">Waiting for events…</div>
              ) : (
                events.map((ev, i) => <EventRow key={i} event={ev} />)
              )}
            </div>
          </div>
        </div>

        <div>
          <FindingsPanel events={events} />
        </div>
      </div>
    </Layout>
  );
}
