interface Option<T extends string> {
  value: T;
  label: string;
  glyph?: string;
}

interface Props<T extends string> {
  options: Option<T>[];
  value: T;
  onChange: (value: T) => void;
  ariaLabel?: string;
}

export function SegmentedToggle<T extends string>({ options, value, onChange, ariaLabel }: Props<T>) {
  const activeIdx = Math.max(0, options.findIndex((o) => o.value === value));
  const pct = 100 / options.length;
  return (
    <div class="seg" role="tablist" aria-label={ariaLabel}>
      <div
        class="seg-thumb"
        style={{ width: `${pct}%`, transform: `translateX(${activeIdx * 100}%)` }}
        aria-hidden="true"
      />
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="tab"
          aria-selected={o.value === value}
          class={`seg-btn${o.value === value ? ' active' : ''}`}
          onClick={() => onChange(o.value)}
        >
          {o.glyph && <span class="seg-glyph">{o.glyph}</span>}
          <span class="seg-label">{o.label}</span>
        </button>
      ))}
    </div>
  );
}
