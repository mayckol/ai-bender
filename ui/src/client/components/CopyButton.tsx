import { useState } from 'preact/hooks';

import { copyText } from '../lib/clipboard.ts';

interface Props {
  /**
   * The text to place on the clipboard. When a function, it's invoked at
   * click time so callers can defer serialisation of large artifacts.
   */
  value: string | (() => string | Promise<string>);
  /** Short button label. Defaults to "copy". */
  label?: string;
  /** Optional CSS class so callers can tune size/placement. */
  class?: string;
  /** Optional aria-label override for screen readers. */
  title?: string;
}

type Phase = 'idle' | 'ok' | 'err';

/**
 * CopyButton is a reusable artifact-copy control (feature 007 US4). It shows
 * a short success/failure state for ~1.5s after the click and always keeps
 * keyboard focus on the button so sighted + screen-reader users both see the
 * change. The button never throws — failures flip into the `err` phase with
 * an aria-live announcement instead of a silent no-op.
 */
export function CopyButton({ value, label = 'copy', class: cls, title }: Props) {
  const [phase, setPhase] = useState<Phase>('idle');

  const onClick = async (ev: MouseEvent) => {
    ev.stopPropagation();
    try {
      const resolved = typeof value === 'function' ? await value() : value;
      await copyText(resolved);
      setPhase('ok');
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('CopyButton: copy failed', err);
      setPhase('err');
    }
    window.setTimeout(() => setPhase('idle'), 1500);
  };

  const displayLabel = phase === 'ok' ? 'copied' : phase === 'err' ? 'failed' : label;
  const finalClass = ['copy-btn', `copy-${phase}`, cls].filter(Boolean).join(' ');
  return (
    <button
      type="button"
      class={finalClass}
      aria-label={title ?? `Copy ${label}`}
      aria-live="polite"
      onClick={onClick}
    >
      <span class="copy-btn-icon" aria-hidden="true">⧉</span>
      <span class="copy-btn-label">{displayLabel}</span>
    </button>
  );
}
