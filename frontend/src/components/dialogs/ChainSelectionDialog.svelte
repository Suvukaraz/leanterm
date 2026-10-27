<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import type { Connection, ExpectEntry } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/config/models.js';

  interface Props {
    open: boolean;
    targetTitle: string;
    targetTmpID: string;
    chainConn: Connection | null;
    skipFirstExpect?: boolean;
    onexecute: (selectedIndices: number[]) => void;
    onclose: () => void;
  }

  let {
    open,
    targetTitle,
    targetTmpID,
    chainConn,
    skipFirstExpect = false,
    onexecute,
    onclose,
  }: Props = $props();

  let selectedIndices = $state<number[]>([]);

  $effect(() => {
    if (open && chainConn?.Expect) {
      const initial: number[] = [];
      chainConn.Expect.forEach((_, idx) => {
        if (!skipFirstExpect || idx > 0) {
          initial.push(idx);
        }
      });
      selectedIndices = initial;
    }
  });

  function toggleIndex(idx: number) {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  }

  function selectAll() {
    if (!chainConn?.Expect) return;
    selectedIndices = chainConn.Expect.map((_, idx) => idx);
  }

  function deselectAll() {
    selectedIndices = [];
  }

  function handleExecute() {
    onexecute(selectedIndices);
    onclose();
  }
</script>

<Dialog {open} title="Execute Chain" width={560} {onclose}>
  <div class="chain-dialog-body">
    <div class="chain-info-box">
      <p><b>Target Session:</b> {targetTitle} (<code>{targetTmpID}</code>)</p>
      <p><b>Chained Connection:</b> {chainConn?.Title || chainConn?.Name || 'Unknown'}</p>
      <p class="chain-desc">
        Select the automated Expect commands from <b>{chainConn?.Title}</b> to execute in the current terminal:
      </p>
    </div>

    <div class="chain-toolbar">
      <Button label="Select All" onclick={selectAll} />
      <Button label="Deselect All" onclick={deselectAll} />
    </div>

    <div class="entries-table-wrap">
      {#if chainConn?.Expect && chainConn.Expect.length > 0}
        <table class="entries-table">
          <thead>
            <tr>
              <th style="width: 36px;"></th>
              <th style="width: 40px;">#</th>
              <th>Pattern</th>
              <th>Send Command</th>
              <th style="width: 60px;">Timeout</th>
            </tr>
          </thead>
          <tbody>
            {#each chainConn.Expect as entry, idx}
              {@const isChecked = selectedIndices.includes(idx)}
              <tr class={isChecked ? 'is-selected' : ''} onclick={() => toggleIndex(idx)}>
                <td style="text-align: center;">
                  <input
                    type="checkbox"
                    checked={isChecked}
                    onclick={(e) => {
                      e.stopPropagation();
                      toggleIndex(idx);
                    }}
                  />
                </td>
                <td class="col-idx">{idx + 1}</td>
                <td class="col-code"><code>{entry.Pattern || '(immediate)'}</code></td>
                <td class="col-code">
                  <code>{entry.Send}{#if entry.AppendReturn}↵{/if}</code>
                </td>
                <td>{entry.Timeout > 0 ? `${entry.Timeout}s` : 'default'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <div class="empty-hint">No Expect commands configured for this connection.</div>
      {/if}
    </div>
  </div>

  {#snippet actions()}
    <Button label="Cancel" onclick={onclose} />
    <Button
      label="Execute Chain"
      isDefault
      disabled={selectedIndices.length === 0}
      onclick={handleExecute}
    />
  {/snippet}
</Dialog>

<style>
  .chain-dialog-body {
    display: flex;
    flex-direction: column;
    gap: 10px;
    font-size: 12px;
  }

  .chain-info-box {
    padding: 8px 12px;
    background: var(--gtk-header-bg, #f4f4f4);
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 4px;
    line-height: 1.5;
  }

  .chain-info-box p {
    margin: 2px 0;
  }

  .chain-desc {
    margin-top: 6px !important;
    color: var(--gtk-muted-fg);
  }

  .chain-toolbar {
    display: flex;
    gap: 8px;
  }

  .entries-table-wrap {
    max-height: 240px;
    overflow-y: auto;
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 3px;
    background: var(--gtk-base, #ffffff);
  }

  .entries-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
  }

  .entries-table th {
    background: var(--gtk-header-bg, #ececec);
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    padding: 4px 6px;
    text-align: left;
    font-weight: 600;
  }

  .entries-table td {
    padding: 4px 6px;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .entries-table tr:hover {
    background: var(--gtk-bg-color);
    cursor: pointer;
  }

  .entries-table tr.is-selected {
    background: rgba(74, 144, 217, 0.1);
  }

  .col-idx {
    font-weight: bold;
    color: var(--gtk-muted-fg);
  }

  .col-code code {
    background: var(--gtk-header-bg);
    padding: 1px 4px;
    border-radius: 2px;
    font-family: monospace;
    font-size: 11px;
  }

  .empty-hint {
    padding: 20px;
    text-align: center;
    color: var(--gtk-muted-fg);
  }
</style>
