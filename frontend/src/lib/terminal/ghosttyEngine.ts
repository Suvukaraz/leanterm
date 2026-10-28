// ghostty-web implementation of the terminal handle: Ghostty's VT parser (WASM)
// with a canvas renderer and an xterm.js-like API.
//
// Differences to xterm.js handled here:
// - init() must load the WASM once before the first Terminal is created.
// - attachCustomKeyEventHandler: returning true means "consumed" (inverse of xterm).
// - Wheel events are swallowed (stopPropagation), so zoom uses the wheel hook.
// - No search addon: the find bar uses textSearch.ts and sets the selection directly
//   (Terminal.select() maps rows to the wrong place while scrolled back).
// - Built-in link providers call window.open(); they are rewired to hooks.openLink.
// - Theme colours are baked into the WASM terminal at creation (no live update) and
//   #rrggbbaa is not understood, so the background is always opaque.
import { init, FitAddon, Terminal, type ILink, type ILinkProvider } from 'ghostty-web';

import type { SearchOptions, TermHandle, TerminalHooks } from './engine';
import { buildMatcher, findMatch, type SearchMatch, type SearchRow } from './textSearch';
import { buildTerminalOptions, buildTheme, fontFamilyCss, type TerminalLook } from './xtermOptions';

interface SelectionPoint {
  col: number;
  absoluteRow: number;
}

/** Private ghostty-web members used here (pinned version, see package.json). */
interface GhosttyInternals {
  selectionManager?: {
    selectionStart: SelectionPoint | null;
    selectionEnd: SelectionPoint | null;
    requestRender(): void;
    selectionChangedEmitter: { fire(): void };
  };
  linkDetector?: {
    providers: ILinkProvider[];
    invalidateCache(): void;
  };
}

let wasmReady: Promise<void> | undefined;

function ensureWasm(): Promise<void> {
  wasmReady ??= init().catch((err) => {
    wasmReady = undefined;
    throw err;
  });
  return wasmReady;
}

function cursorStyle(look: TerminalLook): 'block' | 'underline' | 'bar' {
  return look.CursorShape === 'ibeam' ? 'bar' : look.CursorShape === 'underline' ? 'underline' : 'block';
}

export async function createGhosttyTerminal(
  container: HTMLElement,
  look: TerminalLook,
  hooks: TerminalHooks
): Promise<TermHandle> {
  await ensureWasm();

  const base = buildTerminalOptions(look);
  const t = new Terminal({
    fontFamily: base.fontFamily,
    fontSize: base.fontSize,
    cursorBlink: true,
    cursorStyle: cursorStyle(look),
    scrollback: base.scrollback,
    theme: buildTheme({ ...look, Transparency: 0 }),
    smoothScrollDuration: 0,
  });
  const fitAddon = new FitAddon();
  t.loadAddon(fitAddon);
  t.attachCustomKeyEventHandler((e) => hooks.onKey(e));
  t.attachCustomWheelEventHandler((e) => hooks.onWheel(e));
  t.open(container);

  const internals = t as unknown as GhosttyInternals;

  // Open links through the backend (xdg-open) instead of window.open().
  for (const provider of internals.linkDetector?.providers ?? []) {
    const provide = provider.provideLinks.bind(provider);
    provider.provideLinks = (y, callback) =>
      provide(y, (links) =>
        callback(
          links?.map(
            (link): ILink => ({
              ...link,
              activate: (e) => {
                if (e.ctrlKey || e.metaKey) hooks.openLink(link.text);
              },
            })
          )
        )
      );
  }
  internals.linkDetector?.invalidateCache();

  t.onData((data) => hooks.onData(data));
  t.onSelectionChange(() => hooks.onSelectionChange());

  // ---- Find bar ----------------------------------------------------------
  let lastMatch: SearchMatch | undefined;
  let lastQuery = '';

  function searchRow(r: number): SearchRow | undefined {
    const line = t.buffer.active.getLine(r);
    if (!line) return undefined;
    let text = '';
    const start: number[] = [];
    const end: number[] = [];
    for (let x = 0; x < line.length; x++) {
      const cell = line.getCell(x);
      if (!cell) continue;
      const width = cell.getWidth();
      if (width === 0) continue; // trailing half of a wide character
      const chars = cell.getChars() || ' ';
      for (let i = 0; i < chars.length; i++) {
        start.push(x);
        end.push(x + Math.max(1, width) - 1);
      }
      text += chars;
    }
    return { text, start, end };
  }

  function find(text: string, opts: SearchOptions, backward: boolean): boolean {
    const re = buildMatcher(text, opts);
    const query = `${text}\u0000${opts.caseSensitive}${opts.wholeWord}${opts.regex}`;
    const changed = query !== lastQuery;
    lastQuery = query;
    if (!re) return false;

    const buffer = t.buffer.active;
    const alternate = buffer.type === 'alternate';
    const scrollback = alternate ? 0 : t.getScrollbackLength();
    const viewportTop = Math.max(0, scrollback - Math.floor(t.getViewportY()));
    const rowCount = buffer.length;
    if (lastMatch && lastMatch.row >= rowCount) lastMatch = undefined;

    let from;
    if (lastMatch) {
      // A changed query re-checks the current match position (incremental typing).
      from = { row: lastMatch.row, col: changed ? lastMatch.col - (backward ? -1 : 1) : lastMatch.col };
    } else {
      from = backward ? { row: viewportTop + t.rows - 1, col: Number.MAX_SAFE_INTEGER } : { row: viewportTop, col: -1 };
    }

    const match = findMatch(searchRow, rowCount, re, from, backward);
    lastMatch = match;
    if (!match) return false;

    if (!alternate) {
      const visible = match.row >= viewportTop && match.row < viewportTop + t.rows;
      if (!visible) t.scrollToLine(Math.max(0, Math.min(scrollback, scrollback - match.row + Math.floor(t.rows / 2))));
    }

    const sel = internals.selectionManager;
    if (sel) {
      // Selection rows are absolute in scrollback + screen, also for the alternate screen.
      const absRow = alternate ? t.getScrollbackLength() + match.row : match.row;
      sel.selectionStart = { col: match.col, absoluteRow: absRow };
      sel.selectionEnd = { col: match.endCol, absoluteRow: absRow };
      sel.requestRender();
      sel.selectionChangedEmitter.fire();
    }
    return true;
  }

  return {
    engine: 'ghostty',
    get cols() {
      return t.cols;
    },
    get rows() {
      return t.rows;
    },
    get element() {
      return t.element;
    },
    search: {
      findNext: (text, opts) => find(text, opts, false),
      findPrevious: (text, opts) => find(text, opts, true),
    },
    write: (data, callback) => t.write(data, callback),
    paste: (text) => t.paste(text),
    getSelection: () => t.getSelection(),
    focus: () => t.focus(),
    reset: () => t.reset(),
    clear: () => t.clear(),
    fit() {
      const dims = fitAddon.proposeDimensions();
      if (!dims || !dims.cols || !dims.rows) return undefined;
      if (dims.cols !== t.cols || dims.rows !== t.rows) t.resize(dims.cols, dims.rows);
      return { cols: t.cols, rows: t.rows };
    },
    setFontSize(px) {
      t.options.fontSize = px;
    },
    setLook(l, fontPx) {
      // Colours only apply to newly opened terminals (see header).
      t.options.fontFamily = fontFamilyCss(l.Font?.Family);
      t.options.fontSize = fontPx;
      t.options.cursorStyle = cursorStyle(l);
    },
    lines(max) {
      const buffer = t.buffer.active;
      const out: string[] = [];
      for (let i = 0; i < Math.min(buffer.length, max); i++) {
        out.push(searchRow(i)?.text.trimEnd() ?? '');
      }
      return out;
    },
    dispose: () => t.dispose(),
  };
}
