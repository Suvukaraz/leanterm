<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';

  interface Props {
    open: boolean;
    connID: string;
    onclose: () => void;
  }

  let { open = false, connID, onclose }: Props = $props();

  interface VarRow {
    id: string;
    value: string;
    hidden: boolean;
  }

  let rows = $state<VarRow[]>([]);
  let isSaving = $state<boolean>(false);
  let statusText = $state<string>('');
  let currentEdit = $state<any>(null);

  $effect(() => {
    if (open && connID) {
      statusText = '';
      loadConnVars();
    }
  });

  async function loadConnVars() {
    try {
      const edit: any = await ConfigService.GetConnectionForEdit(connID);
      currentEdit = edit;
      const existing = edit?.Variables || edit?.variables || [];
      const newRows: VarRow[] = [];
      for (let i = 0; i < 10; i++) {
        const item = existing[i];
        newRows.push({
          id: item?.ID || item?.id || `var_${i}`,
          value: item?.Value || item?.value || '',
          hidden: !!(item?.Hidden ?? item?.hidden),
        });
      }
      rows = newRows;
    } catch (err) {
      console.error('Failed to load connection variables:', err);
    }
  }

  async function handleSave() {
    if (!currentEdit) return;
    isSaving = true;
    try {
      currentEdit.Variables = rows.map((r) => ({
        ID: r.id,
        Value: r.value,
        Hidden: r.hidden,
      }));
      await ConfigService.SaveConnection(connID, currentEdit);
      onclose();
    } catch (err: any) {
      statusText = `Error: ${err?.message || err}`;
    } finally {
      isSaving = false;
    }
  }
</script>

{#if open}
  <Dialog title="Edit Local Variables (<V:0>..<V:9>)" width={520} {onclose}>
    <div class="vars-body">
      <p class="hint">
        Configure user variables for this connection. Reference them in commands or scripts using &lt;V:0&gt; through &lt;V:9&gt;.
      </p>

      <div class="vars-table">
        <div class="vars-header">
          <span style="width: 50px;">Var</span>
          <span style="flex: 1;">Value</span>
          <span style="width: 70px; text-align: center;">Hide</span>
        </div>
        {#each rows as row, i}
          <div class="vars-row">
            <span class="var-tag">&lt;V:{i}&gt;</span>
            <div style="flex: 1;">
              <Entry
                bind:value={row.value}
                type={row.hidden ? 'password' : 'text'}
                placeholder="Value..."
              />
            </div>
            <div style="width: 70px; display: flex; justify-content: center;">
              <CheckButton bind:checked={row.hidden} />
            </div>
          </div>
        {/each}
      </div>

      {#if statusText}
        <div class="status-box">{statusText}</div>
      {/if}
    </div>

    {#snippet actions()}
      <Button label="Cancel" onclick={onclose} />
      <Button label="Save" icon="gtk-save" disabled={isSaving} onclick={handleSave} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .vars-body {
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

  .vars-table {
    border: 1px solid var(--border-color, #444);
    border-radius: 4px;
    background: var(--bg-tertiary, #222);
    overflow: hidden;
  }

  .vars-header {
    display: flex;
    align-items: center;
    padding: 6px 10px;
    background: var(--bg-secondary, #2a2a2a);
    font-size: 11px;
    font-weight: 600;
    border-bottom: 1px solid var(--border-color, #444);
  }

  .vars-row {
    display: flex;
    align-items: center;
    padding: 4px 10px;
    gap: 8px;
    border-bottom: 1px solid var(--border-color, #333);
  }

  .vars-row:last-child {
    border-bottom: none;
  }

  .var-tag {
    width: 50px;
    font-family: monospace;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent-color, #3584e4);
  }

  .status-box {
    font-size: 12px;
    color: var(--gtk-error-fg);
    padding: 4px 8px;
  }
</style>
