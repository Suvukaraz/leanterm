import { Clipboard } from '@wailsio/runtime';

/**
 * Reads the system clipboard through the Go backend. navigator.clipboard.readText()
 * depends on WebView focus and permission state and can fail silently, so it is
 * only used as a fallback.
 */
export async function readClipboardText(): Promise<string> {
  try {
    const text = await Clipboard.Text();
    if (text) return text;
  } catch {
    // fall through to the browser API
  }
  return navigator.clipboard.readText().catch(() => '');
}
