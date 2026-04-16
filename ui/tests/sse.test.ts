import { describe, expect, test } from 'bun:test';

import { sseEncode, sseStream } from '../src/server/sse.ts';

describe('sse helpers', () => {
  test('sseEncode formats event + data per the protocol', () => {
    const bytes = sseEncode('event', { a: 1 });
    const text = new TextDecoder().decode(bytes);
    expect(text).toBe('event: event\ndata: {"a":1}\n\n');
  });

  test('sseEncode accepts raw strings as data payload', () => {
    const text = new TextDecoder().decode(sseEncode('ping', 'hello'));
    expect(text).toBe('event: ping\ndata: hello\n\n');
  });

  test('sseStream streams written chunks to a reader in order', async () => {
    const s = sseStream();
    s.write('a', { n: 1 });
    s.write('b', { n: 2 });
    s.close();

    const reader = s.readable.getReader();
    let received = '';
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      received += new TextDecoder().decode(value);
    }
    expect(received).toBe(
      'event: a\ndata: {"n":1}\n\n' +
      'event: b\ndata: {"n":2}\n\n',
    );
    expect(s.closed).toBe(true);
  });

  test('sseStream becomes a no-op after close', () => {
    const s = sseStream();
    s.close();
    // Should not throw when writing after close.
    s.write('late', { ok: true });
    expect(s.closed).toBe(true);
  });
});
