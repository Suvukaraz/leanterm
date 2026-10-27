<script lang="ts">
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    items: any[];
    disabled?: boolean;
  }

  let { items = $bindable([]), disabled = false }: Props = $props();

  // Ensure at least 10 slots <V:0> to <V:9>
  $effect(() => {
    if (items.length < 10) {
      const copy = [...items];
      while (copy.length < 10) {
        copy.push({
          ID: `v${copy.length}`,
          Value: '',
          Hidden: false,
        });
      }
      items = copy;
    }
  });

  function clearVariable(index: number) {
    if (items[index]) {
      items[index].Value = '';
    }
  }
</script>

<div class="variables-tab-container">
  <div class="info-header">
    Define variables accessible via <code>&lt;V:0&gt;</code> to <code>&lt;V:9&gt;</code> in commands, titles, and expect rules.
  </div>

  <div class="var-list">
    {#each items.slice(0, 10) as variable, i}
      <div class="var-row">
        <span class="var-label">&lt;V:{i}&gt;</span>
        <div class="var-entry">
          <Entry
            type={variable.Hidden ? 'password' : 'text'}
            bind:value={variable.Value}
            placeholder={`Value for <V:${i}>`}
            {disabled}
          />
        </div>
        <CheckButton
          bind:checked={variable.Hidden}
          label="Hide"
          tooltip="Store encrypted in secrets vault"
          {disabled}
        />
        <Button
          icon="gtk-clear"
          label="Clear"
          tooltip="Clear value"
          {disabled}
          onclick={() => clearVariable(i)}
        />
      </div>
    {/each}
  </div>
</div>

<style>
  .variables-tab-container {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-sizing: border-box;
    overflow-y: auto;
  }

  .info-header {
    font-size: 0.9em;
    color: var(--gtk-muted-fg);
    background: var(--gtk-row-alt);
    padding: 6px 8px;
    border-radius: 4px;
  }

  .var-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .var-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .var-label {
    min-width: 50px;
    font-weight: bold;
    font-family: monospace;
    color: var(--gtk-fg-color);
  }

  .var-entry {
    flex: 1;
  }
</style>
