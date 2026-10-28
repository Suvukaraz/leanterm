// xterm.js implementation of the terminal handle (the default engine).
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon, type ISearchOptions } from '@xterm/addon-search';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import '@xterm/xterm/css/xterm.css';

import type { SearchOptions, TermHandle, TerminalHooks } from './engine';
import { buildTerminalOptions, buildTheme, fontFamilyCss, type TerminalLook } from './xtermOptions';

const SEARCH_DECORATIONS: ISearchOptions['decorations'] = {
  matchBackground: '#ffff00',
  matchBorder: '#000000',
  matchOverviewRuler: '#ffff00',
  activeMatchBackground: '#ff9900',
  activeMatchColorOverviewRuler: '#ff9900',
};

export function createXtermTerminal(container: HTMLElement, look: TerminalLook, hooks: TerminalHooks): TermHandle {
  const t = new Terminal(buildTerminalOptions(look));
  const fitAddon = new FitAddon();
  const searchAddon = new SearchAddon();
  t.loadAddon(fitAddon);
  t.loadAddon(searchAddon);
  t.loadAddon(new Unicode11Addon());
  t.unicode.activeVersion = '11';
  t.loadAddon(new WebLinksAddon((_e, uri) => hooks.openLink(uri)));
  t.open(container);

  // xterm: returning false stops xterm from processing the key.
  t.attachCustomKeyEventHandler((e) => e.type !== 'keydown' || !hooks.onKey(e));

  t.onData((data) => hooks.onData(data));
  t.onBinary((data) => {
    const bytes = new Uint8Array(data.length);
    for (let i = 0; i < data.length; i++) bytes[i] = data.charCodeAt(i) & 0xff;
    hooks.onData(bytes);
  });
  t.onSelectionChange(() => hooks.onSelectionChange());

  const searchOpts = (o: SearchOptions): ISearchOptions => ({ ...o, decorations: SEARCH_DECORATIONS });

  return {
    engine: 'xterm',
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
      findNext: (text, o) => searchAddon.findNext(text, searchOpts(o)),
      findPrevious: (text, o) => searchAddon.findPrevious(text, searchOpts(o)),
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
      t.options.theme = buildTheme(l);
      t.options.fontFamily = fontFamilyCss(l.Font?.Family);
      t.options.fontSize = fontPx;
      t.options.cursorStyle = l.CursorShape === 'ibeam' ? 'bar' : l.CursorShape === 'underline' ? 'underline' : 'block';
      t.options.drawBoldTextInBrightColors = !!l.BoldIsBright;
    },
    lines(max) {
      const buffer = t.buffer.active;
      const out: string[] = [];
      for (let i = 0; i < Math.min(buffer.length, max); i++) {
        out.push(buffer.getLine(i)?.translateToString(true) || '');
      }
      return out;
    },
    dispose: () => t.dispose(),
  };
}
