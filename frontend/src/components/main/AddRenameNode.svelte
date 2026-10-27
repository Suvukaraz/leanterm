<script lang="ts">
  import { addRenameDialog } from '../../lib/stores/ui';
  import { addNode, renameNode } from '../../lib/stores/config';
  import Dialog from '../gtk/Dialog.svelte';
  import Entry from '../gtk/Entry.svelte';
  import Button from '../gtk/Button.svelte';

  let nameValue = $state('');
  let titleValue = $state('');
  let errorMsg = $state('');

  // Sync inputs when dialog opens
  $effect(() => {
    if ($addRenameDialog.open) {
      nameValue = $addRenameDialog.currentName || '';
      titleValue = $addRenameDialog.currentTitle || '';
      errorMsg = '';
    }
  });

  async function handleConfirm() {
    const trimmedName = nameValue.trim();
    if (!trimmedName) {
      errorMsg = 'Name cannot be empty';
      return;
    }

    try {
      if ($addRenameDialog.isRename) {
        await renameNode($addRenameDialog.nodeId, trimmedName, titleValue.trim());
      } else {
        // The title defaults to the name (plus group, see "append group name to
        // title"); it can be changed later in Rename or Edit Connection.
        await addNode($addRenameDialog.parentId, $addRenameDialog.isGroup, trimmedName, '');
      }
      close();
    } catch (err: any) {
      errorMsg = err?.message || String(err);
    }
  }

  function close() {
    addRenameDialog.update((s) => ({ ...s, open: false }));
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleConfirm();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  }

  let bannerText = $derived(
    $addRenameDialog.isRename
      ? `Renaming ${$addRenameDialog.isGroup ? 'Group' : 'Connection'} '${$addRenameDialog.currentName}'`
      : `Adding new ${$addRenameDialog.isGroup ? 'Group' : 'Connection'} into '${$addRenameDialog.parentId || 'ROOT'}'`
  );
</script>

<Dialog
  open={$addRenameDialog.open}
  title={$addRenameDialog.isRename ? 'Rename Node' : 'Add Node'}
  {bannerText}
  width={420}
  onclose={close}
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="add-rename-body" onkeydown={handleKeyDown}>
    {#if errorMsg}
      <div class="error-banner">{errorMsg}</div>
    {/if}

    <div class="form-row">
      <label for="node-name-input"><b>Enter new NAME:</b></label>
      <Entry
        value={nameValue}
        oninput={(v) => (nameValue = v)}
        placeholder={$addRenameDialog.isGroup ? 'e.g. Production Servers' : 'e.g. web-prod-01'}
      />
    </div>

    {#if $addRenameDialog.isRename && !$addRenameDialog.isGroup}
      <div class="form-row" style="margin-top: 10px;">
        <label for="node-title-input"><b>Enter new TITLE:</b></label>
        <Entry
          value={titleValue}
          oninput={(v) => (titleValue = v)}
          placeholder="e.g. web-prod-01 (Prod US-East)"
        />
      </div>
    {/if}
  </div>

  {#snippet actions()}
    <Button label="Cancel" icon="gtk-close" onclick={close} />
    <Button label="OK" icon="gtk-connect" isDefault={true} onclick={handleConfirm} />
  {/snippet}
</Dialog>

<style>
  .add-rename-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 6px 0;
  }
  .form-row {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
  }
  .error-banner {
    background: var(--gtk-error-bg);
    color: var(--gtk-error-fg);
    border: 1px solid var(--gtk-error-border);
    padding: 6px 10px;
    border-radius: 3px;
    font-size: 12px;
    margin-bottom: 4px;
  }
</style>
