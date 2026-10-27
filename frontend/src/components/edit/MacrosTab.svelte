<script lang="ts">
  import EditableList from './EditableList.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    title: string;
    items: any[];
    isRemote?: boolean;
    disabled?: boolean;
  }

  let {
    title,
    items = $bindable([]),
    isRemote = false,
    disabled = false
  }: Props = $props();

  function createMacro() {
    return {
      Command: '',
      Description: '',
      Confirm: false,
      AppendReturn: true,
    };
  }
</script>

<div class="macros-tab-container">
  <EditableList
    {title}
    bind:items={items}
    createItem={createMacro}
    addLabel="Add Macro"
    {disabled}
  >
    {#snippet itemSnippet(item: any, i: number, removeItem: (idx: number) => void)}
      <div class="macro-row">
        <CheckButton
          bind:checked={item.Confirm}
          label="Confirm"
          tooltip="Ask confirmation before running"
          {disabled}
        />
        <div class="field-wrap">
          <Entry
            bind:value={item.Command}
            placeholder="Command"
            tooltip="Command text"
            {disabled}
          />
        </div>
        <div class="field-wrap">
          <Entry
            bind:value={item.Description}
            placeholder="Description"
            tooltip="Macro description"
            {disabled}
          />
        </div>
        {#if isRemote}
          <CheckButton
            bind:checked={item.AppendReturn}
            label="Intro"
            tooltip="Append Enter/Return after sending"
            {disabled}
          />
        {/if}
        <Button
          icon="gtk-delete"
          tooltip="Delete macro"
          {disabled}
          onclick={() => removeItem(i)}
        />
      </div>
    {/snippet}
  </EditableList>
</div>

<style>
  .macros-tab-container {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  .macro-row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  .field-wrap {
    flex: 1;
  }
</style>
