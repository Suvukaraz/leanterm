// CodeMirror configuration of the editor window. Settings that change at runtime
// (language, tab width, wrapping, whitespace, theme) sit in compartments.
import { Compartment, EditorSelection, EditorState, StateEffect, StateField, type Extension, type StateCommand } from '@codemirror/state';
import {
  EditorView,
  crosshairCursor,
  drawSelection,
  dropCursor,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  highlightWhitespace,
  keymap,
  lineNumbers,
  rectangularSelection,
} from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { bracketMatching, foldGutter, foldKeymap, indentOnInput, indentUnit } from '@codemirror/language';
import { gotoLine, highlightSelectionMatches, openSearchPanel, search, searchKeymap } from '@codemirror/search';
import { editorTheme, leanHighlighting } from './theme';

export interface ViewPrefs {
  tabWidth: number;
  indentWithTabs: boolean;
  wordWrap: boolean;
  showWhitespace: boolean;
  dark: boolean;
}

export const languageConf = new Compartment();
export const highlightConf = new Compartment();
export const indentConf = new Compartment();
export const wrapConf = new Compartment();
export const whitespaceConf = new Compartment();
export const themeConf = new Compartment();

// ---- Overwrite mode (Insert key, "OVR" in the status bar) ------------------------

export const toggleOverwrite = StateEffect.define<void>();

export const overwriteField = StateField.define<boolean>({
  create: () => false,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(toggleOverwrite)) value = !value;
    return value;
  },
  provide: (f) => EditorView.editorAttributes.from(f, (on): Record<string, string> => (on ? { class: 'cm-overwrite' } : {})),
});

const overwriteInput = EditorView.inputHandler.of((view, from, to, text) => {
  if (!view.state.field(overwriteField) || from !== to || text.includes('\n')) return false;
  const line = view.state.doc.lineAt(from);
  const end = Math.min(from + text.length, line.to);
  if (end === from) return false;
  view.dispatch({
    changes: { from, to: end, insert: text },
    selection: { anchor: from + text.length },
    userEvent: 'input.type',
  });
  return true;
});

function changeCase(upper: boolean): StateCommand {
  return ({ state, dispatch }) => {
    if (state.selection.ranges.every((r) => r.empty)) return false;
    dispatch(
      state.update(
        state.changeByRange((r) => {
          const text = state.sliceDoc(r.from, r.to);
          const insert = upper ? text.toUpperCase() : text.toLowerCase();
          return { changes: { from: r.from, to: r.to, insert }, range: EditorSelection.range(r.from, r.from + insert.length) };
        }),
        { userEvent: 'input' }
      )
    );
    return true;
  };
}

/** KWrite's Ctrl+U / Ctrl+Shift+U on the selection. */
export const upperCase = changeCase(true);
export const lowerCase = changeCase(false);

export function indentExtension(p: Pick<ViewPrefs, 'tabWidth' | 'indentWithTabs'>): Extension {
  return [EditorState.tabSize.of(p.tabWidth), indentUnit.of(p.indentWithTabs ? '\t' : ' '.repeat(p.tabWidth))];
}

export const wrapExtension = (on: boolean): Extension => (on ? EditorView.lineWrapping : []);
export const whitespaceExtension = (on: boolean): Extension => (on ? highlightWhitespace() : []);

/**
 * Builds the extensions of a new document state. `onUpdate` sees every update
 * (status bar, unsaved marker); `language` may be filled in later via languageConf.
 */
export function editorExtensions(prefs: ViewPrefs, onUpdate: (u: import('@codemirror/view').ViewUpdate) => void, highlight = true): Extension {
  return [
    lineNumbers(),
    highlightActiveLineGutter(),
    highlightSpecialChars(),
    history(),
    foldGutter(),
    drawSelection(),
    dropCursor(),
    EditorState.allowMultipleSelections.of(true),
    indentOnInput(),
    bracketMatching(),
    rectangularSelection(),
    crosshairCursor(),
    highlightActiveLine(),
    highlightSelectionMatches(),
    search({ top: true }),
    overwriteField,
    overwriteInput,
    keymap.of([
      // KWrite-like keys that differ from CodeMirror's defaults.
      { key: 'Mod-g', run: gotoLine, preventDefault: true },
      { key: 'Mod-r', run: openSearchPanel, preventDefault: true },
      { key: 'Mod-h', run: openSearchPanel, preventDefault: true },
      { key: 'Mod-u', run: upperCase, preventDefault: true },
      { key: 'Mod-Shift-u', run: lowerCase, preventDefault: true },
      {
        key: 'Insert',
        run: (view) => {
          view.dispatch({ effects: toggleOverwrite.of() });
          return true;
        },
      },
      ...defaultKeymap,
      ...searchKeymap,
      ...historyKeymap,
      ...foldKeymap,
      indentWithTab,
    ]),
    languageConf.of([]),
    highlightConf.of(highlight ? leanHighlighting : []),
    indentConf.of(indentExtension(prefs)),
    wrapConf.of(wrapExtension(prefs.wordWrap)),
    whitespaceConf.of(whitespaceExtension(prefs.showWhitespace)),
    themeConf.of(editorTheme(prefs.dark)),
    EditorView.updateListener.of(onUpdate),
  ];
}

/** Effects that apply changed preferences to an existing state. */
export function prefsEffects(prefs: ViewPrefs): StateEffect<unknown>[] {
  return [
    indentConf.reconfigure(indentExtension(prefs)),
    wrapConf.reconfigure(wrapExtension(prefs.wordWrap)),
    whitespaceConf.reconfigure(whitespaceExtension(prefs.showWhitespace)),
    themeConf.reconfigure(editorTheme(prefs.dark)),
  ];
}

export function highlightEffect(on: boolean): StateEffect<unknown> {
  return highlightConf.reconfigure(on ? leanHighlighting : []);
}
