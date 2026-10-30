<script lang="ts">
  import { onMount } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import {
    commandPanelVisible,
    commandPanelWidth,
    searchVisible,
    isReadOnly,
    isLocked,
    aboutDialogOpen,
    addRenameDialog,
    deleteConfirmDialog,
    editConnDialog,
    prefsDialogOpen,
    wolDialog,
    bulkEditDialogOpen,
    editVariablesDialog,
    clusterAdminDialogOpen,
    pccDialogOpen,
  } from '../../lib/stores/ui';
  import {
    settings,
    selectedIds,
    findNode,
    tree,
    save,
  } from '../../lib/stores/config';
  import {
    activeTab,
    openLocalShell,
  } from '../../lib/stores/sessions';
  import CommandPanel from './CommandPanel.svelte';
  import ConnectionPanel from './ConnectionPanel.svelte';
  import MainButtonBar from './MainButtonBar.svelte';
  import QuitConfirm from './QuitConfirm.svelte';
  import AddRenameNode from './AddRenameNode.svelte';
  import About from './About.svelte';
  import DeleteConfirmDialog from './DeleteConfirmDialog.svelte';
  import QuickConnectDialog from '../dialogs/QuickConnectDialog.svelte';
  import EditConnectionWindow from '../edit/EditConnectionWindow.svelte';
  import PreferencesWindow from '../prefs/PreferencesWindow.svelte';
  import ClusterAdminWindow from '../cluster/ClusterAdminWindow.svelte';
  import PCCWindow from '../cluster/PCCWindow.svelte';
  import WakeOnLanDialog from '../dialogs/WakeOnLanDialog.svelte';
  import BulkEditDialog from '../dialogs/BulkEditDialog.svelte';
  import EditVariablesDialog from '../dialogs/EditVariablesDialog.svelte';
  import GuiLockOverlay from '../dialogs/GuiLockOverlay.svelte';
  import PromptDialog from '../dialogs/PromptDialog.svelte';
  import Paned from '../gtk/Paned.svelte';

  let isCompact = $derived(
    $settings?.general?.Layout === 'compact'
  );

  let treePosition = $derived(
    $settings?.appearance?.TreePosition || 'left'
  );

  let autoHideButtonBar = $derived(
    $settings?.appearance?.AutoHideButtonBar ?? false
  );

  let buttonBarPosition = $derived<'top' | 'bottom'>(
    $settings?.appearance?.ButtonBarPosition === 'bottom' ? 'bottom' : 'top'
  );

  let hideButtonBar = $derived(
    isCompact || (autoHideButtonBar && $activeTab !== 'info')
  );

  let windowTitle = $derived(
    $isReadOnly ? 'LeanTerm - READ ONLY MODE' : 'LeanTerm'
  );

  $effect(() => {
    document.title = windowTitle;
  });

  function handleKeyDown(e: KeyboardEvent) {
    // Keys typed into a terminal, an input field or a dialog belong to that element
    // (Ctrl+S in nano, F2 in mc, Delete in an entry, ...).
    const target = e.target as HTMLElement | null;
    if (e.defaultPrevented || target?.closest?.('.xterm, input, textarea, select, [role="dialog"], [contenteditable="true"]')) {
      return;
    }
    // F9: toggle command panel
    if (e.key === 'F9') {
      e.preventDefault();
      commandPanelVisible.update((v) => !v);
    }
    // Ctrl+F: toggle search
    else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
      e.preventDefault();
      searchVisible.update((v) => !v);
    }
    // Ctrl+S: save
    else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      save();
    }
    // Ctrl+Alt+L: local shell
    // (AltGr arrives as Ctrl+Alt on Windows and types characters.)
    else if (e.ctrlKey && e.altKey && !e.getModifierState?.('AltGraph') && e.key.toLowerCase() === 'l') {
      e.preventDefault();
      openLocalShell('tab');
    }
    // F2: rename selected
    else if (e.key === 'F2') {
      if ($selectedIds.length === 1 && $selectedIds[0] !== '') {
        const node = findNode($selectedIds[0], $tree);
        if (node && !node.protected) {
          e.preventDefault();
          addRenameDialog.set({
            open: true,
            isRename: true,
            isGroup: node.isGroup,
            parentId: node.parentId || '',
            nodeId: node.id,
            currentName: node.name,
            currentTitle: node.title || '',
          });
        }
      }
    }
    // Delete: delete selected
    else if (e.key === 'Delete') {
      // If not focusing an input or textarea
      const tag = (document.activeElement?.tagName || '').toLowerCase();
      if (tag !== 'input' && tag !== 'textarea') {
        const validIds = $selectedIds.filter((id) => id !== '');
        if (validIds.length > 0) {
          e.preventDefault();
          deleteConfirmDialog.set({
            open: true,
            nodeIds: validIds,
            count: validIds.length,
          });
        }
      }
    }
    // Esc: hide search if focused outside
    else if (e.key === 'Escape') {
      if ($searchVisible) {
        searchVisible.set(false);
      }
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeyDown);

    const unsubs = [
      Events.On('ui:open:quick-connect', () => {
        editConnDialog.set({ open: true, connId: 'quick-' + crypto.randomUUID(), isQuick: true });
      }),
      Events.On('ui:open:edit', (ev: any) => {
        const id = ev?.data || ev;
        if (id) {
          editConnDialog.set({ open: true, connId: String(id), isQuick: false });
        }
      }),
      Events.On('ui:open:preferences', () => {
        prefsDialogOpen.set(true);
      }),
    ];

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      unsubs.forEach((u) => u && u());
    };
  });
</script>

<div class="leanterm-main-window">
  <GuiLockOverlay locked={$isLocked} onunlock={() => isLocked.set(false)} />

  {#if !hideButtonBar && buttonBarPosition === 'top'}
    <MainButtonBar position="top" />
  {/if}

  <div class="main-body">
    {#if !$commandPanelVisible}
      <div class="full-panel">
        <ConnectionPanel />
      </div>
    {:else}
      <Paned
        orientation="horizontal"
        initialSplit={$commandPanelWidth}
        fixed={treePosition === 'right' ? 'second' : 'first'}
        minFirst={treePosition === 'right' ? 300 : 200}
        minSecond={treePosition === 'right' ? 200 : 300}
        onchange={(pos: number) => commandPanelWidth.set(pos)}
      >
        {#snippet first()}
          {#if treePosition === 'right'}
            <ConnectionPanel />
          {:else}
            <CommandPanel />
          {/if}
        {/snippet}

        {#snippet second()}
          {#if treePosition === 'right'}
            <CommandPanel />
          {:else}
            <ConnectionPanel />
          {/if}
        {/snippet}
      </Paned>
    {/if}
  </div>

  {#if !hideButtonBar && buttonBarPosition === 'bottom'}
    <MainButtonBar position="bottom" />
  {/if}

  <!-- Dialogs -->
  <AddRenameNode />
  <About />
  <DeleteConfirmDialog />
  <QuickConnectDialog />
  <EditConnectionWindow />
  <PreferencesWindow />
  <ClusterAdminWindow
    open={$clusterAdminDialogOpen}
    onclose={() => clusterAdminDialogOpen.set(false)}
    onopenPCC={() => pccDialogOpen.set(true)}
  />
  <PCCWindow
    open={$pccDialogOpen}
    onclose={() => pccDialogOpen.set(false)}
    onopenClusterAdmin={() => clusterAdminDialogOpen.set(true)}
  />
  <WakeOnLanDialog
    open={$wolDialog.open}
    initialHost={$wolDialog.host}
    initialMAC={$wolDialog.mac}
    onclose={() => wolDialog.update((w) => ({ ...w, open: false }))}
  />
  <BulkEditDialog
    open={$bulkEditDialogOpen.open}
    connIDs={$bulkEditDialogOpen.connIds}
    onclose={() => bulkEditDialogOpen.set({ open: false, connIds: [] })}
  />
  <EditVariablesDialog
    open={$editVariablesDialog.open}
    connID={$editVariablesDialog.connId}
    onclose={() => editVariablesDialog.set({ open: false, connId: '' })}
  />
  <PromptDialog />
  <QuitConfirm />
</div>

<style>
  .leanterm-main-window {
    width: 100vw;
    height: 100vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
    background-color: var(--gtk-base-color, #ffffff);
    color: var(--gtk-text-color, #2e3436);
  }

  .main-body {
    flex: 1;
    display: flex;
    min-height: 0;
    overflow: hidden;
    position: relative;
  }

  .full-panel {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }
</style>
