import { agentColor } from '../lib/agents.ts';

interface Props {
  agents: string[];
  active: Set<string> | null; // null = all
  counts: Map<string, number>;
  onToggle: (agent: string) => void;
  onClear: () => void;
}

export function AgentFilter({ agents, active, counts, onToggle, onClear }: Props) {
  if (agents.length <= 1) return null;
  return (
    <div class="agent-filter">
      <span class="agent-filter-label">Agents:</span>
      {agents.map((a) => {
        const color = agentColor(a);
        const isActive = active === null || active.has(a);
        const count = counts.get(a) ?? 0;
        return (
          <button
            key={a}
            type="button"
            class={`agent-chip${isActive ? ' active' : ''}`}
            style={isActive
              ? { background: `${color}2a`, borderColor: color, color }
              : { borderColor: 'var(--border)' }}
            onClick={() => onToggle(a)}
            title={`Toggle ${a} (${count} events)`}
          >
            <span class="chip-dot" style={{ background: color }} />
            {a}
            <span class="chip-count">{count}</span>
          </button>
        );
      })}
      {active !== null && (
        <button type="button" class="agent-chip clear" onClick={onClear}>
          clear filter
        </button>
      )}
    </div>
  );
}
