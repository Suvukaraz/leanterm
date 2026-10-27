<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';

  interface Props {
    open: boolean;
    tmpID?: string;
    onclose: () => void;
  }

  let { open = $bindable(false), tmpID = '', onclose }: Props = $props();

  let command = $state('');
  let outputLines = $state<string[]>([]);
  let isRunning = $state(false);
  let logEl: HTMLDivElement;

  let unsubStarted: (() => void) | null = null;
  let unsubOutput: (() => void) | null = null;
  let unsubStopped: (() => void) | null = null;

  onMount(() => {
    try {
      unsubStarted = Events.On('session:pipe:started', (event: any) => {
        const data = event?.data || event;
        if (!tmpID || data.tmpID === tmpID) {
          command = data.command || '';
          isRunning = true;
          outputLines = [`[Started pipe: ${command}]`];
          open = true;
        }
      });

      unsubOutput = Events.On('session:pipe:output', (event: any) => {
        const data = event?.data || event;
        if (!tmpID || data.tmpID === tmpID) {
          if (data.data) {
            outputLines = [...outputLines, data.data];
            scrollBottom();
          }
        }
      });

      unsubStopped = Events.On('session:pipe:stopped', (event: any) => {
        const data = event?.data || event;
        if (!tmpID || data.tmpID === tmpID) {
          isRunning = false;
          outputLines = [...outputLines, '\n[Pipe process finished]'];
          scrollBottom();
        }
      });
    } catch (err) {
      console.warn('Could not register pipe event listeners:', err);
    }
  });

  onDestroy(() => {
    if (unsubStarted) unsubStarted();
    if (unsubOutput) unsubOutput();
    if (unsubStopped) unsubStopped();
  });

  function scrollBottom() {
    setTimeout(() => {
      if (logEl) {
        logEl.scrollTop = logEl.scrollHeight;
      }
    }, 10);
  }

  function copyAll() {
    navigator.clipboard.writeText(outputLines.join(''));
  }

  function clearLog() {
    outputLines = [];
  }

  async function handleStopPipe() {
    if (tmpID) {
      await SessionService.StopPipe(tmpID);
    }
    isRunning = false;
  }
</script>

<Dialog {open} title={command ? `Piped Output: ${command}` : 'Piped Output'} width={640} {onclose}>
  <div class="pipe-window-body">
    <div class="pipe-header">
      <span class="cmd-label">Command: <code>{command || '(none)'}</code></span>
      <span class="status-indicator {isRunning ? 'running' : 'stopped'}">
        {isRunning ? '● RUNNING' : '■ STOPPED'}
      </span>
    </div>

    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div class="pipe-log-area" bind:this={logEl} tabindex="0">
      <pre>{outputLines.join('')}</pre>
    </div>

    <div class="pipe-actions-row">
      <Button label="Copy All" onclick={copyAll} />
      <Button label="Clear" onclick={clearLog} />
      {#if isRunning}
        <Button label="Stop Process" onclick={handleStopPipe} />
      {/if}
    </div>
  </div>

  {#snippet actions()}
    <Button label="Close" onclick={onclose} />
  {/snippet}
</Dialog>

<style>
  .pipe-window-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 12px;
  }

  .pipe-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    background: var(--gtk-header-bg, #f4f4f4);
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 3px;
  }

  .cmd-label code {
    font-family: monospace;
    font-weight: bold;
    color: var(--gtk-selected-bg, #4a90d9);
  }

  .status-indicator {
    font-size: 11px;
    font-weight: bold;
  }

  .status-indicator.running {
    color: var(--gtk-success-fg);
  }

  .status-indicator.stopped {
    color: var(--gtk-muted-fg);
  }

  .pipe-log-area {
    height: 320px;
    overflow-y: auto;
    background: #1e1e1e;
    color: var(--gtk-fg-color);
    padding: 8px 12px;
    border-radius: 4px;
    border: 1px solid var(--gtk-border-color);
    font-family: monospace;
    font-size: 11px;
  }

  .pipe-log-area pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .pipe-actions-row {
    display: flex;
    gap: 8px;
  }
</style>
