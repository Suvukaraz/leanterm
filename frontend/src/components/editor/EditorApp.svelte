<script lang="ts">
  // The editor window (#/editor): KWrite-like text editor with one tab per file,
  // built on CodeMirror. Files come from the file browsers via EditorService.Open.
  import { onMount, untrack } from 'svelte';
  import { Clipboard, Events, Window as WailsWindow } from '@wailsio/runtime';
  import { EditorState, Transaction, type Extension, type Text } from '@codemirror/state';
  import { EditorView as CMView, type EditorView, type ViewUpdate } from '@codemirror/view';
  import {
    addCursorAbove,
    addCursorBelow,
    cursorMatchingBracket,
    indentLess,
    indentMore,
    indentSelection,
    redo,
    selectAll,
    selectMatchingBracket,
    toggleComment,
    undo,
  } from '@codemirror/commands';
  import { findNext, findPrevious, gotoLine, openSearchPanel, selectNextOccurrence, selectSelectionMatches } from '@codemirror/search';
  import { foldAll, unfoldAll } from '@codemirror/language';

  import * as EditorService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/editorservice.js';
  import { settings, loadSettings } from '../../lib/stores/config';
  import { readClipboardText } from '../../lib/bridge/clipboard';
  import { setIconTheme } from '../../lib/theme/icons';
  import { fontFamilyCss, ptToPx } from '../../lib/terminal/look';
  import { fontFeaturesCss, parseStyle } from '../../lib/fonts';
  import { LANGUAGES, detectLanguage, languageById, type LanguageGroup } from '../../lib/editor/languages';
  import {
    baseName,
    classifySize,
    displayPath,
    fileKey,
    formatSize,
    loadFile,
    saveFile,
    statFile,
    type EditorFile,
  } from '../../lib/editor/files';
  import {
    editorExtensions,
    highlightEffect,
    languageConf,
    lowerCase,
    overwriteField,
    prefsEffects,
    toggleOverwrite,
    upperCase,
    type ViewPrefs,
  } from '../../lib/editor/setup';
  import {
    addCursorsToLineEnds,
    alignOn,
    applyWordWrap,
    blockModeField,
    bookmarkLines,
    capitalize,
    cleanIndentation,
    clearAllBookmarks,
    clearModifiedLines,
    convertIndentationToSpaces,
    convertIndentationToTabs,
    deselect,
    gotoBookmark,
    goToLineNumber,
    gotoModifiedLine,
    joinLines,
    toggleBlockMode,
    toggleBookmark,
  } from '../../lib/editor/commands';
  import { LocationHistory, type EditorLocation } from '../../lib/editor/navigation';
  import type { MenuItem } from '../gtk/ContextMenu.svelte';
  import ContextMenu from '../gtk/ContextMenu.svelte';
  import Button from '../gtk/Button.svelte';
  import EditorMenuBar, { type MenuDef } from './EditorMenuBar.svelte';
  import EditorPane from './EditorPane.svelte';
  import EditorStatusBar from './EditorStatusBar.svelte';
  import ChoiceDialog, { type Choice } from './ChoiceDialog.svelte';
  import QuickOpen from './QuickOpen.svelte';
  import EnterValueDialog from '../dialogs/EnterValueDialog.svelte';
  import '../../lib/theme/gtk.css';

  type LineEnding = 'lf' | 'crlf';

  interface Tab {
    id: number;
    file: EditorFile | null;
    title: string;
    /** Bumped when the document is replaced (reload); re-creates the view. */
    generation: number;
    initialState: EditorState | null;
    savedDoc: Text | null;
    dirty: boolean;
    encoding: string;
    savedEncoding: string;
    lineEnding: LineEnding;
    savedLineEnding: LineEnding;
    mixedLineEndings: boolean;
    languageId: string;
    highlight: boolean;
    modTime: number;
    size: number;
    loading: string;
  }

  const ENCODINGS: Array<{ id: string; label: string }> = [
    { id: 'utf-8', label: 'UTF-8' },
    { id: 'utf-8-bom', label: 'UTF-8 with BOM' },
    { id: 'windows-1252', label: 'Windows-1252 (Western)' },
    { id: 'iso-8859-1', label: 'ISO-8859-1 (Latin-1)' },
    { id: 'iso-8859-15', label: 'ISO-8859-15 (Latin-9)' },
  ];

  let tabs = $state.raw<Tab[]>([]);
  let activeId = $state<number | null>(null);
  let nextId = 1;
  let untitledCount = 0;
  const views = new Map<number, EditorView>();
  const dirtyTimers = new Map<number, ReturnType<typeof setTimeout>>();

  let ready = $state(false);
  let zoomPt = $state(0);
  let showToolbar = $state(true);
  let statusMessage = $state('');

  let cursor = $state({ line: 1, column: 1, selected: 0, overwrite: false, block: false });
  /** Bumped when bookmarks change, so the Go → Bookmarks menu lists them. */
  let bookmarkVersion = $state(0);
  let quickOpen = $state(false);
  let alignPrompt = $state(false);

  // ---- Location histories (Go menu) ------------------------------------------------------------

  /** Lines a selection change must move to count as a jump (Go to Previous Location). */
  const JUMP_LINES = 10;
  const sameLine = (a: EditorLocation, b: EditorLocation) => {
    if (a.tab !== b.tab) return false;
    const doc = views.get(a.tab)?.state.doc;
    if (!doc) return a.pos === b.pos;
    const clamp = (p: number) => Math.max(0, Math.min(doc.length, p));
    return doc.lineAt(clamp(a.pos)).number === doc.lineAt(clamp(b.pos)).number;
  };
  const jumpHistory = new LocationHistory(sameLine);
  const editHistory = new LocationHistory(sameLine);
  /** Set while the Go menu moves the cursor, so the move is not recorded as a new jump. */
  let navigating = false;

  function currentLocation(): EditorLocation | undefined {
    const view = activeView();
    return view && activeId !== null ? { tab: activeId, pos: view.state.selection.main.head } : undefined;
  }

  function goToLocation(loc: EditorLocation | undefined) {
    if (!loc || !getTab(loc.tab)) return;
    navigating = true;
    try {
      activate(loc.tab);
      const view = views.get(loc.tab);
      if (view) {
        const pos = Math.max(0, Math.min(view.state.doc.length, loc.pos));
        view.dispatch({
          selection: { anchor: pos },
          effects: CMView.scrollIntoView(pos, { y: 'center' }),
          userEvent: 'select.navigate',
        });
        view.focus();
      }
    } finally {
      navigating = false;
    }
  }

  /** Records jumps and edits of one update for the Go menu histories. */
  function recordLocations(id: number, u: ViewUpdate) {
    if (u.docChanged) {
      jumpHistory.map(id, u.changes);
      editHistory.map(id, u.changes);
      if (u.transactions.some((tr) => tr.isUserEvent('input') || tr.isUserEvent('delete') || tr.isUserEvent('move'))) {
        editHistory.edit({ tab: id, pos: u.state.selection.main.head });
      }
    }
    if (!u.selectionSet || navigating || u.docChanged) return;
    // Jumps: go to line, search, bookmarks, clicks far away; not cursor keys.
    const tr = u.transactions[u.transactions.length - 1];
    const event = tr?.annotation(Transaction.userEvent);
    const isJump = event === undefined || tr.isUserEvent('select.pointer') || tr.isUserEvent('select.search');
    if (!isJump || tr.isUserEvent('select.navigate')) return;
    const from = u.startState.selection.main.head;
    const to = u.state.selection.main.head;
    const distance = Math.abs(u.startState.doc.lineAt(from).number - u.state.doc.lineAt(to).number);
    if (distance >= JUMP_LINES) jumpHistory.jump({ tab: id, pos: from }, { tab: id, pos: to });
  }

  let activeTab = $derived(tabs.find((t) => t.id === activeId) ?? null);
  let anyDirty = $derived(tabs.some((t) => t.dirty));

  // ---- Settings, theme and font --------------------------------------------------------------

  let editorCfg = $derived(($settings as any)?.editor ?? {});
  let dark = $derived(($settings as any)?.general?.ColorScheme !== 'light');
  let prefs = $derived<ViewPrefs>({
    tabWidth: editorCfg.TabWidth > 0 ? editorCfg.TabWidth : 4,
    indentWithTabs: !!editorCfg.IndentWithTabs,
    wordWrap: !!editorCfg.WordWrap,
    showWhitespace: !!editorCfg.ShowWhitespace,
    showMinimap: editorCfg.ShowMinimap ?? true,
    dark,
  });
  let basePt = $derived(editorCfg.Font?.Size > 0 ? editorCfg.Font.Size : 11);
  let fontFamily = $derived(fontFamilyCss(editorCfg.Font?.Family));
  let fontFace = $derived.by(() => {
    const p = parseStyle(editorCfg.Font?.Style);
    return (
      `--editor-font-weight: ${p.weight}; --editor-font-style: ${p.italic ? 'italic' : 'normal'}; ` +
      `--editor-font-stretch: ${p.stretch || 'normal'}; --editor-font-features: ${fontFeaturesCss(editorCfg.Font?.Features)};`
    );
  });
  let fontPx = $derived(ptToPx(Math.max(6, basePt + zoomPt)));

  $effect(() => {
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  });

  // Word wrap / whitespace / minimap can be toggled per window from the View menu.
  let wrapOverride = $state<boolean | null>(null);
  let whitespaceOverride = $state<boolean | null>(null);
  let minimapOverride = $state<boolean | null>(null);
  let effectivePrefs = $derived<ViewPrefs>({
    ...prefs,
    wordWrap: wrapOverride ?? prefs.wordWrap,
    showWhitespace: whitespaceOverride ?? prefs.showWhitespace,
    showMinimap: minimapOverride ?? prefs.showMinimap,
  });

  $effect(() => {
    const p = effectivePrefs;
    untrack(() => {
      for (const view of views.values()) view.dispatch({ effects: prefsEffects(p) });
    });
  });

  $effect(() => {
    void EditorService.SetUnsaved(anyDirty).catch(() => {});
  });

  $effect(() => {
    const t = activeTab;
    const title = t ? `${t.title}${t.dirty ? ' *' : ''} — LeanTerm Editor` : 'LeanTerm Editor';
    document.title = title;
    void WailsWindow.SetTitle(title).catch(() => {});
  });

  // ---- Dialogs ----------------------------------------------------------------------------------------

  interface Question {
    title: string;
    message: string;
    detail?: string;
    type?: 'question' | 'warning' | 'error' | 'info';
    choices: Choice[];
    cancelId?: string;
    resolve: (id: string) => void;
  }
  let question = $state<Question | null>(null);

  function ask(q: Omit<Question, 'resolve'>): Promise<string> {
    return new Promise((resolve) => {
      question = {
        ...q,
        resolve: (id) => {
          question = null;
          resolve(id);
        },
      };
    });
  }

  const CANCEL: Choice = { id: 'cancel', label: 'Cancel', icon: 'gtk-close' };

  function showError(message: string, err?: unknown): Promise<string> {
    const detail = err === undefined ? '' : String((err as any)?.message ?? err);
    return ask({
      title: 'LeanTerm Editor',
      type: 'error',
      message,
      detail,
      choices: [{ id: 'ok', label: 'OK', isDefault: true }],
      cancelId: 'ok',
    });
  }

  // ---- Tabs -------------------------------------------------------------------------------------------

  function patchTab(id: number, patch: Partial<Tab>) {
    tabs = tabs.map((t) => (t.id === id ? { ...t, ...patch } : t));
  }

  function getTab(id: number): Tab | undefined {
    return tabs.find((t) => t.id === id);
  }

  function activate(id: number) {
    const before = currentLocation();
    activeId = id;
    const view = views.get(id);
    if (view) updateCursor(view);
    // A tab switch is a jump for Go to Previous / Next Location.
    if (!navigating && before && before.tab !== id && view) {
      jumpHistory.jump(before, { tab: id, pos: view.state.selection.main.head });
    }
  }

  function updateCursor(view: EditorView) {
    const sel = view.state.selection.main;
    const line = view.state.doc.lineAt(sel.head);
    let selected = 0;
    for (const r of view.state.selection.ranges) selected += r.to - r.from;
    cursor = {
      line: line.number,
      column: sel.head - line.from + 1,
      selected,
      overwrite: view.state.field(overwriteField, false) ?? false,
      block: view.state.field(blockModeField, false) ?? false,
    };
  }

  function handleUpdate(id: number, u: ViewUpdate) {
    if (u.docChanged) scheduleDirtyCheck(id);
    recordLocations(id, u);
    const effects = u.transactions.some((tr) => tr.effects.length > 0);
    if (id === activeId && (u.docChanged || u.selectionSet || effects)) updateCursor(u.view);
    if (id === activeId && effects) bookmarkVersion++;
  }

  function computeDirty(t: Tab): boolean {
    const view = views.get(t.id);
    if (!view || !t.savedDoc) return false;
    return t.encoding !== t.savedEncoding || t.lineEnding !== t.savedLineEnding || !view.state.doc.eq(t.savedDoc);
  }

  function scheduleDirtyCheck(id: number) {
    clearTimeout(dirtyTimers.get(id));
    dirtyTimers.set(
      id,
      setTimeout(() => {
        dirtyTimers.delete(id);
        const t = getTab(id);
        if (!t) return;
        const dirty = computeDirty(t);
        if (dirty !== t.dirty) patchTab(id, { dirty });
        if (!dirty) views.get(id)?.dispatch({ effects: clearModifiedLines.of() });
      }, 150)
    );
  }

  /** Creates a document state, with the language already applied when highlighting is on. */
  async function buildState(id: number, text: string, languageId: string, highlight: boolean): Promise<EditorState> {
    let state = EditorState.create({
      doc: text,
      extensions: editorExtensions(effectivePrefs, (u) => handleUpdate(id, u), highlight),
    });
    if (highlight && languageId) {
      const ext = await loadLanguage(languageId);
      if (ext) state = state.update({ effects: languageConf.reconfigure(ext) }).state;
    }
    return state;
  }

  async function loadLanguage(languageId: string): Promise<Extension | null> {
    const lang = languageById(languageId);
    if (!lang) return null;
    try {
      return await lang.load();
    } catch (err) {
      console.warn(`Loading ${lang.name} support failed:`, err);
      return null;
    }
  }

  async function applyLanguage(id: number) {
    const t = getTab(id);
    const view = views.get(id);
    if (!t || !view) return;
    const ext = t.highlight && t.languageId ? await loadLanguage(t.languageId) : null;
    view.dispatch({ effects: [languageConf.reconfigure(ext ?? []), highlightEffect(t.highlight)] });
  }

  function newTab(file: EditorFile | null, title: string): Tab {
    return {
      id: nextId++,
      file,
      title,
      generation: 0,
      initialState: null,
      savedDoc: null,
      dirty: false,
      encoding: 'utf-8',
      savedEncoding: 'utf-8',
      lineEnding: 'lf',
      savedLineEnding: 'lf',
      mixedLineEndings: false,
      languageId: '',
      highlight: true,
      modTime: 0,
      size: 0,
      loading: '',
    };
  }

  async function newDocument() {
    untitledCount++;
    const t = newTab(null, `Untitled ${untitledCount}`);
    tabs = [...tabs, t];
    activate(t.id);
    const state = await buildState(t.id, '', '', true);
    patchTab(t.id, { initialState: state, savedDoc: state.doc });
  }

  // ---- Opening ----------------------------------------------------------------------------------------

  async function openFile(file: EditorFile) {
    const key = fileKey(file);
    const existing = tabs.find((t) => t.file && fileKey(t.file) === key);
    if (existing) {
      activate(existing.id);
      return;
    }

    let info;
    try {
      info = await statFile(file);
    } catch (err) {
      await showError(`${displayPath(file)} cannot be opened.`, err);
      return;
    }
    if (!info.exists) {
      await showError(`${displayPath(file)} does not exist.`);
      return;
    }
    if (info.isDir) {
      await showError(`${displayPath(file)} is a folder.`);
      return;
    }

    const largeMB = editorCfg.LargeFileMB > 0 ? editorCfg.LargeFileMB : 10;
    const hugeMB = editorCfg.HugeFileMB >= largeMB ? editorCfg.HugeFileMB : 100;
    const sizeClass = classifySize(info.size, largeMB, hugeMB);
    const name = baseName(file.path);
    if (sizeClass === 'large') {
      const a = await ask({
        title: 'Large file',
        type: 'warning',
        message: `${name} is large (${formatSize(info.size)}).`,
        detail: 'Opening it may take a moment and use a lot of memory.',
        choices: [{ id: 'open', label: 'Open', icon: 'gtk-open', isDefault: true }, CANCEL],
      });
      if (a !== 'open') return;
    } else if (sizeClass === 'huge') {
      const a = await ask({
        title: 'Very large file',
        type: 'warning',
        message: `${name} is very large (${formatSize(info.size)}).`,
        detail: 'Opening it needs a lot of memory and can take a long time; LeanTerm may become slow.',
        choices: [{ id: 'continue', label: 'Continue', isDefault: false }, { ...CANCEL, isDefault: true }],
      });
      if (a !== 'continue') return;
      const b = await ask({
        title: 'Very large file',
        type: 'warning',
        message: `Really open ${name} (${formatSize(info.size)})?`,
        detail: 'Syntax highlighting stays off for this file; you can turn it on in View → Syntax Highlighting.',
        choices: [{ id: 'open', label: 'Open anyway', icon: 'gtk-open' }, { ...CANCEL, isDefault: true }],
      });
      if (b !== 'open') return;
    }

    const t = newTab(file, name);
    t.highlight = sizeClass !== 'huge';
    t.loading = 'Loading…';
    tabs = [...tabs, t];
    activate(t.id);
    await loadInto(t.id, '', false);
  }

  /** (Re)loads the file of a tab, replacing its document. */
  async function loadInto(id: number, encoding: string, allowBinary: boolean): Promise<boolean> {
    const t = getTab(id);
    if (!t?.file) return false;
    const file = t.file;
    patchTab(id, { loading: 'Loading…' });
    let loaded;
    try {
      loaded = await loadFile(file, encoding, allowBinary, (chars) => {
        if (chars > 1 << 20) patchTab(id, { loading: `Loading… ${formatSize(chars)}` });
      });
    } catch (err) {
      patchTab(id, { loading: '' });
      await showError(`${displayPath(file)} could not be read.`, err);
      if (!getTab(id)?.initialState) closeTabNow(id);
      return false;
    }
    if (loaded === null) {
      const a = await ask({
        title: 'Binary file',
        type: 'warning',
        message: `${baseName(file.path)} looks like a binary file.`,
        detail: 'Editing and saving it as text can damage it.',
        choices: [{ id: 'open', label: 'Open anyway' }, { ...CANCEL, isDefault: true }],
      });
      if (a === 'open') return loadInto(id, encoding, true);
      patchTab(id, { loading: '' });
      if (!getTab(id)?.initialState) closeTabNow(id);
      return false;
    }

    const current = getTab(id);
    if (!current) return false;
    const firstLine = loaded.text.slice(0, 200).split('\n', 1)[0];
    const languageId = current.languageId || detectLanguage(file.path, firstLine)?.id || '';
    const state = await buildState(id, loaded.text, languageId, current.highlight);
    patchTab(id, {
      initialState: state,
      generation: current.generation + 1,
      savedDoc: state.doc,
      dirty: false,
      encoding: loaded.encoding,
      savedEncoding: loaded.encoding,
      lineEnding: loaded.lineEnding,
      savedLineEnding: loaded.lineEnding,
      mixedLineEndings: loaded.mixedLineEndings,
      languageId,
      modTime: loaded.modTime,
      size: loaded.size,
      loading: '',
    });
    statusMessage = loaded.mixedLineEndings
      ? `${baseName(file.path)} has mixed line endings; saving converts them to ${loaded.lineEnding === 'crlf' ? 'Windows (CRLF)' : 'Unix (LF)'}.`
      : '';
    return true;
  }

  async function chooseAndOpen() {
    const start = activeTab?.file?.kind === 'local' ? activeTab.file.path.replace(/\/[^/]*$/, '') : '';
    try {
      const path = await EditorService.ChooseOpenFile(start);
      if (path) await openFile({ kind: 'local', path });
    } catch (err) {
      await showError('The file could not be opened.', err);
    }
  }

  async function takePending() {
    try {
      const files = (await EditorService.TakePending()) as EditorFile[];
      for (const f of files ?? []) await openFile(f);
    } catch (err) {
      console.warn('Taking pending files failed:', err);
    }
  }

  // ---- Saving -------------------------------------------------------------------------------------------

  /** Saves a tab; returns false when it was not saved (cancelled or failed). */
  async function saveTab(id: number, force = false): Promise<boolean> {
    const t = getTab(id);
    const view = views.get(id);
    if (!t || !view) return false;
    if (!t.file) return saveTabAs(id);

    const doc = view.state.doc;
    let res;
    try {
      statusMessage = `Saving ${t.title}…`;
      res = await saveFile(t.file, doc, {
        encoding: t.encoding,
        lineEnding: t.lineEnding,
        expectedModTime: t.modTime,
        expectedSize: t.size,
        force,
      });
    } catch (err) {
      statusMessage = '';
      const msg = String((err as any)?.message ?? err);
      if (t.file.kind === 'sftp' && /connection of this file is closed/i.test(msg)) {
        const a = await ask({
          title: 'Connection closed',
          type: 'warning',
          message: `The connection to ${t.file.host || 'the server'} is closed.`,
          detail: 'Connect to it again in LeanTerm and save again, or save a local copy now.',
          choices: [{ id: 'local', label: 'Save Local Copy…', icon: 'gtk-save-as' }, { ...CANCEL, isDefault: true }],
        });
        return a === 'local' ? saveTabAs(id) : false;
      }
      await showError(`${t.title} could not be saved.`, err);
      return false;
    }
    statusMessage = '';

    if (res.conflict) {
      const a = await ask({
        title: 'File changed on disk',
        type: 'warning',
        message: `${t.title} was changed by another program since it was opened.`,
        detail: 'Overwrite it with your version, or reload it and lose your changes?',
        choices: [
          { id: 'overwrite', label: 'Overwrite', icon: 'gtk-save' },
          { id: 'reload', label: 'Reload', icon: 'gtk-refresh' },
          { ...CANCEL, isDefault: true },
        ],
      });
      if (a === 'overwrite') return saveTab(id, true);
      if (a === 'reload') await loadInto(id, '', true);
      return false;
    }

    const now = getTab(id);
    if (!now) return true;
    patchTab(id, {
      savedDoc: doc,
      savedEncoding: now.encoding,
      savedLineEnding: now.lineEnding,
      mixedLineEndings: false,
      modTime: res.modTime,
      size: res.size,
    });
    const after = getTab(id);
    if (after) patchTab(id, { dirty: computeDirty(after) });
    views.get(id)?.dispatch({ effects: clearModifiedLines.of() });
    statusMessage = `Saved ${displayPath(t.file)}`;
    return true;
  }

  async function saveTabAs(id: number): Promise<boolean> {
    const t = getTab(id);
    const view = views.get(id);
    if (!t || !view) return false;
    const suggested = t.file?.kind === 'local' ? t.file.path : t.file ? baseName(t.file.path) : `${t.title}.txt`;
    let path = '';
    try {
      path = await EditorService.ChooseSaveFile(suggested);
    } catch (err) {
      await showError('The file could not be saved.', err);
      return false;
    }
    if (!path) return false;
    const file: EditorFile = { kind: 'local', path };
    const doc = view.state.doc;
    try {
      // The native dialog has already asked before replacing an existing file.
      const res = await saveFile(file, doc, {
        encoding: t.encoding,
        lineEnding: t.lineEnding,
        expectedModTime: 0,
        expectedSize: 0,
        force: true,
      });
      const firstLine = doc.lineAt(0).text;
      const languageId = detectLanguage(path, firstLine)?.id ?? t.languageId;
      patchTab(id, {
        file,
        title: baseName(path),
        savedDoc: doc,
        savedEncoding: t.encoding,
        savedLineEnding: t.lineEnding,
        mixedLineEndings: false,
        modTime: res.modTime,
        size: res.size,
        languageId,
      });
      const after = getTab(id);
      if (after) patchTab(id, { dirty: computeDirty(after) });
      await applyLanguage(id);
      statusMessage = `Saved ${path}`;
      return true;
    } catch (err) {
      await showError(`${path} could not be saved.`, err);
      return false;
    }
  }

  async function saveAll(): Promise<boolean> {
    for (const t of tabs.filter((x) => x.dirty)) {
      activate(t.id);
      if (!(await saveTab(t.id))) return false;
    }
    return true;
  }

  async function reloadTab(id: number, encoding = '') {
    const t = getTab(id);
    if (!t?.file) return;
    if (t.dirty) {
      const a = await ask({
        title: 'Reload',
        type: 'warning',
        message: `Discard the changes to ${t.title} and reload it?`,
        choices: [{ id: 'reload', label: 'Reload', icon: 'gtk-refresh' }, { ...CANCEL, isDefault: true }],
      });
      if (a !== 'reload') return;
    }
    await loadInto(id, encoding, true);
  }

  // ---- Closing ------------------------------------------------------------------------------------------

  function closeTabNow(id: number) {
    const idx = tabs.findIndex((t) => t.id === id);
    if (idx < 0) return;
    clearTimeout(dirtyTimers.get(id));
    dirtyTimers.delete(id);
    jumpHistory.removeTab(id);
    editHistory.removeTab(id);
    const rest = tabs.filter((t) => t.id !== id);
    tabs = rest;
    if (activeId === id) {
      const next = rest[Math.min(idx, rest.length - 1)];
      activeId = next ? next.id : null;
      if (next) activate(next.id);
    }
  }

  async function closeTab(id: number): Promise<boolean> {
    const t = getTab(id);
    if (!t) return true;
    if (t.dirty) {
      activate(id);
      const a = await ask({
        title: 'Close Document',
        type: 'warning',
        message: `Save the changes to ${t.title} before closing?`,
        choices: [
          { id: 'save', label: 'Save', icon: 'gtk-save', isDefault: true },
          { id: 'discard', label: 'Discard', icon: 'gtk-delete' },
          CANCEL,
        ],
      });
      if (a === 'cancel') return false;
      if (a === 'save' && !(await saveTab(id))) return false;
    }
    closeTabNow(id);
    return true;
  }

  async function closeAll(): Promise<boolean> {
    for (const t of [...tabs]) {
      if (!(await closeTab(t.id))) return false;
    }
    return true;
  }

  async function requestCloseWindow() {
    const dirty = tabs.filter((t) => t.dirty);
    if (dirty.length > 0) {
      const a = await ask({
        title: 'Close Editor',
        type: 'warning',
        message:
          dirty.length === 1
            ? `Save the changes to ${dirty[0].title} before closing?`
            : `${dirty.length} documents have unsaved changes. Save them before closing?`,
        detail: dirty.length > 1 ? dirty.map((t) => t.title).join('\n') : '',
        choices: [
          { id: 'save', label: dirty.length > 1 ? 'Save All' : 'Save', icon: 'gtk-save', isDefault: true },
          { id: 'discard', label: 'Discard', icon: 'gtk-delete' },
          CANCEL,
        ],
      });
      if (a === 'cancel') return;
      if (a === 'save' && !(await saveAll())) return;
    }
    void EditorService.SetUnsaved(false).catch(() => {});
    void EditorService.CloseWindow();
  }

  // ---- Editing commands ---------------------------------------------------------------------------------

  function activeView(): EditorView | undefined {
    return activeId === null ? undefined : views.get(activeId);
  }

  function run(cmd: (view: EditorView) => boolean | void) {
    const view = activeView();
    if (!view) return;
    cmd(view);
    view.focus();
  }

  async function copySelection(cut: boolean) {
    const view = activeView();
    if (!view) return;
    const text = view.state.selection.ranges.map((r) => view.state.sliceDoc(r.from, r.to)).join('\n');
    if (!text) return;
    await Clipboard.SetText(text).catch(() => navigator.clipboard.writeText(text).catch(() => {}));
    if (cut) view.dispatch(view.state.replaceSelection(''), { userEvent: 'delete.cut' });
    view.focus();
  }

  async function paste() {
    const view = activeView();
    if (!view) return;
    const text = await readClipboardText();
    if (text) view.dispatch(view.state.replaceSelection(text), { userEvent: 'input.paste', scrollIntoView: true });
    view.focus();
  }

  function setLineEnding(le: LineEnding) {
    const t = activeTab;
    if (!t || t.lineEnding === le) return;
    patchTab(t.id, { lineEnding: le });
    const after = getTab(t.id);
    if (after) patchTab(t.id, { dirty: computeDirty(after) });
  }

  function setEncoding(enc: string) {
    const t = activeTab;
    if (!t || t.encoding === enc) return;
    patchTab(t.id, { encoding: enc });
    const after = getTab(t.id);
    if (after) patchTab(t.id, { dirty: computeDirty(after) });
  }

  async function setLanguage(languageId: string) {
    const t = activeTab;
    if (!t) return;
    patchTab(t.id, { languageId });
    await applyLanguage(t.id);
  }

  async function toggleHighlight() {
    const t = activeTab;
    if (!t) return;
    patchTab(t.id, { highlight: !t.highlight });
    await applyLanguage(t.id);
  }

  function zoom(delta: number | null) {
    zoomPt = delta === null ? 0 : Math.max(6 - basePt, Math.min(40 - basePt, zoomPt + delta));
  }

  // ---- Menus --------------------------------------------------------------------------------------------

  const languageName = (id: string) => languageById(id)?.name ?? 'Plain Text';

  function languageItems(): MenuItem[] {
    const current = activeTab?.languageId ?? '';
    const group = (g: LanguageGroup): MenuItem[] =>
      LANGUAGES.filter((l) => l.group === g)
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((l) => ({ label: l.name, checked: current === l.id, action: () => setLanguage(l.id) }));
    return [
      { label: 'Plain Text', checked: current === '', action: () => setLanguage('') },
      { separator: true },
      { label: 'Programming', submenu: group('code') },
      { label: 'Web', submenu: group('web') },
      { label: 'Configuration & Data', submenu: group('config') },
    ];
  }

  /** Go → Bookmarks: commands plus one entry per bookmark ("12: first words of the line"). */
  function bookmarkItems(): MenuItem[] {
    void bookmarkVersion;
    void cursor.line;
    const view = activeView();
    const lines = view ? bookmarkLines(view.state).sort((a, b) => a - b) : [];
    const items: MenuItem[] = [
      { label: 'Set Bookmark', shortcut: 'Ctrl+B', checked: lines.includes(cursor.line), action: () => run(toggleBookmark) },
      { label: 'Clear All Bookmarks', disabled: lines.length === 0, action: () => run(clearAllBookmarks) },
      { separator: true },
      { label: 'Next Bookmark', shortcut: 'Alt+PgDown', disabled: lines.length === 0, action: () => run(gotoBookmark(1)) },
      { label: 'Previous Bookmark', shortcut: 'Alt+PgUp', disabled: lines.length === 0, action: () => run(gotoBookmark(-1)) },
    ];
    if (view && lines.length > 0) {
      items.push({ separator: true });
      for (const n of lines) {
        const text = view.state.doc.line(n).text.trim();
        items.push({ label: `${n}: ${text.length > 40 ? `${text.slice(0, 40)}…` : text}`, action: () => run((v) => goToLineNumber(v, n)) });
      }
    }
    return items;
  }

  function encodingItems(): MenuItem[] {
    const t = activeTab;
    return ENCODINGS.map((e) => ({ label: e.label, checked: t?.encoding === e.id, action: () => setEncoding(e.id) }));
  }

  function reloadEncodingItems(): MenuItem[] {
    const t = activeTab;
    return ENCODINGS.map((e) => ({ label: e.label, action: () => t && reloadTab(t.id, e.id) }));
  }

  let menus = $derived.by<MenuDef[]>(() => {
    const t = activeTab;
    const hasDoc = !!t?.initialState;
    const hasFile = !!t?.file;
    return [
      {
        label: 'File',
        items: [
          { label: 'New', icon: 'gtk-new', shortcut: 'Ctrl+N', action: newDocument },
          { label: 'Open…', icon: 'gtk-open', shortcut: 'Ctrl+O', action: chooseAndOpen },
          { separator: true },
          { label: 'Save', icon: 'gtk-save', shortcut: 'Ctrl+S', disabled: !hasDoc, action: () => t && saveTab(t.id) },
          { label: 'Save As…', icon: 'gtk-save-as', shortcut: 'Ctrl+Shift+S', disabled: !hasDoc, action: () => t && saveTabAs(t.id) },
          { label: 'Save All', disabled: !anyDirty, action: saveAll },
          { separator: true },
          { label: 'Reload', icon: 'gtk-refresh', shortcut: 'F5', disabled: !hasFile, action: () => t && reloadTab(t.id) },
          { label: 'Reload with Encoding', disabled: !hasFile, submenu: reloadEncodingItems() },
          { separator: true },
          { label: 'Close', icon: 'gtk-close', shortcut: 'Ctrl+W', disabled: !t, action: () => t && closeTab(t.id) },
          { label: 'Close All', disabled: tabs.length === 0, action: closeAll },
          { separator: true },
          { label: 'Quit Editor', icon: 'gtk-quit', shortcut: 'Ctrl+Q', action: requestCloseWindow },
        ],
      },
      {
        label: 'Edit',
        items: [
          { label: 'Undo', icon: 'gtk-undo', shortcut: 'Ctrl+Z', disabled: !hasDoc, action: () => run(undo) },
          { label: 'Redo', icon: 'gtk-redo', shortcut: 'Ctrl+Shift+Z', disabled: !hasDoc, action: () => run(redo) },
          { separator: true },
          { label: 'Cut', icon: 'gtk-cut', shortcut: 'Ctrl+X', disabled: !hasDoc, action: () => copySelection(true) },
          { label: 'Copy', icon: 'gtk-copy', shortcut: 'Ctrl+C', disabled: !hasDoc, action: () => copySelection(false) },
          { label: 'Paste', icon: 'gtk-paste', shortcut: 'Ctrl+V', disabled: !hasDoc, action: paste },
          { separator: true },
          { label: 'Find…', icon: 'gtk-find', shortcut: 'Ctrl+F', disabled: !hasDoc, action: () => run(openSearchPanel) },
          { label: 'Replace…', icon: 'gtk-edit', shortcut: 'Ctrl+R', disabled: !hasDoc, action: () => run(openSearchPanel) },
          { label: 'Find Next', shortcut: 'F3', disabled: !hasDoc, action: () => run(findNext) },
          { label: 'Find Previous', shortcut: 'Shift+F3', disabled: !hasDoc, action: () => run(findPrevious) },
        ],
      },
      {
        label: 'Selection',
        items: [
          { label: 'Select All', shortcut: 'Ctrl+A', disabled: !hasDoc, action: () => run(selectAll) },
          { label: 'Deselect', shortcut: 'Ctrl+Shift+A', disabled: !hasDoc || cursor.selected === 0, action: () => run(deselect) },
          {
            label: 'Block Selection Mode',
            shortcut: 'Ctrl+Shift+B',
            checked: cursor.block,
            disabled: !hasDoc,
            action: () => run((v) => v.dispatch({ effects: toggleBlockMode.of() })),
          },
          { separator: true },
          { label: 'Toggle Comment', shortcut: 'Ctrl+/', disabled: !hasDoc, action: () => run(toggleComment) },
          { label: 'Join Lines', shortcut: 'Ctrl+J', disabled: !hasDoc, action: () => run(joinLines) },
          {
            label: 'Capitalisation',
            disabled: !hasDoc,
            submenu: [
              { label: 'Uppercase', shortcut: 'Ctrl+U', action: () => run(upperCase) },
              { label: 'Lowercase', shortcut: 'Ctrl+Shift+U', action: () => run(lowerCase) },
              { label: 'Capitalize', shortcut: 'Ctrl+Alt+U', action: () => run(capitalize) },
            ],
          },
          { separator: true },
          { label: 'Clean Indentation', disabled: !hasDoc, action: () => run(cleanIndentation) },
          { label: 'Convert Indentation to Spaces', disabled: !hasDoc, action: () => run(convertIndentationToSpaces) },
          { label: 'Convert Indentation to Tabs', disabled: !hasDoc, action: () => run(convertIndentationToTabs) },
          { label: 'Format Indentation', disabled: !hasDoc, action: () => run(indentSelection) },
          { label: 'Align On…', disabled: !hasDoc, action: () => (alignPrompt = true) },
          { label: 'Apply Word Wrap', disabled: !hasDoc, action: () => run(applyWordWrap(80)) },
          { separator: true },
          { label: 'Add Caret above Cursor', shortcut: 'Ctrl+Alt+Up', disabled: !hasDoc, action: () => run(addCursorAbove) },
          { label: 'Add Caret below Cursor', shortcut: 'Ctrl+Alt+Down', disabled: !hasDoc, action: () => run(addCursorBelow) },
          { label: 'Add Cursors to Line Ends', shortcut: 'Alt+Shift+I', disabled: !hasDoc, action: () => run(addCursorsToLineEnds) },
          { label: 'Find and Select Next Occurrence', shortcut: 'Alt+J', disabled: !hasDoc, action: () => run(selectNextOccurrence) },
          {
            label: 'Find and Select All Occurrences',
            shortcut: 'Ctrl+Alt+Shift+J',
            disabled: !hasDoc,
            action: () => run(selectSelectionMatches),
          },
        ],
      },
      {
        label: 'View',
        items: [
          { label: 'Word Wrap', checked: effectivePrefs.wordWrap, action: () => (wrapOverride = !effectivePrefs.wordWrap) },
          {
            label: 'Show Whitespace',
            checked: effectivePrefs.showWhitespace,
            action: () => (whitespaceOverride = !effectivePrefs.showWhitespace),
          },
          {
            label: 'Show Scrollbar Minimap',
            checked: effectivePrefs.showMinimap,
            action: () => (minimapOverride = !effectivePrefs.showMinimap),
          },
          { label: 'Syntax Highlighting', checked: !!t?.highlight, disabled: !hasDoc, action: toggleHighlight },
          { separator: true },
          { label: 'Fold All', disabled: !hasDoc, action: () => run(foldAll) },
          { label: 'Unfold All', disabled: !hasDoc, action: () => run(unfoldAll) },
          { separator: true },
          { label: 'Zoom In', shortcut: 'Ctrl++', action: () => zoom(1) },
          { label: 'Zoom Out', shortcut: 'Ctrl+-', action: () => zoom(-1) },
          { label: 'Reset Zoom', shortcut: 'Ctrl+0', action: () => zoom(null) },
          { separator: true },
          { label: 'Show Toolbar', checked: showToolbar, action: () => (showToolbar = !showToolbar) },
        ],
      },
      {
        label: 'Go',
        items: [
          { label: 'Quick Open', shortcut: 'Ctrl+Alt+O', disabled: tabs.length === 0, action: () => (quickOpen = true) },
          { separator: true },
          { label: 'Go to Line…', shortcut: 'Ctrl+G', disabled: !hasDoc, action: () => run(gotoLine) },
          { separator: true },
          { label: 'Next Tab', shortcut: 'Alt+Right', disabled: tabs.length < 2, action: () => cycleTab(1) },
          { label: 'Previous Tab', shortcut: 'Alt+Left', disabled: tabs.length < 2, action: () => cycleTab(-1) },
          { separator: true },
          { label: 'Go to Previous Location', shortcut: 'Ctrl+1', disabled: !hasDoc, action: () => goToLocation(jumpHistory.back(currentLocation())) },
          { label: 'Go to Next Location', shortcut: 'Ctrl+Shift+1', disabled: !hasDoc, action: () => goToLocation(jumpHistory.forward()) },
          { separator: true },
          {
            label: 'Go to Previous Editing Location',
            shortcut: 'Ctrl+E',
            disabled: !hasDoc,
            action: () => goToLocation(editHistory.back(currentLocation())),
          },
          { label: 'Go to Next Editing Location', shortcut: 'Ctrl+Shift+E', disabled: !hasDoc, action: () => goToLocation(editHistory.forward()) },
          { separator: true },
          { label: 'Go to Previous Modified Line', disabled: !hasDoc, action: () => run(gotoModifiedLine(-1)) },
          { label: 'Go to Next Modified Line', disabled: !hasDoc, action: () => run(gotoModifiedLine(1)) },
          { separator: true },
          { label: 'Go to Matching Bracket', shortcut: 'Ctrl+6', disabled: !hasDoc, action: () => run(cursorMatchingBracket) },
          { label: 'Select to Matching Bracket', shortcut: 'Ctrl+Shift+6', disabled: !hasDoc, action: () => run(selectMatchingBracket) },
          { separator: true },
          { label: 'Bookmarks', disabled: !hasDoc, submenu: bookmarkItems() },
        ],
      },
      {
        label: 'Tools',
        items: [
          {
            label: 'Line Endings',
            disabled: !hasDoc,
            submenu: [
              { label: 'Unix (LF)', checked: t?.lineEnding === 'lf', action: () => setLineEnding('lf') },
              { label: 'Windows (CRLF)', checked: t?.lineEnding === 'crlf', action: () => setLineEnding('crlf') },
            ],
          },
          { label: 'Encoding', disabled: !hasDoc, submenu: encodingItems() },
          { label: 'Highlighting', disabled: !hasDoc, submenu: languageItems() },
          { separator: true },
          { label: 'Indent', shortcut: 'Tab', disabled: !hasDoc, action: () => run(indentMore) },
          { label: 'Unindent', shortcut: 'Shift+Tab', disabled: !hasDoc, action: () => run(indentLess) },
        ],
      },
      {
        label: 'Settings',
        items: [
          { label: 'Configure Editor…', icon: 'gtk-preferences', action: () => EditorService.ShowPreferences() },
        ],
      },
    ];
  });

  // Status bar menus (encoding, language) open at the clicked field.
  let popup = $state<{ items: MenuItem[]; x: number; y: number } | null>(null);

  function openPopup(items: MenuItem[], anchor: DOMRect) {
    popup = { items, x: anchor.left, y: Math.max(4, anchor.top - 4 - Math.min(400, items.length * 26)) };
  }

  // ---- Keyboard -----------------------------------------------------------------------------------------

  function handleKeyDown(e: KeyboardEvent) {
    // Keys the editor already handled (search, go to line, case, ...) are done.
    if (question || e.defaultPrevented) return;
    const t = activeTab;
    const ctrl = e.ctrlKey || e.metaKey;
    const k = e.key.toLowerCase();
    let handled = true;
    if (ctrl && !e.shiftKey && k === 'n') newDocument();
    else if (ctrl && !e.shiftKey && k === 'o') chooseAndOpen();
    else if (ctrl && e.shiftKey && k === 's') t && saveTabAs(t.id);
    else if (ctrl && !e.shiftKey && k === 's') t && saveTab(t.id);
    else if (ctrl && !e.shiftKey && k === 'w') t && closeTab(t.id);
    else if (ctrl && !e.shiftKey && k === 'q') requestCloseWindow();
    else if (ctrl && e.key === 'Tab') cycleTab(e.shiftKey ? -1 : 1);
    else if (ctrl && e.key === 'PageDown') cycleTab(1);
    else if (ctrl && e.key === 'PageUp') cycleTab(-1);
    else if (ctrl && (e.key === '+' || e.key === '=')) zoom(1);
    else if (ctrl && e.key === '-') zoom(-1);
    else if (ctrl && e.key === '0') zoom(null);
    else if (e.key === 'F5' && !ctrl) t && reloadTab(t.id);
    // Keys a browser would use for itself (reload, find) when no editor has focus.
    else if (ctrl && (k === 'r' || k === 'f')) run(openSearchPanel);
    else if (ctrl && k === 'g') run(gotoLine);
    else handled = false;
    if (handled) {
      e.preventDefault();
      e.stopPropagation();
    }
  }

  /**
   * Go menu keys that CodeMirror would otherwise take (Alt+Left/Right move by
   * word there), so they are handled before the editor sees them.
   */
  /** AltGr arrives as Ctrl+Alt on Windows; such keys type characters. */
  const isAltGraph = (e: KeyboardEvent) => e.getModifierState?.('AltGraph') ?? false;

  function handleGoKeys(e: KeyboardEvent) {
    if (question || quickOpen || alignPrompt) return;
    const ctrl = e.ctrlKey || e.metaKey;
    // Ctrl+Shift+1 reports "!" (or another symbol) as key; use the physical key.
    const digit1 = e.code === 'Digit1' || e.key === '1';
    let handled = true;
    if (e.altKey && !ctrl && !e.shiftKey && e.key === 'ArrowRight') cycleTab(1);
    else if (e.altKey && !ctrl && !e.shiftKey && e.key === 'ArrowLeft') cycleTab(-1);
    else if (ctrl && !e.altKey && digit1) goToLocation(e.shiftKey ? jumpHistory.forward() : jumpHistory.back(currentLocation()));
    else if (ctrl && !e.altKey && e.code === 'KeyE') goToLocation(e.shiftKey ? editHistory.forward() : editHistory.back(currentLocation()));
    // By character, not physical key: AltGr is Ctrl+Alt on Windows (AltGr+O types "ó" on Polish).
    else if (ctrl && e.altKey && !e.shiftKey && !isAltGraph(e) && e.key.toLowerCase() === 'o') quickOpen = tabs.length > 0;
    else handled = false;
    if (handled) {
      e.preventDefault();
      e.stopPropagation();
    }
  }

  function cycleTab(dir: number) {
    if (tabs.length < 2 || activeId === null) return;
    const idx = tabs.findIndex((t) => t.id === activeId);
    activate(tabs[(idx + dir + tabs.length) % tabs.length].id);
  }

  // ---- Startup ------------------------------------------------------------------------------------------

  onMount(() => {
    const offs: Array<() => void> = [];
    offs.push(Events.On('editor:pending', () => void takePending()));
    offs.push(Events.On('editor:close-requested', () => void requestCloseWindow()));
    offs.push(Events.On('settings:changed', () => void loadSettings()));

    (async () => {
      const s: any = await loadSettings();
      setIconTheme(s?.general?.Theme);
      ready = true;
      await takePending();
    })();

    return () => offs.forEach((off) => off());
  });

  function tabTooltip(t: Tab): string {
    return t.file ? displayPath(t.file) : t.title;
  }
</script>

<svelte:window onkeydowncapture={handleGoKeys} onkeydown={handleKeyDown} onblur={() => (popup = null)} />

<div
  class="editor-app"
  style={`--editor-font-family: ${fontFamily}; --editor-font-size: ${fontPx}px; ${fontFace}`}
>
  <EditorMenuBar {menus} />

  {#if showToolbar}
    <div class="editor-toolbar">
      <Button icon="gtk-new" tooltip="New (Ctrl+N)" variant="flat" onclick={newDocument} />
      <Button icon="gtk-open" tooltip="Open… (Ctrl+O)" variant="flat" onclick={chooseAndOpen} />
      <Button icon="gtk-save" tooltip="Save (Ctrl+S)" variant="flat" disabled={!activeTab?.initialState} onclick={() => activeTab && saveTab(activeTab.id)} />
      <Button icon="gtk-save-as" tooltip="Save As… (Ctrl+Shift+S)" variant="flat" disabled={!activeTab?.initialState} onclick={() => activeTab && saveTabAs(activeTab.id)} />
      <span class="tb-sep"></span>
      <Button icon="gtk-undo" tooltip="Undo (Ctrl+Z)" variant="flat" disabled={!activeTab?.initialState} onclick={() => run(undo)} />
      <Button icon="gtk-redo" tooltip="Redo (Ctrl+Shift+Z)" variant="flat" disabled={!activeTab?.initialState} onclick={() => run(redo)} />
      <span class="tb-sep"></span>
      <Button icon="gtk-cut" tooltip="Cut (Ctrl+X)" variant="flat" disabled={!activeTab?.initialState} onclick={() => copySelection(true)} />
      <Button icon="gtk-copy" tooltip="Copy (Ctrl+C)" variant="flat" disabled={!activeTab?.initialState} onclick={() => copySelection(false)} />
      <Button icon="gtk-paste" tooltip="Paste (Ctrl+V)" variant="flat" disabled={!activeTab?.initialState} onclick={paste} />
      <span class="tb-sep"></span>
      <Button icon="gtk-find" tooltip="Find / Replace (Ctrl+F)" variant="flat" disabled={!activeTab?.initialState} onclick={() => run(openSearchPanel)} />
    </div>
  {/if}

  {#if tabs.length > 0}
    <div class="editor-tabs" role="tablist">
      {#each tabs as t (t.id)}
        <div
          class="editor-tab"
          class:active={t.id === activeId}
          role="tab"
          tabindex="-1"
          aria-selected={t.id === activeId}
          title={tabTooltip(t)}
          onclick={() => activate(t.id)}
          onkeydown={(e) => e.key === 'Enter' && activate(t.id)}
          onauxclick={(e) => {
            if (e.button === 1) {
              e.preventDefault();
              closeTab(t.id);
            }
          }}
        >
          {#if t.file?.kind === 'sftp'}<span class="tab-remote" title={t.file.host}>⇄</span>{/if}
          <span class="tab-title">{t.title}</span>
          {#if t.dirty}<span class="tab-dirty" title="Unsaved changes">●</span>{/if}
          <button
            type="button"
            class="tab-close"
            title="Close (Ctrl+W)"
            onclick={(e) => {
              e.stopPropagation();
              closeTab(t.id);
            }}>×</button
          >
        </div>
      {/each}
    </div>
  {/if}

  <div class="editor-body">
    {#each tabs as t (`${t.id}:${t.generation}`)}
      {#if t.initialState}
        <EditorPane
          editorState={t.initialState}
          active={t.id === activeId}
          onready={(view) => {
            views.set(t.id, view);
            if (t.id === activeId) updateCursor(view);
          }}
          ondestroy={(view) => {
            // A reload mounts the new view before the old one goes away.
            if (views.get(t.id) === view) views.delete(t.id);
          }}
        />
      {/if}
    {/each}

    {#if activeTab?.loading}
      <div class="editor-placeholder">{activeTab.loading}</div>
    {:else if tabs.length === 0 && ready}
      <div class="editor-placeholder">
        <p>No document open.</p>
        <div class="placeholder-actions">
          <Button label="New" icon="gtk-new" onclick={newDocument} />
          <Button label="Open…" icon="gtk-open" onclick={chooseAndOpen} />
        </div>
      </div>
    {/if}
  </div>

  {#if activeTab?.initialState}
    <EditorStatusBar
      line={cursor.line}
      column={cursor.column}
      selected={cursor.selected}
      overwrite={cursor.overwrite}
      block={cursor.block}
      indent={effectivePrefs.indentWithTabs ? `Tabs: ${effectivePrefs.tabWidth}` : `Soft Tabs: ${effectivePrefs.tabWidth}`}
      encoding={activeTab.encoding}
      lineEnding={activeTab.lineEnding}
      mixedLineEndings={activeTab.mixedLineEndings}
      language={activeTab.highlight ? languageName(activeTab.languageId) : `${languageName(activeTab.languageId)} (off)`}
      source={activeTab.file?.kind === 'sftp' ? activeTab.file.host || 'remote' : ''}
      message={statusMessage}
      ontoggleLineEnding={() => setLineEnding(activeTab?.lineEnding === 'crlf' ? 'lf' : 'crlf')}
      ontoggleOverwrite={() => run((v) => v.dispatch({ effects: toggleOverwrite.of() }))}
      ontoggleBlock={() => run((v) => v.dispatch({ effects: toggleBlockMode.of() }))}
      onencoding={(r) => openPopup(encodingItems(), r)}
      onlanguage={(r) => openPopup(languageItems(), r)}
    />
  {:else}
    <div class="editor-statusbar-empty">{statusMessage}</div>
  {/if}
</div>

{#if popup}
  <ContextMenu items={popup.items} x={popup.x} y={popup.y} onclose={() => (popup = null)} />
{/if}

{#if quickOpen}
  <QuickOpen
    items={tabs.map((t) => ({ id: t.id, title: `${t.title}${t.dirty ? ' *' : ''}`, detail: t.file ? displayPath(t.file) : '' }))}
    onchoose={(id) => {
      quickOpen = false;
      activate(id);
      views.get(id)?.focus();
    }}
    onclose={() => {
      quickOpen = false;
      activeView()?.focus();
    }}
  />
{/if}

<EnterValueDialog
  open={alignPrompt}
  title="Align On"
  placeholder="e.g. =  or  :\s*(\S)"
  prompt="Align lines on (regular expression; the first group, if any, marks the column):"
  onsubmit={(pattern) => {
    alignPrompt = false;
    if (pattern) run(alignOn(pattern));
  }}
  oncancel={() => {
    alignPrompt = false;
    activeView()?.focus();
  }}
/>

{#if question}
  <ChoiceDialog
    title={question.title}
    message={question.message}
    detail={question.detail}
    type={question.type}
    choices={question.choices}
    cancelId={question.cancelId}
    onanswer={question.resolve}
  />
{/if}

<style>
  :global(html),
  :global(body) {
    margin: 0;
    height: 100%;
    overflow: hidden;
  }
  .editor-app {
    display: flex;
    flex-direction: column;
    width: 100vw;
    height: 100vh;
    background: var(--gtk-bg-color);
    color: var(--gtk-fg-color);
    font-family: var(--gtk-font-family);
    font-size: var(--gtk-font-size);
  }
  .editor-toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 3px 6px;
    background: var(--gtk-bg-color);
    border-bottom: 1px solid var(--gtk-border-color);
    flex-shrink: 0;
  }
  .tb-sep {
    width: 1px;
    height: 18px;
    background: var(--gtk-border-color);
    margin: 0 4px;
  }
  .editor-tabs {
    display: flex;
    overflow-x: auto;
    background: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color);
    flex-shrink: 0;
    scrollbar-width: thin;
  }
  .editor-tab {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 4px 6px 4px 10px;
    border-right: 1px solid var(--gtk-border-color);
    cursor: default;
    white-space: nowrap;
    color: var(--gtk-muted-fg);
    max-width: 240px;
    user-select: none;
  }
  .editor-tab.active {
    background: var(--gtk-base-color);
    color: var(--gtk-fg-color);
    box-shadow: inset 0 -2px 0 var(--gtk-accent);
  }
  .editor-tab:hover:not(.active) {
    background: var(--gtk-row-hover);
  }
  .tab-title {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tab-remote {
    color: var(--gtk-link-fg);
    font-size: 0.9em;
  }
  .tab-dirty {
    color: var(--gtk-warning-fg);
    font-size: 0.8em;
  }
  .tab-close {
    background: transparent;
    border: none;
    color: inherit;
    font-size: 1.1em;
    line-height: 1;
    padding: 0 3px;
    border-radius: 3px;
    opacity: 0.7;
  }
  .tab-close:hover {
    opacity: 1;
    background: var(--gtk-row-hover);
  }
  .editor-body {
    position: relative;
    flex: 1;
    min-height: 0;
    background: var(--gtk-base-color);
  }
  .editor-placeholder {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    color: var(--gtk-muted-fg);
    background: var(--gtk-base-color);
  }
  .placeholder-actions {
    display: flex;
    gap: 8px;
  }
  .editor-statusbar-empty {
    min-height: 24px;
    background: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
    flex-shrink: 0;
    padding: 0 8px;
    display: flex;
    align-items: center;
    color: var(--gtk-muted-fg);
    font-size: 0.9em;
  }
</style>
