import { useEffect, useState } from 'preact/hooks';

import { Layout } from '../components/Layout.tsx';
import { agentColor } from '../lib/agents.ts';
import { fetchSessions, type SessionSummary } from '../lib/api.ts';
import { subscribeSSE } from '../lib/sse.ts';

export function SessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

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
        'session-added': () => {
          // Cheap re-fetch on new-session notifications rather than an
          // incremental patch — the list is tiny.
          fetchSessions().then(setSessions).catch((err) => setError(String(err)));
        },
      },
    });

    return () => { mounted = false; stop(); };
  }, []);

  return (
    <Layout
      title="bender — sessions"
      right={<LiveIndicator connected={connected} />}
    >
      {error && <div class="card" style={{ color: 'var(--err)' }}>Error: {error}</div>}
      <div class="card" style={{ padding: 0, overflow: 'hidden' }}>
        {sessions.length === 0 ? (
          <div class="empty">No sessions yet. Run a slash command in this project.</div>
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
                <th>Findings</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((s) => (
                <tr key={s.id}>
                  <td><a href={`/sessions/${s.id}`}>{s.id}</a></td>
                  <td>{s.state.command}</td>
                  <td><span class={`status-pill ${s.state.status}`}>{s.state.status}</span></td>
                  <td><AgentCell agents={s.agents ?? []} /></td>
                  <td><SkillCell skills={s.skills ?? []} /></td>
                  <td>{s.state.started_at}</td>
                  <td>{fmtDuration(s.duration_ms)}</td>
                  <td>{s.state.files_changed ?? 0}</td>
                  <td>{s.state.findings_count ?? 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Layout>
  );
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
