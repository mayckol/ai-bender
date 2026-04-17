import {
  PIPELINE,
  type PipelineStageStatus,
  stageStatusFor,
  type StageDef,
} from '../lib/pipeline.ts';

interface Props {
  currentStage: StageDef | null;
  sessionStatus: string | undefined;
  isConfirm: boolean;
}

export function PipelineFlow({ currentStage, sessionStatus, isConfirm }: Props) {
  return (
    <div class="pipeline">
      <div class="pipeline-rail" aria-hidden="true" />
      <ol class="pipeline-nodes">
        {PIPELINE.map((stage, idx) => {
          const status = stageStatusFor({ stage, currentStage, sessionStatus, isConfirm });
          const isCurrent = currentStage?.id === stage.id;
          return (
            <li key={stage.id} class={`pnode pnode-${status}${isCurrent ? ' is-current' : ''}`}>
              <div class="pnode-index">
                <span class="pnode-index-num">{String(idx + 1).padStart(2, '0')}</span>
              </div>
              <div class="pnode-frame">
                <span class="pnode-tick pnode-tick-tl" aria-hidden="true" />
                <span class="pnode-tick pnode-tick-tr" aria-hidden="true" />
                <span class="pnode-tick pnode-tick-bl" aria-hidden="true" />
                <span class="pnode-tick pnode-tick-br" aria-hidden="true" />
                <div class="pnode-glyph" aria-hidden="true">
                  <span class="pnode-glyph-char">{stage.glyph}</span>
                  {isCurrent && <span class="pnode-glyph-halo" />}
                </div>
                <div class="pnode-meta">
                  <div class="pnode-label">{stage.label}</div>
                  <div class="pnode-subtitle">/{stage.id} · {stage.subtitle}</div>
                </div>
                <StatusSignal status={status} isConfirmRun={isCurrent && isConfirm} />
              </div>
              {idx < PIPELINE.length - 1 && <Connector status={status} />}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function StatusSignal({ status, isConfirmRun }: { status: PipelineStageStatus; isConfirmRun: boolean }) {
  const text = labelFor(status);
  return (
    <div class={`pnode-signal pnode-signal-${status}`}>
      <span class="pnode-signal-dot" aria-hidden="true" />
      <span class="pnode-signal-text">
        {text}
        {isConfirmRun && <span class="pnode-signal-conf"> · confirm</span>}
      </span>
    </div>
  );
}

function Connector({ status }: { status: PipelineStageStatus }) {
  const cls = status === 'completed' || status === 'awaiting_confirm' ? 'done' : status === 'running' ? 'active' : 'idle';
  return (
    <div class={`pnode-connector connector-${cls}`} aria-hidden="true">
      <span class="connector-beam" />
      <span class="connector-arrow">→</span>
    </div>
  );
}

function labelFor(s: PipelineStageStatus): string {
  switch (s) {
    case 'completed': return 'done';
    case 'awaiting_confirm': return 'awaits confirm';
    case 'running': return 'in flight';
    case 'upcoming': return 'upcoming';
    case 'skipped': return 'skipped';
  }
}
