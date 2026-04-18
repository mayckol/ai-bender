import { useEffect, useMemo, useRef, useState } from 'preact/hooks';

import { AgentFilter } from '../components/AgentFilter.tsx';
import { EventRow } from '../components/EventRow.tsx';
import { Layout } from '../components/Layout.tsx';
import { SessionAgentFlow } from '../components/LiveProjectFlow.tsx';
import { ProgressBar } from '../components/ProgressBar.tsx';
import { SegmentedToggle } from '../components/SegmentedToggle.tsx';
import { SessionAgentGraph } from '../components/SessionAgentGraph.tsx';
import {
  fetchSession,
  reportUrl,
  type BenderEvent,
  type SessionExport,
  type SessionState,
} from '../lib/api.ts';
import { distinctAgents, responsibleAgent } from '../lib/agents.ts';
import { subscribeSSE } from '../lib/sse.ts';

/**
 * Idempotency key for a single event. Two events with identical keys are
 * treated as duplicates and silently dropped at ingest. Intentionally
 * derived from fields whose combination uniquely identifies a real-world
 * emission: timestamp + type + actor name + any inline agent/skill hints.
 */
function eventKey(ev: BenderEvent): string {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  const agent = typeof p.agent === 'string' ? p.agent : '';
  const skill = typeof p.skill === 'string' ? p.skill : '';
  const decision = typeof p.decision_type === 'string' ? p.decision_type : '';
  const dispatched = Array.isArray(p.dispatched_agents) ? (p.dispatched_agents as unknown[]).join(',') : '';
  return `${ev.timestamp}|${ev.type}|${ev.actor?.kind ?? ''}|${ev.actor?.name ?? ''}|${agent}|${skill}|${decision}|${dispatched}`;
}

function dedupEvents(list: BenderEvent[]): { events: BenderEvent[]; seen: Set<string> } {
  const seen = new Set<string>();
  const out: BenderEvent[] = [];
  for (const ev of list) {
    const k = eventKey(ev);
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(ev);
  }
  return { events: out, seen };
}

interface Props { params: { id: string }; }

type TimelineView = 'flow' | 'graph' | 'logs';

export function SessionTimeline({ params }: Props) {
  const id = params.id;
  const [state, setState] = useState<SessionState | null>(null);
  const [events, setEvents] = useState<BenderEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [agentFilter, setAgentFilter] = useState<Set<string> | null>(null);
  // Idempotency — set of eventKey() hashes already ingested. Survives a
  // snapshot reset so streaming events arriving after the initial load
  // still dedupe against the snapshot.
  const eventSeenRef = useRef<Set<string>>(new Set());
  const [view, setView] = useState<TimelineView>('flow');
  const [effectiveStatus, setEffectiveStatus] = useState<string | null>(null);

  const displayStatus = effectiveStatus ?? state?.status;
  const frozen =
    displayStatus === 'completed' ||
    displayStatus === 'failed' ||
    displayStatus === 'awaiting_confirm';
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
      if (next.size === agents.length) return null;
      if (next.size === 0) return null;
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
              const { events: deduped, seen } = dedupEvents(snap.events);
              eventSeenRef.current = seen;
              setEvents(deduped);
              if (snap.effective_status) setEffectiveStatus(snap.effective_status);
            } catch (err) { setError(String(err)); }
          },
          event: (ev) => {
            try {
              const parsed = JSON.parse(ev.data) as BenderEvent;
              const key = eventKey(parsed);
              if (eventSeenRef.current.has(key)) return;
              eventSeenRef.current.add(key);
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
          const { events: deduped, seen } = dedupEvents(snap.events);
          eventSeenRef.current = seen;
          setEvents(deduped);
          if (snap.effective_status) setEffectiveStatus(snap.effective_status);
          setPending(false);
          setError(null);
          subscribe();
        })
        .catch((err) => {
          if (!mounted) return;
          const msg = String(err);
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
      <div class="toolbar-row">
        <a class="btn" href="/">← all stages</a>
        {reportAvailable && (
          <a class="btn" href={reportUrl(id)} target="_blank" rel="noopener">
            Open report ↗
          </a>
        )}
      </div>

      {error && (
        <div class="card" style={{ color: 'var(--err)' }}>Error: {error}</div>
      )}

      {pending && !state && !error && (
        <div class="card">
          <h2>Stage starting…</h2>
          <p class="muted-small">
            Waiting for <code>{id}</code> to appear under <code>.bender/sessions/</code>.
            This page will switch to the live telemetry the moment the first event lands.
          </p>
        </div>
      )}

      <section class="card card-telemetry">
        <header class="card-frame-head">
          <span class="card-frame-kicker">001</span>
          <h2>Telemetry</h2>
          <span class="card-frame-rule" />
        </header>
        <ProgressBar events={events} frozen={!!frozen} status={state?.status} />
      </section>


      {state && (
        <section class="card">
          <header class="card-frame-head">
            <span class="card-frame-kicker">002</span>
            <h2>Manifest</h2>
            <span class="card-frame-rule" />
          </header>
          <dl class="meta-grid">
            <dt>Command</dt><dd>{state.command}</dd>
            <dt>Status</dt>
            <dd>
              <span class={`status-pill ${displayStatus}`}>
                {displayStatus === 'awaiting_confirm' ? 'awaiting confirm' : displayStatus}
              </span>
            </dd>
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
        </section>
      )}

      <section class="card card-timeline">
        <header class="card-frame-head">
          <span class="card-frame-kicker">003</span>
          <h2>
            {view === 'flow' ? 'Stage flow' : view === 'graph' ? 'Stage graph' : 'Stage logs'}
            <span class="card-frame-count">
              {view === 'logs'
                ? ` · ${visibleEvents.length}${visibleEvents.length !== events.length ? ` of ${events.length}` : ''} event${events.length === 1 ? '' : 's'}`
                : ''}
            </span>
          </h2>
          <span class="card-frame-rule" />
          <SegmentedToggle
            ariaLabel="Timeline view"
            value={view}
            onChange={setView}
            options={[
              { value: 'flow',  label: 'Flow',  glyph: '◈' },
              { value: 'graph', label: 'Graph', glyph: '▤' },
              { value: 'logs',  label: 'Logs',  glyph: '≡' },
            ]}
          />
        </header>

        {view === 'logs' && (
          <>
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
          </>
        )}
        {view === 'flow'  && <SessionAgentFlow  events={events} state={state} />}
        {view === 'graph' && <SessionAgentGraph events={events} state={state} />}
      </section>
    </Layout>
  );
}
