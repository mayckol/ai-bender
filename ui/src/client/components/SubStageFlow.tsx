import { agentColor } from '../lib/agents.ts';
import { formatDuration, type SkillStep, type StageDef } from '../lib/pipeline.ts';

interface Props {
  stage: StageDef | null;
  steps: SkillStep[];
  sessionStatus: string | undefined;
}

export function SubStageFlow({ stage, steps, sessionStatus }: Props) {
  if (!stage) {
    return (
      <div class="substage-empty">
        <div class="substage-empty-rule" aria-hidden="true" />
        <p>This command is outside the core pipeline. The flow view tracks the four canonical stages.</p>
      </div>
    );
  }

  if (steps.length === 0) {
    return (
      <div class="substage-empty">
        <div class="substage-empty-rule" aria-hidden="true" />
        <p>No sub-steps emitted yet. Waiting for the first <code>skill_invoked</code> event.</p>
      </div>
    );
  }

  const doneCount = steps.filter((s) => s.status === 'completed').length;
  const total = steps.length;

  return (
    <div class="substage">
      <header class="substage-head">
        <div class="substage-title">
          <span class="substage-kicker">/{stage.id}</span>
          <span class="substage-heading">{stage.label} flow</span>
        </div>
        <div class="substage-counter">
          <span class="counter-done">{String(doneCount).padStart(2, '0')}</span>
          <span class="counter-sep">/</span>
          <span class="counter-total">{String(total).padStart(2, '0')}</span>
          <span class="counter-label">steps</span>
        </div>
      </header>
      <ol class="substage-list">
        {steps.map((step, idx) => (
          <SubStageNode
            key={step.name}
            step={step}
            index={idx}
            total={steps.length}
            sessionStatus={sessionStatus}
          />
        ))}
      </ol>
    </div>
  );
}

function SubStageNode({
  step,
  index,
  total,
  sessionStatus,
}: {
  step: SkillStep;
  index: number;
  total: number;
  sessionStatus: string | undefined;
}) {
  const isLast = index === total - 1;
  const status = resolveStatus(step, sessionStatus);
  const color = step.agent ? agentColor(step.agent) : undefined;
  return (
    <li class={`substage-node substage-${status}`}>
      <div class="substage-lane" aria-hidden="true">
        <div class="substage-bullet">
          <span class="substage-bullet-core" />
          {status === 'running' && <span class="substage-bullet-pulse" />}
        </div>
        {!isLast && <div class="substage-stem" />}
      </div>
      <div class="substage-card">
        <div class="substage-card-head">
          <span class="substage-idx">{String(index + 1).padStart(2, '0')}</span>
          <span class="substage-name">{step.name}</span>
          {step.agent && (
            <span
              class="substage-agent"
              style={color ? { color, borderColor: `${color}55`, background: `${color}1a` } : undefined}
            >
              {step.agent}
            </span>
          )}
          <span class={`substage-chip chip-${status}`}>{chipText(status)}</span>
        </div>
        <div class="substage-card-foot">
          <span class="substage-meta">
            <span class="substage-meta-key">duration</span>
            <span class="substage-meta-val">{formatDuration(step.durationMs)}</span>
          </span>
          {step.startedAt && (
            <span class="substage-meta">
              <span class="substage-meta-key">started</span>
              <span class="substage-meta-val">{step.startedAt.slice(11, 19)}</span>
            </span>
          )}
          {step.completedAt && (
            <span class="substage-meta">
              <span class="substage-meta-key">completed</span>
              <span class="substage-meta-val">{step.completedAt.slice(11, 19)}</span>
            </span>
          )}
        </div>
      </div>
    </li>
  );
}

function resolveStatus(step: SkillStep, sessionStatus: string | undefined): SkillStep['status'] {
  if (step.status === 'running' && (sessionStatus === 'completed' || sessionStatus === 'awaiting_confirm' || sessionStatus === 'failed')) {
    return 'completed';
  }
  return step.status;
}

function chipText(status: SkillStep['status']): string {
  switch (status) {
    case 'pending': return 'queued';
    case 'running': return 'running';
    case 'completed': return 'done';
    case 'failed': return 'failed';
  }
}
