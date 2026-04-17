import { useEffect, useMemo, useState } from 'preact/hooks';

import { AgentFilter } from '../components/AgentFilter.tsx';
import { EventRow } from '../components/EventRow.tsx';
import { Layout } from '../components/Layout.tsx';
import { ProgressBar } from '../components/ProgressBar.tsx';
import {
  fetchSession,
  reportUrl,
  type BenderEvent,
  type SessionExport,
  type SessionState,
} from '../lib/api.ts';
import { distinctAgents, responsibleAgent } from '../lib/agents.ts';
import { subscribeSSE } from '../lib/sse.ts';

interface Props { params: { id: string }; }

export function SessionTimeline({ params }: Props) {
  const id = params.id;
  const [state, setState] = useState<SessionState | null>(null);
  const [events, setEvents] = useState<BenderEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [agentFilter, setAgentFilter] = useState<Set<string> | null>(null);

  const frozen = state?.status === 'completed' || state?.status === 'failed';
  const liveClass = frozen ? 'frozen' : connected ? 'connected' : '';

  const agents = useMemo(() => distinctAgents(events), [events]);
  const agentCounts = useMemo(() => {
    const m = new Map<string, number>();
    for (const ev of events) {
      const a = responsibleAgent(ev);
      m.set(a, (m.get(a) ?? 0) + 1);
    }
    return m;
  }, [events]);
  const visibleEvents = useMemo(() => {
    if (agentFilter === null || agentFilter.size === 0) return events;
    return events.filter((ev) => agentFilter.has(responsibleAgent(ev)));
  }, [events, agentFilter]);

  function toggleAgent(a: string) {
    setAgentFilter((prev) => {
      const next = new Set(prev ?? agents);
      if (next.has(a)) next.delete(a);
      else next.add(a);
      if (next.size === agents.length) return null; // all selected → no filter
      if (next.size === 0) return null;             // none selected → show all
      return next;
    });
  }

  const [pending, setPending] = useState(true);

  useEffect(() => {
    let mounted = true;
    let stopSSE: (() => void) | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;

    function subscribe() {
      stopSSE = subscribeSSE(`/api/sessions/${encodeURIComponent(id)}/stream`, {
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
    }

    function load() {
      if (!mounted) return;
      fetchSession(id)
        .then((snap: SessionExport) => {
          if (!mounted) return;
          setState(snap.state);
          setEvents(snap.events);
          setPending(false);
          setError(null);
          subscribe();
        })
        .catch((err) => {
          if (!mounted) return;
          const msg = String(err);
          // Treat "session not found" as a pending session (race between
          // Chrome opening and the dispatcher seeding the directory) and
          // retry every 500ms for up to ~30s before surfacing it.
          if (/not\s*found|404/i.test(msg)) {
            setPending(true);
            retryTimer = setTimeout(load, 500);
            return;
          }
          setError(msg);
          setPending(false);
        });
    }

    load();

    return () => {
      mounted = false;
      if (retryTimer) clearTimeout(retryTimer);
      if (stopSSE) stopSSE();
    };
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

      {pending && !state && !error && (
        <div class="card">
          <h2>Session starting…</h2>
          <p class="muted-small">
            Waiting for <code>{id}</code> to appear under <code>.bender/sessions/</code>.
            This page will switch to the live timeline the moment the first event lands.
          </p>
        </div>
      )}

      <div class="card">
        <h2>Progress</h2>
        <ProgressBar events={events} frozen={!!frozen} />
      </div>

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
          </dl>
        </div>
      )}

      <div class="card">
        <h2>
          Timeline ({visibleEvents.length}
          {visibleEvents.length !== events.length && <> of {events.length}</>} events)
        </h2>
        <AgentFilter
          agents={agents}
          active={agentFilter}
          counts={agentCounts}
          onToggle={toggleAgent}
          onClear={() => setAgentFilter(null)}
        />
        <div class="event-log">
          {visibleEvents.length === 0 ? (
            <div class="empty">
              {events.length === 0 ? 'Waiting for events…' : 'No events match the current filter.'}
            </div>
          ) : (
            visibleEvents.map((ev, i) => <EventRow key={i} event={ev} />)
          )}
        </div>
      </div>
    </Layout>
  );
}
