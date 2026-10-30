// Engine-neutral terminal handle. TerminalView, TerminalStream, the find bar and
// the context menu only talk to this interface, so rioterm and ghostty-web can be
// swapped per terminal ([terminal] engine).
import type { TerminalLook } from './look';

export type TerminalEngine = 'rio' | 'ghostty';

export interface SearchOptions {
  caseSensitive: boolean;
  wholeWord: boolean;
  regex: boolean;
}

export interface TerminalSearch {
  /** Selects and reveals the next match; false when there is none. */
  findNext(text: string, opts: SearchOptions): boolean;
  findPrevious(text: string, opts: SearchOptions): boolean;
}

export interface TerminalHooks {
  /** Keydown before the terminal handles it; return true when LeanTerm consumed it. */
  onKey(e: KeyboardEvent): boolean;
  /** Wheel events the engine swallows before they bubble (Ctrl+wheel zoom); true = consumed. */
  onWheel(e: WheelEvent): boolean;
  /** Input for the backend (keystrokes, pastes, terminal responses). */
  onData(data: string | Uint8Array): void;
  onSelectionChange(): void;
  openLink(uri: string): void;
}

export interface TermHandle {
  readonly engine: TerminalEngine;
  readonly cols: number;
  readonly rows: number;
  /** Element the terminal was opened in (contains its canvas). */
  readonly element: HTMLElement | undefined;
  readonly search: TerminalSearch;
  write(data: Uint8Array, callback?: () => void): void;
  /** Sends text as a paste (bracketed if the remote side asked for it). */
  paste(text: string): void;
  getSelection(): string;
  focus(): void;
  reset(): void;
  clear(): void;
  /** Resizes the grid to the container; returns the resulting size. */
  fit(): { cols: number; rows: number } | undefined;
  setFontSize(px: number): void;
  /** Applies a changed look (Preferences) to the running terminal. */
  setLook(look: TerminalLook, fontPx: number): void;
  /** Text of the first `max` buffer lines (screenshot fallback). */
  lines(max: number): string[];
  dispose(): void;
}

/** Rio is the default; unknown values (and the former "xterm") fall back to it. */
export function normalizeEngine(value: string | undefined | null): TerminalEngine {
  return value === 'ghostty' ? 'ghostty' : 'rio';
}

/** Creates a terminal of the given engine inside `container`. */
export async function createTerminal(
  engine: TerminalEngine,
  container: HTMLElement,
  look: TerminalLook,
  hooks: TerminalHooks
): Promise<TermHandle> {
  // Loaded on demand: each engine brings its own WASM core.
  if (engine === 'ghostty') {
    const { createGhosttyTerminal } = await import('./ghosttyEngine');
    return createGhosttyTerminal(container, look, hooks);
  }
  const { createRioTerminal } = await import('./rioEngine');
  return createRioTerminal(container, look, hooks);
}
