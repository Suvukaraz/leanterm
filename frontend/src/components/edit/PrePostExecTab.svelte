<script lang="ts">
  import EditableList from './EditableList.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    title: string;
    tooltip?: string;
    items: any[];
    disabled?: boolean;
  }

  let {
    title,
    tooltip = '',
    items = $bindable([]),
    disabled = false
  }: Props = $props();

  function createEntry() {
    return {
      Command: '',
      Ask: false,
      Default: true,
    };
  }

</script>

<div class="prepost-tab-container">
  <div class="top-bar">
    <span class="info-text">{tooltip}</span>
  </div>

  <EditableList
    {title}
    {tooltip}
    bind:items={items}
    createItem={createEntry}
    addLabel="Add Command"
    {disabled}
  >
    {#snippet itemSnippet(item: any, i: number, removeItem: (idx: number) => void)}
      <div class="cmd-row">
        <CheckButton
          bind:checked={item.Ask}
          label="Ask: "
          tooltip="Ask before executing"
          {disabled}
        />
        <CheckButton
          bind:checked={item.Default}
          label="Default: "
          tooltip="Selected by default in ask dialog"
          {disabled}
        />
        <div class="cmd-entry">
          <Entry
            bind:value={item.Command}
            placeholder="Command to execute locally"
            {disabled}
          />
        </div>
        <Button
          icon="gtk-delete"
          tooltip="Delete command"
          {disabled}
          onclick={() => removeItem(i)}
        />
      </div>
    {/snippet}
  </EditableList>
</div>

<style>
  .prepost-tab-container {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
  }

  .top-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .info-text {
    font-size: 0.9em;
    color: var(--gtk-muted-fg);
  }

  .cmd-row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  .cmd-entry {
    flex: 1;
  }
</style>
