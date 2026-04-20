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
  isBaseStage?: boolean;
  sessionId?: string;
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

export type BaseStageId = 'cry' | 'plan' | 'tdd';

const BASE_STAGES: BaseStageId[] = ['cry', 'plan', 'tdd'];

export interface BaseStageInfo {
  state: NodeState;
  sessionId?: string;
  startedAt?: string;
  completedAt?: string;
}

export type BaseStageStates = Record<BaseStageId, BaseStageInfo>;

const EMPTY_BASE_STAGES: BaseStageStates = {
  cry: { state: 'disabled' },
  plan: { state: 'disabled' },
  tdd: { state: 'disabled' },
};

function commandHead(cmd: string | undefined | null): string {
  if (!cmd) return '';
  return cmd.trim().split(/\s+/)[0];
}

/**
 * Collapse the session list into one state per base stage. We use the
 * latest session per command (by id, which is timestamp-prefixed) as the
 * authoritative record: a running run supersedes a prior completed one.
 */
export function pickBaseStageStates(sessions: SessionSummary[]): BaseStageStates {
  const out: BaseStageStates = {
    cry: { state: 'disabled' },
    plan: { state: 'disabled' },
    tdd: { state: 'disabled' },
  };
  const latest = new Map<BaseStageId, SessionSummary>();
  for (const s of sessions) {
    const head = commandHead(s.state.command);
    const id = head.startsWith('/') ? (head.slice(1) as BaseStageId) : null;
    if (!id || !BASE_STAGES.includes(id)) continue;
    const prev = latest.get(id);
    if (!prev || s.id.localeCompare(prev.id) > 0) latest.set(id, s);
  }
  for (const [id, s] of latest) {
    out[id] = {
      state: stateFromStatus(s.effective_status ?? s.state.status),
      sessionId: s.id,
      startedAt: s.state.started_at,
      completedAt: s.state.completed_at,
    };
  }
  return out;
}

function stateFromStatus(status: string | undefined): NodeState {
  switch (status) {
    case 'running': return 'running';
    case 'completed':
    case 'awaiting_confirm': return 'completed';
    case 'failed': return 'failed';
    default: return 'disabled';
  }
}

function baseStageWave(id: BaseStageId, info: BaseStageInfo): FlowWave {
  const waveId = `wave-anchor-${id}`;
  const node: FlowNode = {
    id: `anchor-${id}`,
    agent: id,
    state: info.state,
    isAnchor: true,
    isBaseStage: true,
    sessionId: info.sessionId,
    startedAt: info.startedAt,
    completedAt: info.completedAt,
    waveId,
  };
  return { id: waveId, nodes: [node], parallel: false };
}

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
 * Feature 007: Pick the workflow id the live view should subscribe to.
 *
 * Preference order:
 *   1. Newest running session's workflow_id (TDD + /ghu share one when
 *      linked on the same feature branch).
 *   2. Newest completed /ghu session's workflow_id, as a fallback so the
 *      view still stitches its immediate predecessors.
 *
 * Returns null when no session carries a workflow_id — callers fall back to
 * `pickLiveGhu` and subscribe to a single session instead.
 */
export function pickLiveWorkflowId(sessions: SessionSummary[]): string | null {
  const withWorkflow = sessions.filter((s) => !!s.state.workflow_id);
  if (withWorkflow.length === 0) return null;

  const running = withWorkflow.find((s) => s.state.status === 'running');
  if (running?.state.workflow_id) return running.state.workflow_id;

  const sorted = [...withWorkflow].sort((a, b) => b.id.localeCompare(a.id));
  return sorted[0]?.state.workflow_id ?? null;
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
  baseStages: BaseStageStates = EMPTY_BASE_STAGES,
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
  const seenDispatchSignatures = new Set<string>();
  for (const ev of events) {
    if (ev.type !== 'orchestrator_decision') continue;
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const kind = typeof p.decision_type === 'string' ? p.decision_type : '';

    if (kind === 'parallel_dispatch') {
      const agents = pickStringArray(p, ['dispatched_agents']);
      if (agents.length === 0) continue;
      // Idempotency — two decisions with the same sorted member set are
      // the same dispatch re-emitted; don't create a second wave.
      const sig = [...agents].sort().join('|');
      if (seenDispatchSignatures.has(sig)) continue;
      seenDispatchSignatures.add(sig);
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

  const baseWaves = BASE_STAGES.map((id) => baseStageWave(id, baseStages[id]));

  return [...baseWaves, headWave, ...middleWaves, shipWave];
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

/**
 * Session-scoped wave builder. One node per `skill_invoked` event — the
 * node represents a single agent × skill invocation, not a collapsed
 * agent. No `crafter` / `ship` anchors (the session page shows a complete
 * story; anchors would be noise).
 *
 * Parallel grouping: a `parallel_dispatch` orchestrator_decision opens a
 * window whose `dispatched_agents` share a wave. The window closes when
 * every member has emitted a terminal event. Skills invoked while the
 * window is open AND whose agent is a member fall into that wave;
 * everything else gets its own solo wave.
 */
export function buildSessionWaves(events: BenderEvent[], _state: SessionState | null): FlowWave[] {
  interface OpenInvocation {
    node: FlowNode;
    skill: string;
  }

  interface ParallelWindow {
    waveId: string;
    remaining: Set<string>;
  }

  const waves: FlowWave[] = [];
  const waveIndex = new Map<string, FlowWave>();
  const openByAgent = new Map<string, OpenInvocation[]>();
  let parallel: ParallelWindow | null = null;
  let pCounter = 0;
  let sCounter = 0;
  let invocCounter = 0;

  const ensureWave = (waveId: string, isParallel: boolean): FlowWave => {
    let w = waveIndex.get(waveId);
    if (!w) {
      w = { id: waveId, nodes: [], parallel: isParallel };
      waveIndex.set(waveId, w);
      waves.push(w);
    }
    return w;
  };

  for (const ev of events) {
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const agent = typeof p.agent === 'string' ? p.agent : '';

    if (ev.type === 'orchestrator_decision') {
      const kind = typeof p.decision_type === 'string' ? p.decision_type : '';
      if (kind === 'parallel_dispatch') {
        const agents = pickStringArray(p, ['dispatched_agents']);
        if (agents.length >= 2) {
          // Idempotency: if a window with the same member set is already
          // open, treat this as a re-emission and keep the existing
          // window so downstream skill_invokeds stay in one wave.
          const sameAsOpen =
            parallel !== null &&
            parallel.remaining.size === agents.length &&
            agents.every((a) => parallel!.remaining.has(a));
          if (!sameAsOpen) {
            pCounter++;
            parallel = {
              waveId: `wave-p-${pCounter}`,
              remaining: new Set(agents),
            };
          }
        }
      }
      continue;
    }

    if (!agent) continue;

    if (ev.type === 'skill_invoked') {
      const skill = typeof p.skill === 'string' ? p.skill : '';
      if (!skill) continue;

      // Idempotency: the same (agent, skill) pair already has an open
      // invocation → skip the duplicate event.
      const existing = openByAgent.get(agent);
      if (existing && existing.some((o) => o.skill === skill)) continue;

      let waveId: string;
      let isParallel: boolean;
      if (parallel && parallel.remaining.has(agent)) {
        waveId = parallel.waveId;
        isParallel = true;
      } else {
        sCounter++;
        waveId = `wave-s-${sCounter}`;
        isParallel = false;
      }

      const wave = ensureWave(waveId, isParallel);
      invocCounter++;
      const node: FlowNode = {
        id: `node-${agent}-${skill}-${invocCounter}`,
        agent,
        skill,
        state: 'running',
        waveId,
        startedAt: ev.timestamp,
      };
      wave.nodes.push(node);
      // Promote to parallel once the wave has two or more members.
      if (wave.nodes.length > 1) wave.parallel = true;

      const stack = openByAgent.get(agent) ?? [];
      stack.push({ node, skill });
      openByAgent.set(agent, stack);
      continue;
    }

    if (ev.type === 'skill_completed' || ev.type === 'skill_failed') {
      const skill = typeof p.skill === 'string' ? p.skill : '';
      const stack = openByAgent.get(agent);
      if (!stack || stack.length === 0) continue;
      // Match the most recent open invocation with the same skill name;
      // fall back to the most recent if no exact match (defensive).
      let idx = -1;
      for (let i = stack.length - 1; i >= 0; i--) {
        if (stack[i].skill === skill) { idx = i; break; }
      }
      if (idx === -1) idx = stack.length - 1;
      const open = stack[idx];
      if (ev.type === 'skill_completed') {
        open.node.state = 'completed';
        open.node.completedAt = ev.timestamp;
      } else {
        open.node.state = 'failed';
        open.node.completedAt = ev.timestamp;
      }
      stack.splice(idx, 1);
      if (stack.length === 0) openByAgent.delete(agent);
      continue;
    }

    if (ev.type === 'agent_failed') {
      const stack = openByAgent.get(agent);
      if (stack && stack.length > 0) {
        const open = stack[stack.length - 1];
        open.node.state = 'failed';
        open.node.completedAt = ev.timestamp;
        stack.pop();
      }
      if (parallel && parallel.remaining.has(agent)) {
        parallel.remaining.delete(agent);
        if (parallel.remaining.size === 0) parallel = null;
      }
      continue;
    }

    if (ev.type === 'agent_blocked') {
      const stack = openByAgent.get(agent);
      if (stack && stack.length > 0) {
        const open = stack[stack.length - 1];
        open.node.state = 'blocked';
        stack.pop();
      }
      if (parallel && parallel.remaining.has(agent)) {
        parallel.remaining.delete(agent);
        if (parallel.remaining.size === 0) parallel = null;
      }
      continue;
    }

    if (ev.type === 'agent_completed') {
      if (parallel && parallel.remaining.has(agent)) {
        parallel.remaining.delete(agent);
        if (parallel.remaining.size === 0) parallel = null;
      }
      continue;
    }
  }

  // Sort members of parallel waves deterministically for stable render.
  for (const wave of waves) {
    if (wave.parallel) {
      wave.nodes.sort((a, b) => a.agent.localeCompare(b.agent) || a.id.localeCompare(b.id));
    }
  }
  return waves;
}
