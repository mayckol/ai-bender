import { useEffect, useMemo, useState } from 'preact/hooks';

import { Layout } from '../components/Layout.tsx';
import { LiveProjectFlow } from '../components/LiveProjectFlow.tsx';
import { SegmentedToggle } from '../components/SegmentedToggle.tsx';
import { agentColor } from '../lib/agents.ts';
import {
  deleteAllSessions,
  deleteSession,
  fetchSessions,
  type SessionSummary,
} from '../lib/api.ts';
import { subscribeSSE } from '../lib/sse.ts';

type View = 'list' | 'flow';

export function SessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [view, setView] = useState<View>('flow');

  useEffect(() => {
    let mounted = true;
    fetchSessions()
      .then((list) => { if (mounted) setSessions(list); })
      .catch((err) => { if (mounted) setError(String(err)); });

    const stop = subscribeSSE('/api/sessions/stream', {
      onOpen: () => setConnected(true),
      onError: () => setConnected(false),
      handlers: {
        snapshot: (ev) => {
          try { setSessions(JSON.parse(ev.data) as SessionSummary[]); }
          catch (err) { setError(String(err)); }
        },
      },
    });

    return () => { mounted = false; stop(); };
  }, []);

  async function onDelete(id: string) {
    if (!window.confirm(`Remove stage ${id}?\n\nThe events.jsonl, state.json, and this stage's scout cache will be deleted. Artifacts under .bender/artifacts/ are preserved.`)) return;
    setBusy(id);
    try {
      await deleteSession(id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  async function onClearAll() {
    if (sessions.length === 0) return;
    if (!window.confirm(`Remove ALL ${sessions.length} stage(s)?\n\nEvery stage directory and the whole scout cache will be deleted. Artifacts under .bender/artifacts/ are preserved. This cannot be undone.`)) return;
    setBusy('all');
    try {
      await deleteAllSessions();
      setSessions([]);
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  return (
    <Layout
      title="bender — stages"
      right={
        <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
          <SegmentedToggle
            ariaLabel="stages view"
            value={view}
            onChange={setView}
            options={[
              { value: 'flow', label: 'Flow', glyph: '◈' },
              { value: 'list', label: 'List', glyph: '≡' },
            ]}
          />
          {sessions.length > 0 && (
            <button
              type="button"
              class="btn danger"
              disabled={busy !== null}
              onClick={onClearAll}
              title="Delete every stage directory + the scout cache"
            >
              {busy === 'all' ? 'Clearing…' : `Clear all (${sessions.length})`}
            </button>
          )}
          <LiveIndicator connected={connected} />
        </div>
      }
    >
      {error && <div class="card" style={{ color: 'var(--err)' }}>Error: {error}</div>}

      {view === 'flow' ? (
        <section class="card card-projflow">
          <header class="card-frame-head">
            <span class="card-frame-kicker">pipeline</span>
            <h2>Project flow</h2>
            <span class="card-frame-rule" />
          </header>
          <LiveProjectFlow sessions={sessions} />
        </section>
      ) : (
        <div class="card" style={{ padding: 0, overflow: 'hidden' }}>
          {sessions.length === 0 ? (
            <div class="empty">No stages yet. Run a slash command in this project.</div>
          ) : (
            <table class="sessions">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Command</th>
                  <th>Status</th>
                  <th>Agents</th>
                  <th>Skills</th>
                  <th>Started</th>
                  <th>Duration</th>
                  <th>Files</th>
                  <th aria-label="actions" />
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => (
                  <StageRow
                    key={s.id}
                    session={s}
                    busy={busy}
                    onDelete={() => onDelete(s.id)}
                  />
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </Layout>
  );
}

interface StageRowProps {
  session: SessionSummary;
  busy: string | null;
  onDelete: () => void;
}

function StageRow({ session: s, busy, onDelete }: StageRowProps) {
  const status = s.effective_status ?? s.state.status;
  const isRunning = s.state.status === 'running';
  return (
    <tr class={`stage-row${isRunning ? ' stage-row-live' : ''} stage-row-${status}`}>
      <td><a href={`/sessions/${s.id}`}>{s.id}</a></td>
      <td>{s.state.command}</td>
      <td><StatusPill status={status} running={isRunning} /></td>
      <td><AgentCell agents={s.agents ?? []} /></td>
      <td><SkillCell skills={s.skills ?? []} /></td>
      <td>{s.state.started_at}</td>
      <td><DurationCell durationMs={s.duration_ms} session={s} running={isRunning} /></td>
      <td>{s.state.files_changed ?? 0}</td>
      <td class="row-actions">
        <button
          type="button"
          class="row-delete"
          disabled={busy !== null}
          onClick={onDelete}
          title={`Remove ${s.id}`}
          aria-label={`Remove ${s.id}`}
        >
          {busy === s.id ? '…' : '×'}
        </button>
      </td>
    </tr>
  );
}

function StatusPill({ status, running }: { status: string; running: boolean }) {
  const label = status === 'awaiting_confirm' ? 'awaiting confirm' : status;
  return (
    <span class={`status-pill ${status}${running ? ' is-live' : ''}`}>
      {running && <span class="status-pill-heart" aria-hidden="true" />}
      {label}
    </span>
  );
}

function DurationCell({ durationMs, session: s, running }: { durationMs: number; session: SessionSummary; running: boolean }) {
  if (!running) return <>{fmtDuration(durationMs)}</>;
  return <LiveDuration startedAt={s.state.started_at} />;
}

function LiveDuration({ startedAt }: { startedAt: string }) {
  const start = useMemo(() => Date.parse(startedAt) || Date.now(), [startedAt]);
  const [, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((x) => x + 1), 1000);
    return () => clearInterval(id);
  }, []);
  return <span class="live-duration">{fmtDuration(Date.now() - start)}</span>;
}

function AgentCell({ agents }: { agents: string[] }) {
  if (agents.length === 0) return <span class="muted-small">—</span>;
  const visible = agents.slice(0, 4);
  const remainder = agents.length - visible.length;
  return (
    <span class="agent-cell">
      {visible.map((a) => {
        const color = agentColor(a);
        return (
          <span
            key={a}
            class="agent-mini"
            style={{ background: `${color}22`, color, borderColor: `${color}55` }}
          >
            {a}
          </span>
        );
      })}
      {remainder > 0 && <span class="agent-mini muted">+{remainder}</span>}
    </span>
  );
}

function SkillCell({ skills }: { skills: string[] }) {
  if (skills.length === 0) return <span class="muted-small">—</span>;
  return (
    <span class="skill-cell" title={skills.join('\n')}>
      {skills.length} {skills.length === 1 ? 'skill' : 'skills'}
    </span>
  );
}

function LiveIndicator({ connected }: { connected: boolean }) {
  return (
    <span class={`live-indicator ${connected ? 'connected' : ''}`}>
      <span class="dot" />
      {connected ? 'live' : 'offline'}
    </span>
  );
}

function fmtDuration(ms: number): string {
  if (!ms || ms < 0) return '—';
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.floor(s - m * 60);
  return `${m}m${rem}s`;
}
