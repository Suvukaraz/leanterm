// KWrite's "Selection" and "Go" menu commands that CodeMirror has no built-in for:
// case changes, join lines, indentation clean-up, align on, hard word wrap, block
// selection mode, cursors at line ends, bookmarks and modified-line tracking.
import {
  EditorSelection,
  RangeSet,
  StateEffect,
  StateField,
  countColumn,
  findColumn,
  type ChangeSpec,
  type EditorState,
  type Extension,
  type Line,
  type SelectionRange,
  type StateCommand,
} from '@codemirror/state';
import { Decoration, EditorView, GutterMarker, gutterLineClass, type Command } from '@codemirror/view';
import { indentUnit } from '@codemirror/language';

// ---- Helpers -------------------------------------------------------------------------------------

/** Lines touched by the selection (a range ending at a line start leaves that line out). */
function selectedLines(state: EditorState, wholeDocWhenEmpty: boolean): Line[] {
  const doc = state.doc;
  if (wholeDocWhenEmpty && state.selection.ranges.every((r) => r.empty)) {
    const all: Line[] = [];
    for (let n = 1; n <= doc.lines; n++) all.push(doc.line(n));
    return all;
  }
  const seen = new Set<number>();
  const out: Line[] = [];
  for (const r of state.selection.ranges) {
    const first = doc.lineAt(r.from).number;
    let last = doc.lineAt(r.to).number;
    if (last > first && doc.line(last).from === r.to) last--;
    for (let n = first; n <= last; n++) {
      if (seen.has(n)) continue;
      seen.add(n);
      out.push(doc.line(n));
    }
  }
  return out.sort((a, b) => a.number - b.number);
}

function dispatchChanges(state: EditorState, dispatch: (tr: ReturnType<EditorState['update']>) => void, changes: ChangeSpec[], userEvent: string): boolean {
  if (changes.length === 0) return false;
  dispatch(state.update({ changes, userEvent, scrollIntoView: true }));
  return true;
}

// ---- Selection ------------------------------------------------------------------------------------

/** KWrite's Deselect (Ctrl+Shift+A): keeps only the main cursor. */
export const deselect: StateCommand = ({ state, dispatch }) => {
  const main = state.selection.main;
  if (state.selection.ranges.length === 1 && main.empty) return false;
  dispatch(state.update({ selection: EditorSelection.cursor(main.head), userEvent: 'select' }));
  return true;
};

/** Add Cursors to Line Ends (Alt+Shift+I): one cursor at the end of every selected line. */
export const addCursorsToLineEnds: StateCommand = ({ state, dispatch }) => {
  const lines = selectedLines(state, false);
  if (lines.length === 0) return false;
  const ranges = lines.map((l) => EditorSelection.cursor(l.to));
  dispatch(state.update({ selection: EditorSelection.create(ranges, ranges.length - 1), userEvent: 'select', scrollIntoView: true }));
  return true;
};

// ---- Capitalisation ---------------------------------------------------------------------------

export type CaseMode = 'upper' | 'lower' | 'capitalize';

function transformCase(text: string, mode: CaseMode): string {
  if (mode === 'upper') return text.toUpperCase();
  if (mode === 'lower') return text.toLowerCase();
  return text.replace(/(^|[^\p{L}\p{N}_])(\p{L})/gu, (_m, before: string, letter: string) => before + letter.toUpperCase());
}

/**
 * Changes the case of the selection. Without a selection KWrite changes the
 * character right of the cursor (upper/lower case) or the word under it (capitalize).
 */
export function changeCase(mode: CaseMode): StateCommand {
  return ({ state, dispatch }) => {
    const tr = state.update(
      state.changeByRange((r) => {
        let from = r.from;
        let to = r.to;
        let moveAfter = false;
        if (r.empty) {
          const word = state.wordAt(r.head);
          if (mode === 'capitalize') {
            if (!word) return { range: r };
            from = word.from;
            to = word.to;
          } else {
            const line = state.doc.lineAt(r.head);
            if (r.head >= line.to) return { range: r };
            to = r.head + 1;
            moveAfter = true;
          }
        }
        const text = state.sliceDoc(from, to);
        const insert = transformCase(text, mode);
        const range = r.empty
          ? EditorSelection.cursor(moveAfter ? from + insert.length : r.head)
          : EditorSelection.range(r.anchor === r.from ? from : from + insert.length, r.anchor === r.from ? from + insert.length : from);
        return { changes: insert === text ? [] : { from, to, insert }, range };
      }),
      { userEvent: 'input', scrollIntoView: true }
    );
    if (tr.changes.empty && tr.selection?.eq(state.selection)) return false;
    dispatch(tr);
    return true;
  };
}

export const upperCase = changeCase('upper');
export const lowerCase = changeCase('lower');
export const capitalize = changeCase('capitalize');

// ---- Join lines --------------------------------------------------------------------------------

/** Join Lines (Ctrl+J): joins the selected lines, or the cursor line with the next one. */
export const joinLines: StateCommand = ({ state, dispatch }) => {
  const doc = state.doc;
  const breaks = new Set<number>();
  for (const r of state.selection.ranges) {
    const first = doc.lineAt(r.from).number;
    let last = doc.lineAt(r.to).number;
    if (last === first) last = first + 1;
    for (let n = first; n < last && n < doc.lines; n++) breaks.add(n);
  }
  const changes: ChangeSpec[] = [];
  for (const n of [...breaks].sort((a, b) => a - b)) {
    const line = doc.line(n);
    const next = doc.line(n + 1);
    const indent = /^[ \t]*/.exec(next.text)![0].length;
    const needsSpace = line.text.length > 0 && !/[ \t]$/.test(line.text) && next.text.length > indent;
    changes.push({ from: line.to, to: next.from + indent, insert: needsSpace ? ' ' : '' });
  }
  return dispatchChanges(state, dispatch, changes, 'input');
};

// ---- Indentation -----------------------------------------------------------------------------

function indentWidth(text: string, tabSize: number): number {
  return countColumn(/^[ \t]*/.exec(text)![0], tabSize);
}

function buildIndent(width: number, useTabs: boolean, tabSize: number): string {
  return useTabs ? '\t'.repeat(Math.floor(width / tabSize)) + ' '.repeat(width % tabSize) : ' '.repeat(width);
}

/** Rewrites the leading whitespace of the selected lines (all lines without a selection). */
function convertIndentation(useTabs: (state: EditorState) => boolean): StateCommand {
  return ({ state, dispatch }) => {
    const tabs = useTabs(state);
    const changes: ChangeSpec[] = [];
    for (const line of selectedLines(state, true)) {
      const current = /^[ \t]*/.exec(line.text)![0];
      if (!current) continue;
      const next = buildIndent(indentWidth(line.text, state.tabSize), tabs, state.tabSize);
      if (next !== current) changes.push({ from: line.from, to: line.from + current.length, insert: next });
    }
    return dispatchChanges(state, dispatch, changes, 'input.indent');
  };
}

export const convertIndentationToSpaces = convertIndentation(() => false);
export const convertIndentationToTabs = convertIndentation(() => true);
/** Clean Indentation: only tabs or only spaces, as configured (Indent with tabs). */
export const cleanIndentation = convertIndentation((state) => state.facet(indentUnit).includes('\t'));

// ---- Align On ------------------------------------------------------------------------------------

/** Builds the matcher for Align On: a regular expression, or literal text if it is not valid. */
export function alignPattern(pattern: string): RegExp | null {
  if (!pattern) return null;
  try {
    return new RegExp(pattern, 'd');
  } catch {
    return new RegExp(pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'd');
  }
}

/**
 * Align On…: pads the selected lines (all lines without a selection) so that the
 * first match of `pattern` (or of its first group) starts in the same column.
 */
export function alignOn(pattern: string): StateCommand {
  return ({ state, dispatch }) => {
    const re = alignPattern(pattern);
    if (!re) return false;
    const tabSize = state.tabSize;
    const hits: Array<{ line: Line; offset: number; column: number }> = [];
    for (const line of selectedLines(state, true)) {
      const m = re.exec(line.text);
      if (!m) continue;
      const offset = m.indices?.[1]?.[0] ?? m.index;
      hits.push({ line, offset, column: countColumn(line.text, tabSize, offset) });
    }
    if (hits.length < 2) return false;
    const target = Math.max(...hits.map((h) => h.column));
    const changes: ChangeSpec[] = hits
      .filter((h) => h.column < target)
      .map((h) => ({ from: h.line.from + h.offset, insert: ' '.repeat(target - h.column) }));
    return dispatchChanges(state, dispatch, changes, 'input');
  };
}

// ---- Apply Word Wrap -------------------------------------------------------------------------------

/** Breaks one line at whitespace so no part is longer than `width` columns (long words stay). */
export function wrapLine(text: string, width: number, tabSize: number): string[] {
  const indent = /^[ \t]*/.exec(text)![0];
  const out: string[] = [];
  let rest = text;
  while (countColumn(rest, tabSize) > width) {
    const limit = findColumn(rest, width, tabSize);
    const head = rest.slice(0, limit + 1);
    let cut = -1;
    for (let i = head.length - 1; i > indent.length; i--) {
      if (head[i] === ' ' || head[i] === '\t') {
        cut = i;
        break;
      }
    }
    if (cut < 0) {
      // A word longer than the width: break after it.
      const m = /[ \t]/.exec(rest.slice(limit));
      if (!m) break;
      cut = limit + m.index;
    }
    out.push(rest.slice(0, cut).replace(/[ \t]+$/, ''));
    rest = indent + rest.slice(cut).replace(/^[ \t]+/, '');
    if (rest.trim() === '') break;
  }
  out.push(rest);
  return out;
}

/** Apply Word Wrap: hard-wraps the selected lines (or the cursor line) at `width` columns. */
export function applyWordWrap(width = 80): StateCommand {
  return ({ state, dispatch }) => {
    const changes: ChangeSpec[] = [];
    for (const line of selectedLines(state, false)) {
      const parts = wrapLine(line.text, width, state.tabSize);
      if (parts.length > 1) changes.push({ from: line.from, to: line.to, insert: parts.join(state.lineBreak) });
    }
    return dispatchChanges(state, dispatch, changes, 'input');
  };
}

// ---- Block selection mode (Ctrl+Shift+B) ---------------------------------------------------------

export const toggleBlockMode = StateEffect.define<void>();

export const blockModeField = StateField.define<boolean>({
  create: () => false,
  update(on, tr) {
    for (const e of tr.effects) if (e.is(toggleBlockMode)) on = !on;
    return on;
  },
  provide: (f) => EditorView.editorAttributes.from(f, (on): Record<string, string> => (on ? { class: 'cm-block-mode' } : {})),
});

/** For rectangularSelection(): plain drags select blocks in block mode, Alt+drag always. */
export function blockMouseFilter(e: MouseEvent): boolean {
  return e.altKey || !!(e.target as Element | null)?.closest?.('.cm-editor.cm-block-mode');
}

interface BlockAnchor {
  anchor: { line: number; col: number };
  head: { line: number; col: number };
  selection: EditorSelection;
}
const blockState = new WeakMap<EditorView, BlockAnchor>();

function blockSelection(state: EditorState, a: BlockAnchor['anchor'], h: BlockAnchor['head']): EditorSelection {
  const ranges: SelectionRange[] = [];
  const step = h.line >= a.line ? 1 : -1;
  for (let n = a.line; ; n += step) {
    const line = state.doc.line(n);
    const from = line.from + findColumn(line.text, a.col, state.tabSize);
    const to = line.from + findColumn(line.text, h.col, state.tabSize);
    ranges.push(EditorSelection.range(from, to));
    if (n === h.line) break;
  }
  return EditorSelection.create(ranges, ranges.length - 1);
}

/** Shift+arrow keys in block mode extend a rectangular selection. */
export function extendBlock(dir: 'up' | 'down' | 'left' | 'right' | 'home' | 'end'): Command {
  return (view) => {
    const { state } = view;
    if (!state.field(blockModeField, false)) return false;
    let b = blockState.get(view);
    if (!b || !b.selection.eq(state.selection)) {
      const main = state.selection.main;
      const at = (pos: number) => {
        const line = state.doc.lineAt(pos);
        return { line: line.number, col: countColumn(line.text, state.tabSize, pos - line.from) };
      };
      b = { anchor: at(main.anchor), head: at(main.head), selection: state.selection };
    }
    const head = { ...b.head };
    if (dir === 'up') head.line = Math.max(1, head.line - 1);
    else if (dir === 'down') head.line = Math.min(state.doc.lines, head.line + 1);
    else if (dir === 'left') head.col = Math.max(0, head.col - 1);
    else if (dir === 'right') head.col += 1;
    else if (dir === 'home') head.col = 0;
    else {
      const line = state.doc.line(head.line);
      head.col = countColumn(line.text, state.tabSize);
    }
    const selection = blockSelection(state, b.anchor, head);
    blockState.set(view, { anchor: b.anchor, head, selection });
    view.dispatch({ selection, userEvent: 'select', scrollIntoView: true });
    return true;
  };
}

// ---- Bookmarks (Ctrl+B, Alt+PageUp/PageDown) -----------------------------------------------------

const bookmarkMarker = new (class extends GutterMarker {
  elementClass = 'cm-bookmarked';
})();
const bookmarkLine = Decoration.line({ class: 'cm-bookmark-line' });

const toggleBookmarkAt = StateEffect.define<number>({ map: (pos, mapping) => mapping.mapPos(pos) });
export const clearBookmarks = StateEffect.define<void>();

/** Bookmarked line starts (as decorations; the gutter class is derived from the same set). */
export const bookmarkField = StateField.define<RangeSet<Decoration>>({
  create: () => Decoration.none,
  update(set, tr) {
    set = set.map(tr.changes);
    for (const e of tr.effects) {
      if (e.is(clearBookmarks)) set = Decoration.none;
      else if (e.is(toggleBookmarkAt)) {
        const lineStart = tr.state.doc.lineAt(e.value).from;
        let exists = false;
        set.between(lineStart, lineStart, (from) => {
          if (from === lineStart) exists = true;
        });
        set = exists
          ? set.update({ filter: (from) => from !== lineStart })
          : set.update({ add: [bookmarkLine.range(lineStart)], sort: true });
      }
    }
    return set;
  },
  provide: (f) => [
    EditorView.decorations.from(f),
    gutterLineClass.from(f, (set) => {
      const ranges = [];
      for (let it = set.iter(); it.value; it.next()) ranges.push(bookmarkMarker.range(it.from));
      return RangeSet.of(ranges);
    }),
  ],
});

/** Line numbers (1-based) of all bookmarks. */
export function bookmarkLines(state: EditorState): number[] {
  const set = state.field(bookmarkField, false);
  const out: number[] = [];
  if (!set) return out;
  for (let it = set.iter(); it.value; it.next()) out.push(state.doc.lineAt(it.from).number);
  return [...new Set(out)];
}

export const toggleBookmark: Command = (view) => {
  view.dispatch({ effects: toggleBookmarkAt.of(view.state.selection.main.head) });
  return true;
};

export const clearAllBookmarks: Command = (view) => {
  if (bookmarkLines(view.state).length === 0) return false;
  view.dispatch({ effects: clearBookmarks.of() });
  return true;
};

/** The next (dir 1) or previous (dir -1) line in `lines` after the cursor, wrapping around. */
function nextLineIn(lines: number[], current: number, dir: 1 | -1, wrap: boolean): number | undefined {
  const sorted = [...lines].sort((a, b) => a - b);
  if (dir > 0) return sorted.find((n) => n > current) ?? (wrap ? sorted[0] : undefined);
  return [...sorted].reverse().find((n) => n < current) ?? (wrap ? sorted[sorted.length - 1] : undefined);
}

export function goToLineNumber(view: EditorView, n: number) {
  const line = view.state.doc.line(Math.max(1, Math.min(view.state.doc.lines, n)));
  view.dispatch({
    selection: EditorSelection.cursor(line.from),
    effects: EditorView.scrollIntoView(line.from, { y: 'center' }),
    userEvent: 'select.navigate',
  });
}

export function gotoBookmark(dir: 1 | -1): Command {
  return (view) => {
    const current = view.state.doc.lineAt(view.state.selection.main.head).number;
    const n = nextLineIn(bookmarkLines(view.state), current, dir, true);
    if (n === undefined) return false;
    goToLineNumber(view, n);
    return true;
  };
}

// ---- Modified lines (since load / save) ---------------------------------------------------------------

const modifiedMarker = new (class extends GutterMarker {
  elementClass = 'cm-modified-line';
})();

/** Clears the modified-line marks (after saving or reloading). */
export const clearModifiedLines = StateEffect.define<void>();

export const modifiedLinesField = StateField.define<RangeSet<GutterMarker>>({
  create: () => RangeSet.empty,
  update(set, tr) {
    for (const e of tr.effects) if (e.is(clearModifiedLines)) set = RangeSet.empty;
    if (!tr.docChanged) return set;
    set = set.map(tr.changes);
    const doc = tr.state.doc;
    const add: ReturnType<GutterMarker['range']>[] = [];
    const seen = new Set<number>();
    tr.changes.iterChangedRanges((_fA, _tA, fromB, toB) => {
      const first = doc.lineAt(fromB).number;
      let last = doc.lineAt(toB).number;
      // Whole lines inserted at a line start leave the following line unchanged.
      if (last > first && doc.line(first).from === fromB && doc.line(last).from === toB) last--;
      for (let n = first; n <= last; n++) {
        const from = doc.line(n).from;
        if (!seen.has(from)) {
          seen.add(from);
          add.push(modifiedMarker.range(from));
        }
      }
    });
    // Drop duplicates on the same line start before adding.
    set = set.update({ filter: (from) => !seen.has(from), add, sort: true });
    return set;
  },
  provide: (f) => gutterLineClass.from(f),
});

function modifiedLines(state: EditorState): number[] {
  const set = state.field(modifiedLinesField, false);
  const out: number[] = [];
  if (!set) return out;
  for (let it = set.iter(); it.value; it.next()) out.push(state.doc.lineAt(it.from).number);
  return out;
}

/** Go to Next / Previous Modified Line (no wrap-around, like KWrite). */
export function gotoModifiedLine(dir: 1 | -1): Command {
  return (view) => {
    const current = view.state.doc.lineAt(view.state.selection.main.head).number;
    // Skip the block of modified lines the cursor is in.
    const lines = modifiedLines(view.state);
    const set = new Set(lines);
    let start = current;
    while (set.has(start + dir)) start += dir;
    const n = nextLineIn(lines, start, dir, false);
    if (n === undefined) return false;
    goToLineNumber(view, n);
    return true;
  };
}

/** Bookmarks, modified-line marks and block mode for a document state. */
export function navigationExtensions(): Extension {
  return [bookmarkField, modifiedLinesField, blockModeField];
}
