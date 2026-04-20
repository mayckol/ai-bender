import { afterEach, beforeEach, describe, expect, test } from 'bun:test';

import { copyText } from '../src/client/lib/clipboard.ts';

describe('copyText', () => {
  const original = globalThis.navigator;
  let stubCalls: string[] = [];
  let stubResult: 'ok' | 'reject' = 'ok';

  beforeEach(() => {
    stubCalls = [];
    stubResult = 'ok';
    (globalThis as unknown as { navigator: { clipboard: { writeText: (v: string) => Promise<void> } } }).navigator = {
      clipboard: {
        async writeText(v: string) {
          stubCalls.push(v);
          if (stubResult === 'reject') throw new Error('denied');
        },
      },
    };
  });

  afterEach(() => {
    if (original) (globalThis as unknown as { navigator: typeof original }).navigator = original;
  });

  test('rejects non-string values', async () => {
    // @ts-expect-error — intentionally bad call
    await expect(copyText(42)).rejects.toThrow(/string/);
  });

  test('uses navigator.clipboard when available', async () => {
    await copyText('hello');
    expect(stubCalls).toEqual(['hello']);
  });

  test('falls back to execCommand when clipboard path rejects', async () => {
    stubResult = 'reject';
    // Fallback needs a DOM; jsdom isn't loaded here. Assert the fallback
    // surface is reached by expecting a deterministic rejection with the
    // fallback error (no document in bun test env).
    await expect(copyText('oops')).rejects.toThrow(/no document available|execCommand/);
  });
});
