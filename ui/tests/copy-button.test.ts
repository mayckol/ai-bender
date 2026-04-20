import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { render } from 'preact-render-to-string';
import { h } from 'preact';

import { CopyButton } from '../src/client/components/CopyButton.tsx';

describe('CopyButton', () => {
  const original = globalThis.navigator;
  let writes: string[] = [];

  beforeEach(() => {
    writes = [];
    (globalThis as unknown as { navigator: { clipboard: { writeText: (v: string) => Promise<void> } } }).navigator = {
      clipboard: {
        async writeText(v: string) { writes.push(v); },
      },
    };
  });

  afterEach(() => {
    if (original) (globalThis as unknown as { navigator: typeof original }).navigator = original;
  });

  test('renders with default label', () => {
    const html = render(h(CopyButton, { value: 'hello' }));
    expect(html).toContain('copy');
    expect(html).toContain('aria-label="Copy copy"');
  });

  test('renders with custom label', () => {
    const html = render(h(CopyButton, { value: 'hello', label: 'copy spec', title: 'Copy the spec file' }));
    expect(html).toContain('copy spec');
    expect(html).toContain('Copy the spec file');
  });

  test('accepts a function value (lazy serialisation)', () => {
    const html = render(h(CopyButton, { value: () => 'expensive' }));
    // Rendered output does not reveal the value itself — just assert render
    // does not throw and emits a button.
    expect(html).toContain('<button');
  });
});
