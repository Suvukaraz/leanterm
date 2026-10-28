<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';

  import { TerminalStream } from '../../lib/bridge/terminalStream';
  import { settings } from '../../lib/stores/config';
  import { markNewData } from '../../lib/stores/sessions';
  import { mergeLook, ptToPx, DEFAULT_FONT_PT, type TerminalLook } from '../../lib/terminal/xtermOptions';
  import { createTerminal, normalizeEngine, type TermHandle } from '../../lib/terminal/engine';
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
  let term = $state.raw<TermHandle | undefined>(undefined);
  let stream: TerminalStream | undefined;
  let resizeObserver: ResizeObserver | undefined;
  let fitFrame = 0;
  let destroyed = false;

  let menuOpen = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let findOpen = $state(false);

  // Zoom offset in points relative to the configured size.
  let zoomPt = 0;

  let look = $derived(mergeLook($settings?.terminal?.Look as TerminalLook | undefined, personalLook));
  let basePt = $derived(look.Font?.Size && look.Font.Size > 0 ? look.Font.Size : DEFAULT_FONT_PT);

  function fitNow() {
    if (!term || !terminalEl || terminalEl.offsetParent === null) return;
    const dims = term.fit();
    if (dims) stream?.resize(dims.cols, dims.rows);
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
    term.setFontSize(ptToPx(basePt + zoomPt));
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

  // Terminal shortcuts that must not reach the shell. Everything else (incl.
  // Ctrl+C / Ctrl+V without Shift) goes to the remote side.
  function handleTerminalKey(e: KeyboardEvent): boolean {
    const k = e.key.toLowerCase();
    if (e.ctrlKey && e.shiftKey && !e.altKey && k === 'c') {
      void copySelection();
      return true;
    }
    if (e.ctrlKey && e.shiftKey && !e.altKey && k === 'v') {
      void pasteClipboard();
      return true;
    }
    if (e.shiftKey && !e.ctrlKey && e.key === 'Insert') {
      void pasteClipboard();
      return true;
    }
    if (e.ctrlKey && !e.shiftKey && e.key === 'Insert') {
      void copySelection();
      return true;
    }
    if (e.ctrlKey && !e.altKey && (e.key === '+' || e.key === '=')) {
      setZoom(1);
      return true;
    }
    if (e.ctrlKey && !e.altKey && e.key === '-') {
      setZoom(-1);
      return true;
    }
    if (e.ctrlKey && !e.altKey && e.key === '0') {
      setZoom(null);
      return true;
    }
    if (e.ctrlKey && e.key === 'F3') {
      findOpen = true;
      return true;
    }
    if (e.ctrlKey && e.altKey && k === 's' && hasSFTP) {
      ontoggleSftp?.();
      return true;
    }
    return false;
  }

  onMount(() => {
    // The engine is chosen when the terminal opens; a changed preference applies to new terminals.
    const engine = normalizeEngine(untrack(() => $settings?.terminal?.Engine));
    const s = new TerminalStream(tmpID, {
      write: (data, callback) => {
        if (term) term.write(data, callback);
      },
    });

    createTerminal(engine, terminalEl, untrack(() => look), {
      onKey: handleTerminalKey,
      onWheel: (e) => {
        if (!e.ctrlKey) return false;
        handleWheel(e);
        return true;
      },
      onData: (data) => s.write(data),
      onSelectionChange: () => {
        if (!$settings?.appearance?.SelectionToClipboard) return;
        const selection = term?.getSelection();
        if (selection) navigator.clipboard.writeText(selection).catch(() => {});
      },
      openLink: (uri) => {
        SystemService.OpenURL(uri).catch(() => window.open(uri, '_blank'));
      },
    })
      .then((t) => {
        if (destroyed) {
          t.dispose();
          return;
        }
        term = t;
        stream = s;
        s.ondata = () => {
          if (!active) markNewData(tmpID);
        };
        void s.connect();

        resizeObserver = new ResizeObserver(() => scheduleFit());
        resizeObserver.observe(terminalEl);
        scheduleFit();
        if (active) t.focus();
      })
      .catch((err) => {
        console.error(`Failed to create ${engine} terminal for ${tmpID}:`, err);
      });
  });

  // Apply look changes from Preferences live (colours, font, cursor).
  $effect(() => {
    const l = look;
    const t = term;
    if (!t) return;
    untrack(() => {
      t.setLook(l, ptToPx(basePt + zoomPt));
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
    destroyed = true;
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

  {#if term}
    <FindInTerminal bind:open={findOpen} search={term.search} onclose={() => (findOpen = false)} />
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
