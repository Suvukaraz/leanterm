<script lang="ts">
  import Button from '../gtk/Button.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import MessageDialog from '../gtk/MessageDialog.svelte';
  import { settings, tree, findNode } from '../../lib/stores/config';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';

  interface MacroItem {
    kind: 'remote' | 'local';
    command: string;
    description: string;
    confirm: boolean;
    appendReturn: boolean;
  }

  interface Props {
    tmpID: string;
    connID?: string;
    mode?: 'never' | 'combo' | 'buttons';
  }

  let { tmpID, connID = '', mode = 'buttons' }: Props = $props();

  let confirmDialog = $state<{
    open: boolean;
    macro: MacroItem | null;
  }>({
    open: false,
    macro: null,
  });

  let selectedMacroKey = $state<string>('');

  let allMacros = $derived.by<MacroItem[]>(() => {
    const list: MacroItem[] = [];
    const cfg = $settings;

    // 1. Connection macros from node if available
    if (connID && $tree) {
      const node: any = findNode(connID, $tree);
      if (node?.macros?.remote) {
        for (const m of node.macros.remote) {
          if (m.command) {
            list.push({
              kind: 'remote',
              command: m.command,
              description: m.description || m.command,
              confirm: !!m.confirm,
              appendReturn: m.append_return !== false,
            });
          }
        }
      }
      if (node?.macros?.local) {
        for (const m of node.macros.local) {
          if (m.command) {
            list.push({
              kind: 'local',
              command: m.command,
              description: m.description || m.command,
              confirm: !!m.confirm,
              appendReturn: false,
            });
          }
        }
      }
    }

    // 2. Global macros
    if (cfg?.macros?.Remote) {
      for (const m of cfg.macros.Remote) {
        const cmd = m.Command || (m as any).command;
        if (cmd) {
          list.push({
            kind: 'remote',
            command: cmd,
            description: `[G] ${m.Description || (m as any).description || cmd}`,
            confirm: !!(m.Confirm ?? (m as any).confirm),
            appendReturn: (m.AppendReturn ?? (m as any).append_return) !== false,
          });
        }
      }
    }
    if (cfg?.macros?.Local) {
      for (const m of cfg.macros.Local) {
        const cmd = m.Command || (m as any).command;
        if (cmd) {
          list.push({
            kind: 'local',
            command: cmd,
            description: `[G] ${m.Description || (m as any).description || cmd}`,
            confirm: !!(m.Confirm ?? (m as any).confirm),
            appendReturn: false,
          });
        }
      }
    }

    return list;
  });

  let comboOptions = $derived.by(() => {
    return allMacros.map((m, idx) => ({
      value: String(idx),
      label: m.description,
    }));
  });

  function triggerMacro(m: MacroItem) {
    if (m.confirm) {
      confirmDialog = { open: true, macro: m };
      return;
    }
    execute(m);
  }

  async function execute(m: MacroItem) {
    try {
      if (m.kind === 'remote') {
        await SessionService.RunRemoteMacro(tmpID, m.command, m.appendReturn);
      } else {
        await SessionService.RunLocalMacro(tmpID, m.command);
      }
    } catch (err) {
      console.error('Failed to run macro:', err);
    }
  }

  function handleConfirmDialog(choice: string) {
    if (choice === 'yes' && confirmDialog.macro) {
      execute(confirmDialog.macro);
    }
    confirmDialog = { open: false, macro: null };
  }

  function runSelectedCombo() {
    const idx = parseInt(selectedMacroKey, 10);
    if (!isNaN(idx) && allMacros[idx]) {
      triggerMacro(allMacros[idx]);
    }
  }
</script>

{#if mode !== 'never' && allMacros.length > 0}
  <div class="macros-bar">
    <span class="label">Macros:</span>
    {#if mode === 'combo'}
      <div class="combo-row">
        <ComboBoxText
          options={comboOptions}
          bind:value={selectedMacroKey}
        />
        <Button
          label="Execute"
          icon="gtk-execute"
          disabled={!selectedMacroKey}
          onclick={runSelectedCombo}
        />
      </div>
    {:else}
      <div class="buttons-row">
        {#each allMacros as m}
          <Button
            label={m.description}
            icon={m.kind === 'remote' ? 'gtk-media-play' : 'gtk-home'}
            tooltip={m.confirm ? `CONFIRM: ${m.command}` : m.command}
            onclick={() => triggerMacro(m)}
          />
        {/each}
      </div>
    {/if}
  </div>
{/if}

{#if confirmDialog.open && confirmDialog.macro}
  <MessageDialog
    title="Confirm Macro Execution"
    primaryText={`Execute ${confirmDialog.macro.kind} macro?`}
    secondaryText={`Command: ${confirmDialog.macro.command}`}
    buttons="yes_no"
    type="question"
    onresponse={handleConfirmDialog}
  />
{/if}

<style>
  .macros-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 2px 8px;
    background: var(--bg-secondary, #303030);
    border-bottom: 1px solid var(--border-color, #444);
    font-size: 11px;
    overflow-x: auto;
    white-space: nowrap;
  }

  .label {
    font-weight: 600;
    color: var(--text-muted, #aaa);
    user-select: none;
  }

  .combo-row,
  .buttons-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
</style>
