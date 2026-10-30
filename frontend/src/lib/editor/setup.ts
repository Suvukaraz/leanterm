// CodeMirror configuration of the editor window. Settings that change at runtime
// (language, tab width, wrapping, whitespace, theme) sit in compartments.
import { Compartment, EditorState, StateEffect, StateField, type Extension } from '@codemirror/state';
import {
  EditorView,
  ViewPlugin,
  crosshairCursor,
  drawSelection,
  dropCursor,
  gutters,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  highlightWhitespace,
  keymap,
  lineNumbers,
  rectangularSelection,
  type PluginValue,
  type ViewUpdate,
} from '@codemirror/view';
import {
  addCursorAbove,
  addCursorBelow,
  cursorMatchingBracket,
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
  selectMatchingBracket,
} from '@codemirror/commands';
import { bracketMatching, foldGutter, foldKeymap, indentOnInput, indentUnit } from '@codemirror/language';
import {
  gotoLine,
  highlightSelectionMatches,
  openSearchPanel,
  search,
  searchKeymap,
  selectNextOccurrence,
  selectSelectionMatches,
} from '@codemirror/search';
import { editorTheme, leanHighlighting } from './theme';
import { minimap } from './minimap';
import {
  addCursorsToLineEnds,
  blockMouseFilter,
  capitalize,
  deselect,
  extendBlock,
  gotoBookmark,
  joinLines,
  lowerCase,
  navigationExtensions,
  toggleBlockMode,
  toggleBookmark,
  upperCase,
} from './commands';

export { lowerCase, upperCase } from './commands';

export interface ViewPrefs {
  tabWidth: number;
  indentWithTabs: boolean;
  wordWrap: boolean;
  showWhitespace: boolean;
  showMinimap: boolean;
  dark: boolean;
}

export const languageConf = new Compartment();
export const highlightConf = new Compartment();
export const indentConf = new Compartment();
export const wrapConf = new Compartment();
export const whitespaceConf = new Compartment();
export const minimapConf = new Compartment();
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

/**
 * The gutters scroll with the content instead of using position: sticky. WebKitGTK
 * moves a sticky layer a frame after the scrolled text, so the line numbers
 * flickered while scrolling. Horizontally they are kept in place by this plugin.
 */
const gutterStayLeft = ViewPlugin.fromClass(
  class implements PluginValue {
    x = 0;
    constructor(readonly view: EditorView) {
      view.scrollDOM.addEventListener('scroll', this.onScroll, { passive: true });
    }
    onScroll = () => {
      const x = this.view.scrollDOM.scrollLeft;
      if (x === this.x) return;
      this.x = x;
      const el = this.view.dom.querySelector<HTMLElement>('.cm-gutters');
      if (el) el.style.transform = x ? `translateX(${x}px)` : '';
    };
    update(u: ViewUpdate) {
      // The gutter element is rebuilt on reconfiguration.
      if (u.transactions.some((tr) => tr.reconfigured)) {
        this.x = -1;
        this.onScroll();
      }
    }
    destroy() {
      this.view.scrollDOM.removeEventListener('scroll', this.onScroll);
    }
  }
);

export function indentExtension(p: Pick<ViewPrefs, 'tabWidth' | 'indentWithTabs'>): Extension {
  return [EditorState.tabSize.of(p.tabWidth), indentUnit.of(p.indentWithTabs ? '\t' : ' '.repeat(p.tabWidth))];
}

export const wrapExtension = (on: boolean): Extension => (on ? EditorView.lineWrapping : []);
export const whitespaceExtension = (on: boolean): Extension => (on ? highlightWhitespace() : []);
export const minimapExtension = (on: boolean): Extension => (on ? minimap() : []);

/**
 * Builds the extensions of a new document state. `onUpdate` sees every update
 * (status bar, unsaved marker); `language` may be filled in later via languageConf.
 */
export function editorExtensions(prefs: ViewPrefs, onUpdate: (u: import('@codemirror/view').ViewUpdate) => void, highlight = true): Extension {
  return [
    gutters({ fixed: false }),
    gutterStayLeft,
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
    rectangularSelection({ eventFilter: blockMouseFilter }),
    crosshairCursor(),
    highlightActiveLine(),
    highlightSelectionMatches(),
    search({ top: true }),
    overwriteField,
    overwriteInput,
    navigationExtensions(),
    keymap.of([
      // KWrite-like keys that differ from CodeMirror's defaults.
      { key: 'Mod-g', run: gotoLine, preventDefault: true },
      { key: 'Mod-r', run: openSearchPanel, preventDefault: true },
      { key: 'Mod-h', run: openSearchPanel, preventDefault: true },
      // Selection menu
      { key: 'Mod-Shift-a', run: deselect, preventDefault: true },
      {
        key: 'Mod-Shift-b',
        run: (view) => {
          view.dispatch({ effects: toggleBlockMode.of() });
          return true;
        },
        preventDefault: true,
      },
      { key: 'Shift-ArrowUp', run: extendBlock('up') },
      { key: 'Shift-ArrowDown', run: extendBlock('down') },
      { key: 'Shift-ArrowLeft', run: extendBlock('left') },
      { key: 'Shift-ArrowRight', run: extendBlock('right') },
      { key: 'Shift-Home', run: extendBlock('home') },
      { key: 'Shift-End', run: extendBlock('end') },
      { key: 'Mod-j', run: joinLines, preventDefault: true },
      { key: 'Mod-u', run: upperCase, preventDefault: true },
      { key: 'Mod-Shift-u', run: lowerCase, preventDefault: true },
      { key: 'Mod-Alt-u', run: capitalize, preventDefault: true },
      { key: 'Mod-Alt-ArrowUp', run: addCursorAbove, preventDefault: true },
      { key: 'Mod-Alt-ArrowDown', run: addCursorBelow, preventDefault: true },
      { key: 'Alt-Shift-i', run: addCursorsToLineEnds, preventDefault: true },
      { key: 'Alt-j', run: selectNextOccurrence, preventDefault: true },
      { key: 'Mod-Alt-Shift-j', run: selectSelectionMatches, preventDefault: true },
      // Go menu (tab and location navigation lives in the editor window)
      { key: 'Mod-6', run: cursorMatchingBracket, preventDefault: true },
      { key: 'Mod-Shift-6', run: selectMatchingBracket, preventDefault: true },
      { key: 'Mod-b', run: toggleBookmark, preventDefault: true },
      { key: 'Alt-PageDown', run: gotoBookmark(1), preventDefault: true },
      { key: 'Alt-PageUp', run: gotoBookmark(-1), preventDefault: true },
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
    minimapConf.of(minimapExtension(prefs.showMinimap)),
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
    minimapConf.reconfigure(minimapExtension(prefs.showMinimap)),
    themeConf.reconfigure(editorTheme(prefs.dark)),
  ];
}

export function highlightEffect(on: boolean): StateEffect<unknown> {
  return highlightConf.reconfigure(on ? leanHighlighting : []);
}
