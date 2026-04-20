/**
 * copyText places `value` on the user's clipboard. Prefers the async
 * Clipboard API (secure contexts, modern browsers) and falls back to a
 * hidden-textarea + `document.execCommand('copy')` when the async API is
 * unavailable — unpoisoned non-secure contexts, older browsers, or iframes
 * that don't have the relevant permissions.
 *
 * Returns a Promise that resolves on success and rejects with a descriptive
 * Error on failure. Callers render the error message to the user; silent
 * failures are the worst possible UX for copy buttons.
 */
export async function copyText(value: string): Promise<void> {
  if (typeof value !== 'string') {
    throw new Error('copyText: value must be a string');
  }
  const nav = typeof navigator !== 'undefined' ? navigator : null;
  const clipboard = nav?.clipboard;
  if (clipboard && typeof clipboard.writeText === 'function') {
    try {
      await clipboard.writeText(value);
      return;
    } catch (err) {
      // Fall through to the legacy path on permission errors.
      // eslint-disable-next-line no-console
      console.warn('clipboard.writeText failed, trying fallback', err);
    }
  }
  return copyViaTextarea(value);
}

function copyViaTextarea(value: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (typeof document === 'undefined') {
      reject(new Error('copyText: no document available'));
      return;
    }
    const ta = document.createElement('textarea');
    ta.value = value;
    // Off-screen but focusable; display:none blocks selection.
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.top = '0';
    ta.style.left = '0';
    ta.style.width = '1px';
    ta.style.height = '1px';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    try {
      ta.focus();
      ta.select();
      const ok = document.execCommand('copy');
      if (!ok) {
        reject(new Error('copyText: execCommand returned false'));
        return;
      }
      resolve();
    } catch (err) {
      reject(err instanceof Error ? err : new Error(String(err)));
    } finally {
      document.body.removeChild(ta);
    }
  });
}
