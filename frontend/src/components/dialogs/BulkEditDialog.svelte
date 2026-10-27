<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
  import type { BulkEditRule } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/config/models.js';
  import { loadTree } from '../../lib/stores/config';

  interface Props {
    open: boolean;
    connIDs?: string[];
    onclose: () => void;
  }

  let { open = false, connIDs = [], onclose }: Props = $props();

  interface RowData {
    change: boolean;
    field: string;
    label: string;
    newValue: string;
    useRegex: boolean;
    matchRegex: string;
  }

  let rows = $state<RowData[]>([
    { change: false, field: 'user', label: 'User', newValue: '', useRegex: false, matchRegex: '' },
    { change: false, field: 'host', label: 'Host / IP', newValue: '', useRegex: false, matchRegex: '' },
    { change: false, field: 'port', label: 'Port', newValue: '', useRegex: false, matchRegex: '' },
    { change: false, field: 'title', label: 'Title', newValue: '', useRegex: false, matchRegex: '' },
    { change: false, field: 'description', label: 'Description', newValue: '', useRegex: false, matchRegex: '' },
  ]);

  let statusMsg = $state<string>('');
  let isSaving = $state<boolean>(false);

  $effect(() => {
    if (open) {
      statusMsg = '';
      for (const r of rows) {
        r.change = false;
        r.newValue = '';
        r.useRegex = false;
        r.matchRegex = '';
      }
    }
  });

  async function handleApply() {
    const activeRules: BulkEditRule[] = rows
      .filter((r) => r.change)
      .map((r) => ({
        field: r.field,
        newValue: r.newValue,
        useRegex: r.useRegex,
        matchRegex: r.matchRegex,
      }));

    if (activeRules.length === 0) {
      statusMsg = 'Please select at least one field to change.';
      return;
    }

    isSaving = true;
    try {
      const res = await ConfigService.ApplyBulkEdit(connIDs, activeRules as any);
      statusMsg = `Modified ${res.modifiedCount} connection(s).`;
      await loadTree();
      setTimeout(() => {
        onclose();
      }, 1000);
    } catch (err: any) {
      statusMsg = `Error applying bulk edit: ${err?.message || err}`;
    } finally {
      isSaving = false;
    }
  }
</script>

{#if open}
  <Dialog title={`Bulk Edit (${connIDs.length} connections selected)`} width={680} {onclose}>
    <div class="bulk-body">
      <p class="hint">
        Select fields to update across all selected connections. Regex substitutions support capture groups ($1, $2, etc.).
      </p>

      <table class="bulk-table">
        <thead>
          <tr>
            <th style="width: 40px;">Change</th>
            <th style="width: 90px;">Field</th>
            <th>New Value / Replacement</th>
            <th style="width: 70px;">RegExp</th>
            <th>Match Pattern</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as row}
            <tr>
              <td style="text-align: center;">
                <CheckButton bind:checked={row.change} />
              </td>
              <td class="field-name">{row.label}</td>
              <td>
                <Entry bind:value={row.newValue} disabled={!row.change} placeholder="Value..." />
              </td>
              <td style="text-align: center;">
                <CheckButton bind:checked={row.useRegex} disabled={!row.change} />
              </td>
              <td>
                <Entry
                  bind:value={row.matchRegex}
                  disabled={!row.change || !row.useRegex}
                  placeholder={row.useRegex ? '^prefix_(.*)' : ''}
                />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      {#if statusMsg}
        <div class="status-box">{statusMsg}</div>
      {/if}
    </div>

    {#snippet actions()}
      <Button label="Cancel" onclick={onclose} />
      <Button label="Apply Changes" icon="gtk-apply" disabled={isSaving} onclick={handleApply} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .bulk-body {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .hint {
    font-size: 12px;
    color: var(--text-muted, #aaa);
    margin: 0;
  }

  .bulk-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  .bulk-table th {
    background: var(--bg-tertiary, #222);
    padding: 6px 8px;
    border: 1px solid var(--border-color, #444);
    font-weight: 600;
  }

  .bulk-table td {
    padding: 4px 6px;
    border: 1px solid var(--border-color, #444);
    vertical-align: middle;
  }

  .field-name {
    font-weight: 500;
  }

  .status-box {
    font-size: 12px;
    background: var(--bg-tertiary, #222);
    padding: 6px 10px;
    border-radius: 4px;
    border-left: 3px solid var(--accent-color, #3584e4);
  }
</style>
