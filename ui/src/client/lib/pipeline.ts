import type { BenderEvent } from './api.ts';

export type StageId = 'cry' | 'plan' | 'tdd' | 'ghu';

export interface StageDef {
  id: StageId;
  label: string;
  subtitle: string;
  glyph: string;
  commands: string[];
  skills: string[];
  confirmable: boolean;
}

export const PIPELINE: StageDef[] = [
  {
    id: 'cry',
    label: 'Capture',
    subtitle: 'intent',
    glyph: '◇',
    commands: ['/cry'],
    skills: ['classification', 'predecessor_lookup', 'drafting'],
    confirmable: true,
  },
  {
    id: 'plan',
    label: 'Plan',
    subtitle: 'design',
    glyph: '◈',
    commands: ['/plan'],
    skills: ['spec_draft', 'data_model', 'api_contract', 'risk_assessment', 'tasks_decompose'],
    confirmable: true,
  },
  {
    id: 'tdd',
    label: 'Scaffold',
    subtitle: 'tests',
    glyph: '◉',
    commands: ['/tdd'],
    skills: ['scaffold_enumerate'],
    confirmable: true,
  },
  {
    id: 'ghu',
    label: 'Execute',
    subtitle: 'code + review',
    glyph: '◆',
    commands: ['/ghu'],
    skills: [],
    confirmable: false,
  },
];

export function stageForCommand(command: string | undefined | null): StageDef | null {
  if (!command) return null;
  const head = command.trim().split(/\s+/)[0];
  return PIPELINE.find((s) => s.commands.includes(head)) ?? null;
}

export function isConfirmRun(command: string | undefined | null): boolean {
  if (!command) return false;
  return /\bconfirm\b/.test(command);
}

export interface SkillStep {
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  agent?: string;
  durationMs?: number;
  startedAt?: string;
  completedAt?: string;
}

export function extractSkillSteps(events: BenderEvent[], stage: StageDef | null): SkillStep[] {
  const invoked = new Map<string, SkillStep>();
  const order: string[] = [];

  for (const ev of events) {
    const p = (ev.payload ?? {}) as Record<string, unknown>;
    const skill = typeof p.skill === 'string' ? p.skill : '';
    if (!skill) continue;

    if (ev.type === 'skill_invoked') {
      if (!invoked.has(skill)) {
        order.push(skill);
        invoked.set(skill, {
          name: skill,
          status: 'running',
          agent: typeof p.agent === 'string' ? p.agent : undefined,
          startedAt: ev.timestamp,
        });
      }
    } else if (ev.type === 'skill_completed') {
      const existing = invoked.get(skill);
      if (!existing) {
        order.push(skill);
        invoked.set(skill, {
          name: skill,
          status: 'completed',
          agent: typeof p.agent === 'string' ? p.agent : undefined,
          durationMs: typeof p.duration_ms === 'number' ? p.duration_ms : undefined,
          completedAt: ev.timestamp,
        });
      } else {
        existing.status = 'completed';
        existing.durationMs = typeof p.duration_ms === 'number' ? p.duration_ms : existing.durationMs;
        existing.completedAt = ev.timestamp;
      }
    } else if (ev.type === 'skill_failed') {
      const existing = invoked.get(skill);
      if (!existing) {
        order.push(skill);
        invoked.set(skill, { name: skill, status: 'failed', agent: typeof p.agent === 'string' ? p.agent : undefined });
      } else {
        existing.status = 'failed';
      }
    }
  }

  const fromEvents = order.map((k) => invoked.get(k)!);

  if (stage && stage.skills.length > 0) {
    const tmpl = stage.skills.map<SkillStep>((name) => ({ name, status: 'pending' }));
    for (const actual of fromEvents) {
      const slot = tmpl.find((t) => t.name === actual.name);
      if (slot) Object.assign(slot, actual);
      else tmpl.push(actual);
    }
    return tmpl;
  }
  return fromEvents;
}

export type PipelineStageStatus = 'completed' | 'awaiting_confirm' | 'running' | 'upcoming' | 'skipped';

export function stageStatusFor(opts: {
  stage: StageDef;
  currentStage: StageDef | null;
  sessionStatus: string | undefined;
  isConfirm: boolean;
}): PipelineStageStatus {
  const { stage, currentStage, sessionStatus, isConfirm } = opts;
  if (!currentStage) return 'upcoming';

  const order = PIPELINE.map((s) => s.id);
  const stageIdx = order.indexOf(stage.id);
  const currentIdx = order.indexOf(currentStage.id);

  if (stageIdx < currentIdx) return 'completed';
  if (stageIdx > currentIdx) return 'upcoming';

  if (sessionStatus === 'completed') return 'completed';
  if (sessionStatus === 'failed') return 'upcoming';
  if (sessionStatus === 'awaiting_confirm') return 'awaiting_confirm';
  if (isConfirm) return 'completed';
  return 'running';
}

export function formatDuration(ms: number | undefined): string {
  if (!ms || ms <= 0) return '—';
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = Math.floor(s - m * 60);
  return `${m}m${rem}s`;
}
