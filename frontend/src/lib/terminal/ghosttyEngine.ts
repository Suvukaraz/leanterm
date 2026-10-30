// ghostty-web implementation of the terminal handle: Ghostty's VT parser (WASM)
// with a canvas renderer and an xterm.js-like API.
//
// Notes on its API:
// - attachCustomKeyEventHandler: returning true means "consumed" (inverse of xterm.js).
// - Wheel events are swallowed (stopPropagation), so zoom uses the wheel hook.
// - No search addon: the find bar uses textSearch.ts and sets the selection directly
//   (Terminal.select() maps rows to the wrong place while scrolled back).
// - Built-in link providers call window.open(); they are rewired to hooks.openLink.
// - Theme colours are baked into the WASM terminal at creation (no live update) and
//   #rrggbbaa is not understood, so the background is always opaque.
// - Every terminal gets its own WASM instance. With one shared instance, memory freed
//   by a closed terminal is handed to another one without being zeroed, and a resize
//   (new columns/rows) then shows old text from that closed terminal.
import { Ghostty, FitAddon, Terminal, type ILink, type ILinkProvider } from 'ghostty-web';

import type { SearchOptions, TermHandle, TerminalHooks } from './engine';
import { buildMatcher, findMatch, type SearchMatch, type SearchRow } from './textSearch';
import { buildTerminalOptions, buildTheme, cursorStyle, fontFamilyCss, type TerminalLook } from './look';

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
    boundContextMenuHandler: ((e: MouseEvent) => void) | null;
  };
  renderer?: { getCanvas(): HTMLCanvasElement };
  linkDetector?: {
    providers: ILinkProvider[];
    invalidateCache(): void;
  };
}

export async function createGhosttyTerminal(
  container: HTMLElement,
  look: TerminalLook,
  hooks: TerminalHooks
): Promise<TermHandle> {
  const ghostty = await Ghostty.load();

  const base = buildTerminalOptions(look);
  const t = new Terminal({
    ghostty,
    fontFamily: base.fontFamily,
    fontSize: base.fontSize,
    cursorBlink: true,
    cursorStyle: base.cursorStyle,
    scrollback: base.scrollback,
    // ghostty-web blends a translucent selection itself and keeps the text colours.
    theme: { ...buildTheme(look), selectionBackground: 'rgba(120, 160, 220, 0.45)', selectionForeground: undefined },
    smoothScrollDuration: 0,
  });
  const fitAddon = new FitAddon();
  t.loadAddon(fitAddon);
  t.attachCustomKeyEventHandler((e) => hooks.onKey(e));
  // Accumulates fractional wheel deltas (touchpads) so slow scrolling still moves.
  let wheelRemainder = 0;
  t.attachCustomWheelEventHandler((e) => {
    if (hooks.onWheel(e)) return true;
    // ghostty-web turns the wheel into arrow keys on the alternate screen and never
    // reports it to applications. Like other terminals, send wheel reports when the
    // application enabled mouse tracking (mc, htop, tmux, vim mouse=a, ...).
    if (!t.hasMouseTracking()) return false;

    const rect = t.element?.getBoundingClientRect();
    const lineHeight = rect && t.rows ? rect.height / t.rows : 20;
    const lines =
      e.deltaMode === WheelEvent.DOM_DELTA_LINE
        ? e.deltaY
        : e.deltaMode === WheelEvent.DOM_DELTA_PAGE
          ? e.deltaY * t.rows
          : e.deltaY / lineHeight;
    wheelRemainder += lines;
    const steps = Math.trunc(wheelRemainder);
    wheelRemainder -= steps;
    if (steps === 0) return true;

    let col = 0;
    let row = 0;
    if (rect && rect.width > 0 && rect.height > 0) {
      col = Math.floor(((e.clientX - rect.left) / rect.width) * t.cols);
      row = Math.floor(((e.clientY - rect.top) / rect.height) * t.rows);
    }
    col = Math.max(0, Math.min(t.cols - 1, col)) + 1;
    row = Math.max(0, Math.min(t.rows - 1, row)) + 1;

    const button = (steps < 0 ? 64 : 65) + (e.shiftKey ? 4 : 0) + (e.altKey ? 8 : 0);
    const sgr = t.getMode(1006);
    const report = sgr
      ? `\x1b[<${button};${col};${row}M`
      : `\x1b[M${String.fromCharCode(32 + button, 32 + Math.min(col, 223), 32 + Math.min(row, 223))}`;
    hooks.onData(report.repeat(Math.min(Math.abs(steps), 10)));
    return true;
  });
  t.open(container);

  const internals = t as unknown as GhosttyInternals;

  // ghostty-web moves a hidden textarea under the pointer and focuses it on right
  // click (meant for the browser's native menu). That breaks LeanTerm's own
  // context menu (Paste did nothing), so drop the handler.
  const sel = internals.selectionManager;
  if (sel?.boundContextMenuHandler) {
    internals.renderer?.getCanvas().removeEventListener('contextmenu', sel.boundContextMenuHandler);
    sel.boundContextMenuHandler = null;
  }

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
