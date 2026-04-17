import type { BenderEvent, SessionState, SessionSummary } from './api.ts';

export type NodeState =
  | 'disabled'
  | 'running'
  | 'completed'
  | 'failed'
  | 'blocked';

/**
 * A single node in the live project flow. Nodes are keyed by agent name so
 * repeated invocations of the same agent collapse to one cell. The two
 * permanent anchors — `crafter` (entry) and `ship` (terminal) — are always
 * rendered; everything between them is discovered dynamically from the
 * running session's events stream.
 *
 * `waveId` groups nodes dispatched in the same `orchestrator_decision(parallel_dispatch)`
 * event. The renderer uses it to stack parallel nodes vertically inside one
 * horizontal cell.
 */
export interface FlowNode {
  id: string;
  agent: string;
  skill?: string;
  state: NodeState;
  isAnchor?: boolean;
  startedAt?: string;
  completedAt?: string;
  waveId: string;
}

/**
 * A wave = one horizontal "slot" in the flow chain. If `nodes.length > 1`,
 * the slot renders them as a vertical parallel stack.
 */
export interface FlowWave {
  id: string;
  nodes: FlowNode[];
  parallel: boolean;
}

const ANCHOR_CRAFTER_WAVE = 'wave-anchor-crafter';
const ANCHOR_SHIP_WAVE = 'wave-anchor-ship';

const ANCHOR_CRAFTER: FlowNode = {
  id: 'anchor-crafter',
  agent: 'crafter',
  state: 'disabled',
  isAnchor: true,
  waveId: ANCHOR_CRAFTER_WAVE,
};

const ANCHOR_SHIP: FlowNode = {
  id: 'anchor-ship',
  agent: 'ship',
  state: 'disabled',
  isAnchor: true,
  waveId: ANCHOR_SHIP_WAVE,
};

export function pickLiveGhu(sessions: SessionSummary[]): SessionSummary | null {
  const ghus = sessions.filter(
    (s) => s.state.command === '/ghu' || s.state.command === '/implement',
  );
  const running = ghus.find((s) => s.state.status === 'running');
  if (running) return running;
  const sorted = [...ghus].sort((a, b) => b.id.localeCompare(a.id));
  return sorted[0] ?? null;
}

/**
 * Fold an event stream into the ordered wave list. Parallelism is made
 * explicit: every `orchestrator_decision(parallel_dispatch)` event opens a
 * shared wave for its `dispatched_agents`; every solo `agent_assignment`
 * (or an `agent_started` for an agent not in a prior parallel batch) gets
 * its own single-member wave.
 */
export function buildFlowWaves(
  events: BenderEvent[],
  state: SessionState | null,
  isLive: boolean,
): FlowWave[] {
  const agentWave = new Map<string, string>();
  const waveParallel = new Map<string, boolean>();
  const waveOrder: string[] = [];
  const waveMembers = new Map<string, Set<string>>();
  let waveCounter = 0;

  const seedWave = (id: string, parallel: boolean) => {
    if (waveMembers.has(id)) return;
    waveOrder.push(id);
    waveMembers.set(id, new Set());
    waveParallel.set(id, parallel);
  };

  // First pass — assign wave membership from orchestrator decisions so the
  // parallel signal survives any event re-ordering.
  for (const ev of events) {
    if (ev.type !== 'orchestrator_decision') continue;
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const kind = typeof p.decision_type === 'string' ? p.decision_type : '';

    if (kind === 'parallel_dispatch') {
      const agents = pickStringArray(p, ['dispatched_agents']);
      if (agents.length === 0) continue;
      const waveId = `wave-p-${++waveCounter}`;
      seedWave(waveId, true);
      for (const a of agents) {
        if (!agentWave.has(a)) agentWave.set(a, waveId);
        waveMembers.get(waveId)!.add(a);
      }
    } else if (kind === 'agent_assignment') {
      const agent = typeof p.dispatched_agent === 'string' ? p.dispatched_agent : '';
      if (!agent) continue;
      if (!agentWave.has(agent)) {
        const waveId = `wave-s-${++waveCounter}`;
        seedWave(waveId, false);
        agentWave.set(agent, waveId);
        waveMembers.get(waveId)!.add(agent);
      }
    }
  }

  // Second pass — build per-agent FlowNode state from agent_* events.
  const byAgent = new Map<string, FlowNode>();
  const appearOrder: string[] = [];

  const ensureNode = (agent: string, ts: string): FlowNode => {
    let node = byAgent.get(agent);
    if (node) return node;
    // Agent appeared without a prior orchestrator_decision — give it its
    // own single-member wave so the chain still renders.
    if (!agentWave.has(agent)) {
      const waveId = `wave-s-${++waveCounter}`;
      seedWave(waveId, false);
      agentWave.set(agent, waveId);
      waveMembers.get(waveId)!.add(agent);
    }
    node = {
      id: `node-${agent}`,
      agent,
      state: 'running',
      startedAt: ts,
      waveId: agentWave.get(agent)!,
    };
    byAgent.set(agent, node);
    appearOrder.push(agent);
    return node;
  };

  for (const ev of events) {
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const agent = typeof p.agent === 'string' ? p.agent : '';
    if (!agent) continue;

    if (ev.type === 'agent_started') {
      const node = ensureNode(agent, ev.timestamp);
      node.state = 'running';
      if (!node.startedAt) node.startedAt = ev.timestamp;
    } else if (ev.type === 'agent_completed') {
      const node = ensureNode(agent, ev.timestamp);
      node.state = 'completed';
      node.completedAt = ev.timestamp;
    } else if (ev.type === 'agent_failed' || ev.type === 'skill_failed') {
      const node = ensureNode(agent, ev.timestamp);
      node.state = 'failed';
    } else if (ev.type === 'agent_blocked') {
      const node = ensureNode(agent, ev.timestamp);
      node.state = 'blocked';
    } else if (ev.type === 'skill_invoked' && !byAgent.has(agent)) {
      const node = ensureNode(agent, ev.timestamp);
      const skill = typeof p.skill === 'string' ? p.skill : undefined;
      if (skill) node.skill = skill;
    } else if (ev.type === 'skill_invoked') {
      const node = byAgent.get(agent)!;
      const skill = typeof p.skill === 'string' ? p.skill : undefined;
      if (skill && !node.skill) node.skill = skill;
    }
  }

  // Third pass — assemble waves in discovery order, collapsing to the
  // anchor waves at the ends.
  const dynamicWaves: FlowWave[] = [];
  const seenWaves = new Set<string>();

  for (const waveId of waveOrder) {
    const members = waveMembers.get(waveId)!;
    const nodes: FlowNode[] = [];
    for (const agent of members) {
      const node = byAgent.get(agent);
      if (node) nodes.push(node);
    }
    if (nodes.length === 0) continue;
    nodes.sort((a, b) => a.agent.localeCompare(b.agent));
    dynamicWaves.push({ id: waveId, nodes, parallel: !!waveParallel.get(waveId) && nodes.length > 1 });
    seenWaves.add(waveId);
  }

  // Anchor handling: if a real `crafter` node landed, promote it to the
  // head anchor (still inside its original wave so parallel grouping is
  // preserved). Otherwise render the placeholder.
  const crafterNode = byAgent.get('crafter');
  let headWave: FlowWave;
  let middleWaves = dynamicWaves;
  if (crafterNode) {
    // Pull crafter's wave out and mark it the head.
    const idx = dynamicWaves.findIndex((w) => w.id === crafterNode.waveId);
    if (idx !== -1) {
      const waveCopy: FlowWave = {
        ...dynamicWaves[idx],
        nodes: dynamicWaves[idx].nodes.map((n) =>
          n.agent === 'crafter' ? { ...n, isAnchor: true } : n,
        ),
      };
      headWave = waveCopy;
      middleWaves = [...dynamicWaves.slice(0, idx), ...dynamicWaves.slice(idx + 1)];
    } else {
      headWave = anchorWave('crafter', 'disabled');
    }
  } else {
    headWave = anchorWave('crafter', 'disabled');
  }

  // Ship anchor state follows the session's terminal status.
  let shipState: NodeState = 'disabled';
  if (state?.status === 'completed' || state?.status === 'awaiting_confirm') {
    shipState = 'completed';
  } else if (state?.status === 'failed') {
    shipState = 'failed';
  }
  if (isLive && state?.status !== 'completed' && state?.status !== 'failed') {
    shipState = 'disabled';
  }
  const shipWave = anchorWave('ship', shipState);

  return [headWave, ...middleWaves, shipWave];
}

function anchorWave(which: 'crafter' | 'ship', state: NodeState): FlowWave {
  const anchor = which === 'crafter' ? { ...ANCHOR_CRAFTER } : { ...ANCHOR_SHIP };
  anchor.state = state;
  return {
    id: anchor.waveId,
    nodes: [anchor],
    parallel: false,
  };
}

function pickStringArray(payload: Record<string, unknown>, keys: string[]): string[] {
  for (const key of keys) {
    const v = payload[key];
    if (Array.isArray(v)) {
      return v.filter((x): x is string => typeof x === 'string');
    }
  }
  return [];
}
