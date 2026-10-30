import { describe, expect, it } from 'vitest';
import { ChangeSet, EditorSelection, EditorState, type StateCommand, type Transaction } from '@codemirror/state';
import { indentUnit } from '@codemirror/language';
import {
  addCursorsToLineEnds,
  alignOn,
  applyWordWrap,
  bookmarkField,
  bookmarkLines,
  capitalize,
  cleanIndentation,
  clearModifiedLines,
  convertIndentationToSpaces,
  convertIndentationToTabs,
  deselect,
  joinLines,
  lowerCase,
  modifiedLinesField,
  navigationExtensions,
  upperCase,
  wrapLine,
} from './commands';
import { LocationHistory } from './navigation';

function stateOf(doc: string, selection?: EditorSelection, tabs = false): EditorState {
  return EditorState.create({
    doc,
    selection,
    extensions: [
      navigationExtensions(),
      EditorState.allowMultipleSelections.of(true),
      EditorState.tabSize.of(4),
      indentUnit.of(tabs ? '\t' : '    '),
    ],
  });
}

/** Runs a command and returns the new state (or the old one if it did nothing). */
function exec(cmd: StateCommand, state: EditorState): EditorState {
  let out = state;
  cmd({ state, dispatch: (tr: Transaction) => (out = tr.state) });
  return out;
}

const sel = (anchor: number, head = anchor) => EditorSelection.single(anchor, head);

describe('selection commands', () => {
  it('deselect keeps only the main cursor', () => {
    const s = exec(deselect, stateOf('hello world', sel(0, 5)));
    expect(s.selection.ranges).toHaveLength(1);
    expect(s.selection.main.empty).toBe(true);
    expect(s.selection.main.head).toBe(5);
  });

  it('adds cursors to the ends of the selected lines', () => {
    const s = exec(addCursorsToLineEnds, stateOf('ab\ncde\nf', sel(1, 8)));
    expect(s.selection.ranges.map((r) => r.head)).toEqual([2, 6, 8]);
    // A selection that ends at a line start leaves that line out.
    expect(exec(addCursorsToLineEnds, stateOf('ab\ncde\nf', sel(1, 7))).selection.ranges.map((r) => r.head)).toEqual([2, 6]);
  });

  it('changes case like KWrite', () => {
    expect(exec(upperCase, stateOf('abc', sel(0, 3))).doc.toString()).toBe('ABC');
    expect(exec(lowerCase, stateOf('ABC', sel(0, 3))).doc.toString()).toBe('abc');
    // Without a selection: the character right of the cursor, cursor moves on.
    const u = exec(upperCase, stateOf('abc', sel(1)));
    expect(u.doc.toString()).toBe('aBc');
    expect(u.selection.main.head).toBe(2);
    expect(exec(capitalize, stateOf('hello big world', sel(0, 15))).doc.toString()).toBe('Hello Big World');
    expect(exec(capitalize, stateOf('hello world', sel(8))).doc.toString()).toBe('hello World');
  });

  it('joins lines', () => {
    expect(exec(joinLines, stateOf('a\n    b\nc', sel(0))).doc.toString()).toBe('a b\nc');
    expect(exec(joinLines, stateOf('a\n  b\n  c', sel(0, 9))).doc.toString()).toBe('a b c');
    expect(exec(joinLines, stateOf('a \nb', sel(0))).doc.toString()).toBe('a b');
  });
});

describe('indentation', () => {
  const mixed = '\tone\n    two\n  \tthree';

  it('converts indentation to spaces and tabs', () => {
    expect(exec(convertIndentationToSpaces, stateOf(mixed)).doc.toString()).toBe('    one\n    two\n    three');
    expect(exec(convertIndentationToTabs, stateOf(mixed)).doc.toString()).toBe('\tone\n\ttwo\n\tthree');
  });

  it('cleans indentation according to the indent setting', () => {
    expect(exec(cleanIndentation, stateOf(mixed, undefined, false)).doc.toString()).toBe('    one\n    two\n    three');
    expect(exec(cleanIndentation, stateOf(mixed, undefined, true)).doc.toString()).toBe('\tone\n\ttwo\n\tthree');
  });

  it('only touches selected lines when there is a selection', () => {
    const s = exec(convertIndentationToSpaces, stateOf('\ta\n\tb', sel(0, 2)));
    expect(s.doc.toString()).toBe('    a\n\tb');
  });
});

describe('align on and word wrap', () => {
  it('aligns on a pattern', () => {
    const s = exec(alignOn('='), stateOf('a = 1\nlong = 2\nxy= 3'));
    expect(s.doc.toString()).toBe('a    = 1\nlong = 2\nxy   = 3');
  });

  it('aligns on the first group of a regular expression', () => {
    const s = exec(alignOn(':\\s*(\\S)'), stateOf('a: 1\nbbb:  2'));
    expect(s.doc.toString()).toBe('a:    1\nbbb:  2');
  });

  it('treats an invalid regular expression as text', () => {
    const s = exec(alignOn('('), stateOf('f(x\nfoo(y'));
    expect(s.doc.toString()).toBe('f  (x\nfoo(y');
  });

  it('wraps long lines at whitespace and keeps the indentation', () => {
    expect(wrapLine('  aaa bbb ccc ddd', 10, 4)).toEqual(['  aaa bbb', '  ccc ddd']);
    expect(wrapLine('short', 10, 4)).toEqual(['short']);
    expect(wrapLine('averyveryverylongword x', 10, 4)).toEqual(['averyveryverylongword', 'x']);
    const s = exec(applyWordWrap(10), stateOf('aaa bbb ccc ddd', sel(0)));
    expect(s.doc.toString()).toBe('aaa bbb\nccc ddd');
  });
});

describe('bookmarks and modified lines', () => {
  it('toggles bookmarks that follow edits', async () => {
    const { toggleBookmark, gotoBookmark } = await import('./commands');
    let state = stateOf('one\ntwo\nthree\nfour', sel(4));
    const view = {
      get state() {
        return state;
      },
      dispatch: (spec: any) => {
        state = state.update(spec).state;
      },
    } as any;
    toggleBookmark(view);
    expect(bookmarkLines(state)).toEqual([2]);
    state = state.update({ changes: { from: 0, insert: 'zero\n' } }).state;
    expect(bookmarkLines(state)).toEqual([3]);
    state = state.update({ selection: { anchor: 0 } }).state;
    gotoBookmark(1)(view);
    expect(state.doc.lineAt(state.selection.main.head).number).toBe(3);
    state = state.update({ selection: { anchor: state.doc.line(3).from } }).state;
    toggleBookmark(view);
    expect(bookmarkLines(state)).toEqual([]);
    expect(state.field(bookmarkField).size).toBe(0);
  });

  it('marks changed lines until they are cleared', () => {
    let state = stateOf('a\nb\nc\nd');
    state = state.update({ changes: { from: state.doc.line(3).from, insert: 'x' } }).state;
    const lines = () => {
      const out: number[] = [];
      for (let it = state.field(modifiedLinesField).iter(); it.value; it.next()) out.push(state.doc.lineAt(it.from).number);
      return out;
    };
    expect(lines()).toEqual([3]);
    state = state.update({ changes: { from: 0, insert: 'new\n' } }).state;
    expect(lines()).toEqual([1, 4]);
    state = state.update({ effects: clearModifiedLines.of() }).state;
    expect(lines()).toEqual([]);
  });
});

describe('location history', () => {
  const same = (a: { tab: number; pos: number }, b: { tab: number; pos: number }) => a.tab === b.tab && a.pos === b.pos;

  it('goes back and forward through jumps', () => {
    const h = new LocationHistory(same);
    h.jump({ tab: 1, pos: 0 }, { tab: 1, pos: 100 });
    h.jump({ tab: 1, pos: 100 }, { tab: 2, pos: 5 });
    expect(h.back({ tab: 2, pos: 5 })).toEqual({ tab: 1, pos: 100 });
    expect(h.back({ tab: 1, pos: 100 })).toEqual({ tab: 1, pos: 0 });
    expect(h.back({ tab: 1, pos: 0 })).toBeUndefined();
    expect(h.forward()).toEqual({ tab: 1, pos: 100 });
    expect(h.forward()).toEqual({ tab: 2, pos: 5 });
    expect(h.forward()).toBeUndefined();
  });

  it('returns to where the cursor was after going back', () => {
    const h = new LocationHistory(same);
    h.edit({ tab: 1, pos: 10 });
    expect(h.back({ tab: 1, pos: 50 })).toEqual({ tab: 1, pos: 10 });
    expect(h.forward()).toEqual({ tab: 1, pos: 50 });
  });

  it('maps positions through changes and forgets closed tabs', () => {
    const h = new LocationHistory(same);
    h.jump({ tab: 1, pos: 5 }, { tab: 2, pos: 7 });
    h.map(1, ChangeSet.of({ from: 0, insert: 'abc' }, 20));
    expect(h.entries[0]).toEqual({ tab: 1, pos: 8 });
    h.removeTab(1);
    expect(h.entries).toEqual([{ tab: 2, pos: 7 }]);
    expect(h.position).toBe(0);
  });
});
