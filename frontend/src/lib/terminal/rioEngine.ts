// rioterm implementation of the terminal handle (the default engine): Rio's VT
// core (librio, WASM) with its canvas renderer.
//
// rioterm's open() wires input in ways that do not fit LeanTerm (copy on every
// selection, confirm() before links, its own resize observer), so the Terminal
// and CanvasRenderer are put together here instead:
// - Keys go through a hidden textarea: hooks.onKey first, then Rio's key encoder;
//   AltGr, dead keys and IME text arrive as input/composition events.
// - The renderer takes font and colours at creation, so a changed look or font
//   size replaces it (the grid and scrollback stay in the Terminal).
// - Clicks and drags are reported to applications with mouse tracking (see
//   mouseReports.ts); the wheel is reported by Rio itself.
// - The canvas is opaque, so the background ignores the transparency setting.
import {
  CanvasRenderer,
  MOD_ALT,
  MOD_CTRL,
  MOD_SHIFT,
  Terminal,
  handleKeyboardEvent,
  initWasm,
  type SearchMatch,
} from 'rioterm';

import type { SearchOptions, TermHandle, TerminalHooks } from './engine';
import { buildTerminalOptions, buildTheme, fontFamilyCss, type TerminalLook } from './look';
import { MouseModes, encodeMouseReport, type MouseReport } from './mouseReports';

// Rio's default (1.3) leaves big gaps; xterm.js-like spacing that still fits descenders.
const LINE_HEIGHT = 1.2;
const MAX_SEARCH_MATCHES = 100000;

// Rio's regex syntax (Rust regex): escape only the classic metacharacters, it
// rejects escapes of other punctuation.
const RUST_META = /[\\.+*?()|[\]{}^$]/g;
const WORD_CHAR = /\w/;

/**
 * Turns the find bar text into a Rio search pattern. Rio uses smart case when no
 * flag is given, so case sensitivity is always explicit. Word boundaries are only
 * added next to word characters (a search for "-v" still finds " -v").
 */
export function rioSearchPattern(text: string, opts: SearchOptions): string | undefined {
  if (!text) return undefined;
  let pattern = opts.regex ? `(?:${text})` : text.replace(RUST_META, '\\$&');
  if (opts.wholeWord) {
    const boundary = '(?-u:\\b)';
    const start = opts.regex || WORD_CHAR.test(text[0]);
    const end = opts.regex || WORD_CHAR.test(text[text.length - 1]);
    pattern = `${start ? boundary : ''}${pattern}${end ? boundary : ''}`;
  }
  return `${opts.caseSensitive ? '(?-i)' : '(?i)'}${pattern}`;
}

const before = (a: { line: number; col: number }, b: { line: number; col: number }) =>
  a.line < b.line || (a.line === b.line && a.col < b.col);

/** Picks the match after (before) `from`, wrapping around; `inclusive` also accepts one at `from`. */
export function nextMatch(
  matches: SearchMatch[],
  from: { line: number; col: number },
  backward: boolean,
  inclusive: boolean
): SearchMatch | undefined {
  if (matches.length === 0) return undefined;
  const at = (m: SearchMatch) => ({ line: m.startLine, col: m.startCol });
  const same = (m: SearchMatch) => m.startLine === from.line && m.startCol === from.col;
  if (backward) {
    for (let i = matches.length - 1; i >= 0; i--) {
      if (before(at(matches[i]), from) || (inclusive && same(matches[i]))) return matches[i];
    }
    return matches[matches.length - 1];
  }
  for (const m of matches) {
    if (before(from, at(m)) || (inclusive && same(m))) return m;
  }
  return matches[0];
}

async function fontsReady(fontFamily: string, px: number): Promise<void> {
  // The renderer measures the cell size once; wait until the font can be measured.
  try {
    await document.fonts?.load(`${px}px ${fontFamily}`);
  } catch {
    // Unknown fonts fall back to the next family in the list.
  }
}

export async function createRioTerminal(container: HTMLElement, look: TerminalLook, hooks: TerminalHooks): Promise<TermHandle> {
  const base = buildTerminalOptions(look);
  await Promise.all([initWasm(), fontsReady(base.fontFamily, base.fontSize)]);

  const t = new Terminal({ scrollback: base.scrollback });
  const mouseModes = new MouseModes();

  const root = document.createElement('div');
  root.className = 'rio-terminal';
  Object.assign(root.style, { position: 'relative', width: '100%', height: '100%', overflow: 'hidden' });

  const textarea = document.createElement('textarea');
  textarea.className = 'rio-terminal-input';
  textarea.setAttribute('autocorrect', 'off');
  textarea.setAttribute('autocapitalize', 'off');
  textarea.setAttribute('spellcheck', 'false');
  textarea.setAttribute('aria-label', 'Terminal input');
  Object.assign(textarea.style, {
    position: 'absolute',
    left: '-9999px',
    top: '0',
    width: '1px',
    height: '1px',
    opacity: '0',
  });
  root.appendChild(textarea);
  container.appendChild(root);

  let current = { ...base, theme: buildTheme(look) };
  let renderer = createRenderer();

  function createRenderer(): CanvasRenderer {
    const r = new CanvasRenderer(t, {
      fontFamily: current.fontFamily,
      fontSize: current.fontSize,
      lineHeight: LINE_HEIGHT,
      theme: current.theme,
      cursorStyle: current.cursorStyle,
    });
    r.canvas.style.display = 'block';
    root.insertBefore(r.canvas, textarea);
    return r;
  }

  function replaceRenderer() {
    renderer.dispose();
    renderer = createRenderer();
  }

  const disposers: (() => void)[] = [];
  function on<K extends keyof HTMLElementEventMap>(
    el: HTMLElement | Window,
    type: K,
    fn: (e: HTMLElementEventMap[K]) => void,
    opts?: AddEventListenerOptions
  ) {
    el.addEventListener(type, fn as EventListener, opts);
    disposers.push(() => el.removeEventListener(type, fn as EventListener, opts));
  }

  const dataSub = t.onData((bytes) => hooks.onData(bytes));

  function scrollToBottom() {
    const offset = t.displayOffset();
    if (offset) t.scrollLines(-offset);
  }

  // ---- Keyboard ------------------------------------------------------------
  let composing = false;

  function flushText() {
    const text = textarea.value;
    textarea.value = '';
    if (!text) return;
    t.input(text);
    scrollToBottom();
  }

  on(textarea, 'keydown', (e) => {
    if (composing || e.isComposing || e.keyCode === 229) return;
    if (hooks.onKey(e)) {
      e.preventDefault();
      return;
    }
    // Scrollback by page; on the alternate screen the keys belong to the application.
    if (e.shiftKey && !e.ctrlKey && !e.altKey && (e.key === 'PageUp' || e.key === 'PageDown') && !t.modes().altScreen) {
      t.scrollLines((e.key === 'PageUp' ? 1 : -1) * Math.max(1, t.options.rows - 1));
      e.preventDefault();
      return;
    }
    // AltGr arrives as Ctrl+Alt on Windows; let the browser type the character (input event).
    if (e.getModifierState?.('AltGraph') && [...e.key].length === 1) return;
    if (handleKeyboardEvent(t, e)) {
      e.preventDefault();
      scrollToBottom();
    }
  });
  on(textarea, 'keyup', (e) => {
    if (!composing && !e.isComposing) handleKeyboardEvent(t, e);
  });
  on(textarea, 'input', (e) => {
    if (!composing && !(e as InputEvent).isComposing) flushText();
  });
  on(textarea, 'compositionstart', () => {
    composing = true;
  });
  on(textarea, 'compositionend', () => {
    composing = false;
    flushText();
  });
  on(textarea, 'paste', (e) => {
    e.preventDefault();
    const text = e.clipboardData?.getData('text/plain');
    if (text) t.paste(text);
  });

  // ---- Mouse ---------------------------------------------------------------
  let selecting = false;
  let selectionMoved = false;
  let downAt: { x: number; y: number } | undefined;
  let reportButton = -1;
  let lastReportCell = '';
  let hoverCell = '';

  function report(kind: MouseReport['kind'], button: number, e: MouseEvent): boolean {
    const { col, row } = renderer.cellAt(e.clientX, e.clientY);
    const bytes = encodeMouseReport(mouseModes, { kind, button, col, row, shift: e.shiftKey, alt: e.altKey, ctrl: e.ctrlKey });
    if (!bytes) return false;
    lastReportCell = `${col},${row}`;
    hooks.onData(bytes);
    return true;
  }

  on(root, 'mousedown', (e) => {
    textarea.focus({ preventScroll: true });
    e.preventDefault();
    // Shift forces a local selection while the application tracks the mouse.
    if (mouseModes.tracking !== 'none' && !e.shiftKey) {
      if (report('press', e.button, e)) reportButton = e.button;
      return;
    }
    if (e.button !== 0) return;
    const { col, row, sideRight } = renderer.cellAt(e.clientX, e.clientY);
    const kind = e.detail >= 3 ? 'line' : e.detail === 2 ? 'word' : e.altKey ? 'block' : 'simple';
    t.selectionBegin(row, col, kind, sideRight);
    selecting = true;
    selectionMoved = kind === 'word' || kind === 'line';
    downAt = { x: e.clientX, y: e.clientY };
  });

  on(window, 'mousemove', (e) => {
    if (selecting) {
      const { col, row, sideRight } = renderer.cellAt(e.clientX, e.clientY);
      if (downAt && Math.hypot(e.clientX - downAt.x, e.clientY - downAt.y) >= 5) selectionMoved = true;
      if (selectionMoved) t.selectionUpdate(row, col, sideRight);
      return;
    }
    if (mouseModes.tracking === 'button' || mouseModes.tracking === 'any') {
      if (reportButton < 0 && (mouseModes.tracking !== 'any' || !root.contains(e.target as Node))) return;
      const { col, row } = renderer.cellAt(e.clientX, e.clientY);
      if (`${col},${row}` !== lastReportCell) report('motion', reportButton, e);
    }
  });

  on(window, 'mouseup', (e) => {
    if (reportButton >= 0) {
      report('release', reportButton, e);
      reportButton = -1;
      return;
    }
    if (!selecting || e.button !== 0) return;
    selecting = false;
    if (!selectionMoved) {
      t.clearSelection();
      const { col, row } = renderer.cellAt(e.clientX, e.clientY);
      const link = t.linkAt(row, col);
      if (link) hooks.openLink(link.uri);
    }
    downAt = undefined;
    hooks.onSelectionChange();
  });

  on(root, 'mousemove', (e) => {
    const { col, row } = renderer.cellAt(e.clientX, e.clientY);
    const cell = `${col},${row}`;
    if (cell === hoverCell) return;
    hoverCell = cell;
    const link = mouseModes.tracking === 'none' ? t.linkAt(row, col) : undefined;
    renderer.setHoverLink(link ? { line: row, startCol: link.startCol, endCol: link.endCol } : null);
    root.style.cursor = link ? 'pointer' : '';
  });
  on(root, 'mouseleave', () => {
    hoverCell = '';
    renderer.setHoverLink(null);
    root.style.cursor = '';
  });

  // Ctrl+wheel (zoom) bubbles to TerminalView; everything else scrolls here.
  // Rio sends wheel reports or arrow keys (alternate screen) itself.
  let wheelRemainder = 0;
  on(
    root,
    'wheel',
    (e) => {
      if (e.ctrlKey) return;
      e.preventDefault();
      const lines =
        e.deltaMode === WheelEvent.DOM_DELTA_LINE
          ? -e.deltaY
          : e.deltaMode === WheelEvent.DOM_DELTA_PAGE
            ? -e.deltaY * t.options.rows
            : -e.deltaY / renderer.cellHeight;
      wheelRemainder += lines;
      const whole = Math.trunc(wheelRemainder);
      wheelRemainder -= whole;
      if (whole === 0) return;
      const { col, row } = renderer.cellAt(e.clientX, e.clientY);
      t.scrollWheel(whole, col, row, (e.shiftKey ? MOD_SHIFT : 0) | (e.altKey ? MOD_ALT : 0) | (e.ctrlKey ? MOD_CTRL : 0));
    },
    { passive: false }
  );

  // ---- Find bar --------------------------------------------------------------
  let lastMatch: SearchMatch | undefined;
  let lastPattern = '';

  function find(text: string, opts: SearchOptions, backward: boolean): boolean {
    const pattern = rioSearchPattern(text, opts);
    const changed = pattern !== lastPattern;
    lastPattern = pattern ?? '';
    const matches = pattern ? t.search(pattern, MAX_SEARCH_MATCHES) : [];
    if (matches.length === 0) {
      lastMatch = undefined;
      return false;
    }

    const rows = t.options.rows;
    const history = t.historySize();
    const offset = t.displayOffset();
    const viewportTop = history - offset;
    // Continue from the current match; a changed query re-checks its position (incremental typing).
    const from = lastMatch
      ? { line: lastMatch.startLine, col: lastMatch.startCol }
      : backward
        ? { line: viewportTop + rows, col: 0 }
        : { line: viewportTop, col: -1 };
    const match = nextMatch(matches, from, backward, !!lastMatch && changed)!;
    lastMatch = match;

    // Scroll the match into view (centred), then select it in viewport coordinates.
    let target = offset;
    const viewportLine = match.startLine - viewportTop;
    if (viewportLine < 0 || viewportLine >= rows) {
      target = Math.min(Math.max(history - match.startLine + Math.floor(rows / 2), 0), history);
      t.scrollLines(target - offset);
    }
    const top = history - target;
    t.selectionBegin(match.startLine - top, match.startCol);
    t.selectionUpdate(match.endLine - top, match.endCol, true);
    hooks.onSelectionChange();
    return true;
  }

  function write(data: Uint8Array) {
    mouseModes.feed(data);
    t.write(data);
  }

  return {
    engine: 'rio',
    get cols() {
      return t.options.cols;
    },
    get rows() {
      return t.options.rows;
    },
    get element() {
      return root;
    },
    search: {
      findNext: (text, opts) => find(text, opts, false),
      findPrevious: (text, opts) => find(text, opts, true),
    },
    write(data, callback) {
      write(data);
      callback?.();
    },
    paste(text) {
      t.paste(text);
      scrollToBottom();
    },
    getSelection: () => t.getSelection() ?? '',
    focus: () => textarea.focus({ preventScroll: true }),
    reset() {
      t.clearSelection();
      write(new TextEncoder().encode('\x1bc'));
    },
    clear() {
      // Screen and scrollback, cursor home (xterm.js-like "clear").
      write(new TextEncoder().encode('\x1b[H\x1b[2J\x1b[3J'));
    },
    fit() {
      const width = root.clientWidth;
      const height = root.clientHeight;
      if (!width || !height) return undefined;
      renderer.fit(width, height);
      return { cols: t.options.cols, rows: t.options.rows };
    },
    setFontSize(px) {
      if (px === current.fontSize) return;
      current = { ...current, fontSize: px };
      replaceRenderer();
    },
    setLook(l, fontPx) {
      const next = {
        ...current,
        fontFamily: fontFamilyCss(l.Font?.Family),
        fontSize: fontPx,
        cursorStyle: buildTerminalOptions(l).cursorStyle,
        theme: buildTheme(l),
      };
      if (JSON.stringify(next) === JSON.stringify(current)) return;
      current = next;
      replaceRenderer();
    },
    lines(max) {
      return t.dump().split('\n').slice(0, max);
    },
    dispose() {
      for (const dispose of disposers) dispose();
      dataSub.dispose();
      renderer.dispose();
      t.dispose();
      root.remove();
    },
  };
}
