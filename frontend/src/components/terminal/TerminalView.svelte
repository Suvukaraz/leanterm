<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { SearchAddon } from '@xterm/addon-search';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import { Unicode11Addon } from '@xterm/addon-unicode11';
  import '@xterm/xterm/css/xterm.css';

  import { TerminalStream } from '../../lib/bridge/terminalStream';
  import { settings } from '../../lib/stores/config';
  import { markNewData } from '../../lib/stores/sessions';
  import {
    buildTerminalOptions,
    buildTheme,
    mergeLook,
    ptToPx,
    fontFamilyCss,
    DEFAULT_FONT_PT,
    type TerminalLook,
  } from '../../lib/terminal/xtermOptions';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import FindInTerminal from './FindInTerminal.svelte';
  import TerminalContextMenu from './TerminalContextMenu.svelte';

  interface Props {
    tmpID: string;
    kind?: string;
    hasSFTP?: boolean;
    sftpOpen?: boolean;
    /** Whether this terminal's tab is the visible one. */
    active?: boolean;
    /** Personal look of the connection (used when "use personal settings" is on). */
    personalLook?: TerminalLook | null;
    ontoggleSftp?: () => void;
    ondisconnect?: () => void;
    onclose?: () => void;
  }

  let {
    tmpID,
    kind = 'local',
    hasSFTP = false,
    sftpOpen = false,
    active = true,
    personalLook = null,
    ontoggleSftp,
    ondisconnect,
    onclose,
  }: Props = $props();

  let terminalEl: HTMLElement;
  let term = $state.raw<Terminal | undefined>(undefined);
  let fitAddon: FitAddon;
  let searchAddon = $state.raw<SearchAddon | undefined>(undefined);
  let stream: TerminalStream;
  let resizeObserver: ResizeObserver;
  let fitFrame = 0;

  let menuOpen = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let findOpen = $state(false);

  // Zoom offset in points relative to the configured size.
  let zoomPt = 0;

  let look = $derived(mergeLook($settings?.terminal?.Look as TerminalLook | undefined, personalLook));
  let basePt = $derived(look.Font?.Size && look.Font.Size > 0 ? look.Font.Size : DEFAULT_FONT_PT);

  function fitNow() {
    if (!term || !fitAddon || !terminalEl || terminalEl.offsetParent === null) return;
    const dims = fitAddon.proposeDimensions();
    if (!dims || !dims.cols || !dims.rows) return;
    if (dims.cols !== term.cols || dims.rows !== term.rows) {
      term.resize(dims.cols, dims.rows);
    }
    stream?.resize(term.cols, term.rows);
  }

  function scheduleFit() {
    if (fitFrame) return;
    fitFrame = requestAnimationFrame(() => {
      fitFrame = 0;
      fitNow();
    });
  }

  function setZoom(deltaPt: number | null) {
    if (!term) return;
    zoomPt = deltaPt === null ? 0 : Math.max(6 - basePt, Math.min(40 - basePt, zoomPt + deltaPt));
    term.options.fontSize = ptToPx(basePt + zoomPt);
    scheduleFit();
  }

  async function copySelection() {
    const sel = term?.getSelection();
    if (sel) await navigator.clipboard.writeText(sel).catch(() => {});
  }

  async function pasteClipboard() {
    const text = await navigator.clipboard.readText().catch(() => '');
    if (text) term?.paste(text);
  }

  onMount(() => {
    const t = new Terminal(buildTerminalOptions(untrack(() => look)));
    fitAddon = new FitAddon();
    const search = new SearchAddon();
    t.loadAddon(fitAddon);
    t.loadAddon(search);
    t.loadAddon(new Unicode11Addon());
    t.unicode.activeVersion = '11';
    t.loadAddon(
      new WebLinksAddon((_e, uri) => {
        SystemService.OpenURL(uri).catch(() => window.open(uri, '_blank'));
      })
    );
    t.open(terminalEl);

    // Terminal shortcuts that must not reach the shell. Everything else (incl.
    // Ctrl+C / Ctrl+V without Shift) goes to the remote side.
    t.attachCustomKeyEventHandler((e) => {
      if (e.type !== 'keydown') return true;
      const k = e.key.toLowerCase();
      if (e.ctrlKey && e.shiftKey && !e.altKey && k === 'c') {
        void copySelection();
        return false;
      }
      if (e.ctrlKey && e.shiftKey && !e.altKey && k === 'v') {
        void pasteClipboard();
        return false;
      }
      if (e.shiftKey && !e.ctrlKey && e.key === 'Insert') {
        void pasteClipboard();
        return false;
      }
      if (e.ctrlKey && !e.shiftKey && e.key === 'Insert') {
        void copySelection();
        return false;
      }
      if (e.ctrlKey && !e.altKey && (e.key === '+' || e.key === '=')) {
        setZoom(1);
        return false;
      }
      if (e.ctrlKey && !e.altKey && e.key === '-') {
        setZoom(-1);
        return false;
      }
      if (e.ctrlKey && !e.altKey && e.key === '0') {
        setZoom(null);
        return false;
      }
      if (e.ctrlKey && e.key === 'F3') {
        findOpen = true;
        return false;
      }
      if (e.ctrlKey && e.altKey && k === 's' && hasSFTP) {
        ontoggleSftp?.();
        return false;
      }
      return true;
    });

    stream = new TerminalStream(tmpID, t);
    stream.ondata = () => {
      if (!active) markNewData(tmpID);
    };
    void stream.connect();

    t.onData((data) => stream.write(data));
    t.onBinary((data) => {
      const bytes = new Uint8Array(data.length);
      for (let i = 0; i < data.length; i++) bytes[i] = data.charCodeAt(i) & 0xff;
      stream.write(bytes);
    });

    t.onSelectionChange(() => {
      if (!$settings?.appearance?.SelectionToClipboard) return;
      const selection = t.getSelection();
      if (selection) navigator.clipboard.writeText(selection).catch(() => {});
    });

    resizeObserver = new ResizeObserver(() => scheduleFit());
    resizeObserver.observe(terminalEl);

    term = t;
    searchAddon = search;
    scheduleFit();
    if (active) t.focus();
  });

  // Apply look changes from Preferences live (colours, font, cursor, scrollback).
  $effect(() => {
    const l = look;
    const t = term;
    if (!t) return;
    untrack(() => {
      t.options.theme = buildTheme(l);
      t.options.fontFamily = fontFamilyCss(l.Font?.Family);
      t.options.fontSize = ptToPx(basePt + zoomPt);
      t.options.cursorStyle = l.CursorShape === 'ibeam' ? 'bar' : l.CursorShape === 'underline' ? 'underline' : 'block';
      t.options.drawBoldTextInBrightColors = !!l.BoldIsBright;
      scheduleFit();
    });
  });

  // When the tab becomes visible: refit (size may have changed while hidden) and focus.
  $effect(() => {
    if (active && term) {
      const t = term;
      requestAnimationFrame(() => {
        fitNow();
        t.focus();
      });
    }
  });

  onDestroy(() => {
    cancelAnimationFrame(fitFrame);
    resizeObserver?.disconnect();
    stream?.destroy();
    term?.dispose();
  });

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    menuX = e.clientX;
    menuY = e.clientY;
    menuOpen = true;
  }

  function handleAuxClick(e: MouseEvent) {
    // Middle click pastes (X11 primary selection is approximated by the clipboard).
    if (e.button === 1) {
      e.preventDefault();
      void pasteClipboard();
    }
  }

  function handleWheel(e: WheelEvent) {
    if (e.ctrlKey) {
      e.preventDefault();
      setZoom(e.deltaY < 0 ? 1 : -1);
    }
  }

  function handleRestart() {
    menuOpen = false;
    stream?.write('\r');
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="terminal-wrapper"
  style={`background: ${look.Background || '#000000'};`}
  oncontextmenu={handleContextMenu}
  onauxclick={handleAuxClick}
  onwheel={handleWheel}
>
  <div bind:this={terminalEl} class="xterm-container"></div>

  {#if searchAddon}
    <FindInTerminal bind:open={findOpen} {searchAddon} onclose={() => (findOpen = false)} />
  {/if}

  <TerminalContextMenu
    open={menuOpen}
    x={menuX}
    y={menuY}
    {tmpID}
    {kind}
    {term}
    {hasSFTP}
    {sftpOpen}
    {ontoggleSftp}
    onfind={() => {
      menuOpen = false;
      findOpen = true;
    }}
    onrestart={handleRestart}
    {ondisconnect}
    oncloseTerm={onclose}
    oncloseMenu={() => (menuOpen = false)}
  />
</div>

<style>
  .terminal-wrapper {
    width: 100%;
    height: 100%;
    position: relative;
    overflow: hidden;
    background: var(--gtk-terminal-bg);
  }

  .xterm-container {
    position: absolute;
    inset: 4px 2px 2px 6px;
  }

  :global(.terminal-wrapper .xterm) {
    height: 100%;
  }

  :global(.terminal-wrapper .xterm-viewport) {
    overflow-y: auto !important;
  }
</style>
