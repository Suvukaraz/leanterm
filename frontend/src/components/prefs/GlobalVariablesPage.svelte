<script lang="ts">
  import EditableList from '../edit/EditableList.svelte';
  import Entry from '../gtk/Entry.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  function createVariable() {
    return {
      Name: '',
      Value: '',
      Hidden: false,
    };
  }
</script>

<div class="global-vars-page">
  <div class="info-note">
    Global variables can be referenced anywhere using <code>&lt;GV:variable_name&gt;</code>.
  </div>

  <EditableList
    title="Global Variables"
    bind:items={settings.variables}
    createItem={createVariable}
    addLabel="Add Global Variable"
  >
    {#snippet itemSnippet(item: any, i: number, removeItem: (idx: number) => void)}
      <div class="var-row">
        <span class="field-label">Variable:</span>
        <div class="name-entry">
          <Entry
            bind:value={item.Name}
            placeholder="Name"
            tooltip="Variable name (accessed via <GV:name>)"
          />
        </div>
        <span class="field-label" style="margin-left: 8px;">Value:</span>
        <div class="val-entry">
          <Entry
            type={item.Hidden ? 'password' : 'text'}
            bind:value={item.Value}
            placeholder="Value"
            tooltip="Variable value"
          />
        </div>
        <CheckButton
          bind:checked={item.Hidden}
          label="Hide"
          tooltip="Store encrypted in secrets vault"
        />
        <Button
          icon="gtk-delete"
          tooltip="Delete variable"
          onclick={() => removeItem(i)}
        />
      </div>
    {/snippet}
  </EditableList>
</div>

<style>
  .global-vars-page {
    padding: 10px;
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
  }

  .info-note {
    font-size: 0.9em;
    color: var(--gtk-muted-fg);
    background: var(--gtk-row-alt);
    padding: 6px 8px;
    border-radius: 4px;
  }

  .var-row {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
  }

  .field-label {
    font-size: 0.85em;
    color: var(--gtk-fg-color);
  }

  .name-entry {
    flex: 1;
  }

  .val-entry {
    flex: 2;
  }
</style>
