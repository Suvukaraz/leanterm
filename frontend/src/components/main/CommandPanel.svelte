<script lang="ts">
  import {
    tree,
    selectedIds,
    settings,
    findNode,
    collectFavourites,
    setFavourite,
    search,
  } from '../../lib/stores/config';
  import {
    commandPanelVisible,
    clustersVisible,
    searchVisible,
    searchQuery,
    searchField,
    searchLastMatchId,
    activeLeftTab,
    addRenameDialog,
    deleteConfirmDialog,
    editConnDialog,
    prefsDialogOpen,
    clusterAdminDialogOpen,
    pccDialogOpen,
    requestQuit,
    type LeftTab,
    type SearchField,
  } from '../../lib/stores/ui';
  import {
    sessions,
    history,
    openConnections,
    openLocalShell,
  } from '../../lib/stores/sessions';
  import ConnectionTree from './ConnectionTree.svelte';
  import Button from '../gtk/Button.svelte';
  import ToggleButton from '../gtk/ToggleButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Notebook from '../gtk/Notebook.svelte';
  import { getIcon } from '../../lib/theme/icons';

  // Selected node calculations
  let singleSelectedNode = $derived.by(() => {
    if ($selectedIds.length === 1) {
      return findNode($selectedIds[0], $tree);
    }
    return null;
  });

  let isRootSelected = $derived(
    $selectedIds.length === 1 && $selectedIds[0] === ''
  );

  let isProtected = $derived(
    singleSelectedNode?.protected ?? false
  );

  let canAdd = $derived(
    $activeLeftTab === 'connections' &&
      $selectedIds.length === 1 &&
      (isRootSelected || singleSelectedNode?.isGroup) &&
      !isProtected
  );

  let canEdit = $derived(
    $activeLeftTab === 'connections' &&
      $selectedIds.length >= 1 &&
      !isRootSelected
  );

  let canRename = $derived(
    $activeLeftTab === 'connections' &&
      $selectedIds.length === 1 &&
      !isRootSelected &&
      !isProtected
  );

  let canDelete = $derived(
    $activeLeftTab === 'connections' &&
      $selectedIds.length >= 1 &&
      !isRootSelected &&
      !isProtected
  );

  let canConnect = $derived.by(() => {
    if ($activeLeftTab === 'connections') return $selectedIds.length >= 1;
    if ($activeLeftTab === 'favourites') return $selectedIds.length >= 1;
    if ($activeLeftTab === 'history') return $selectedIds.length >= 1;
    if ($activeLeftTab === 'clusters') return true;
    return false;
  });

  let isFavouriteActive = $derived(
    $selectedIds.length === 1 &&
      singleSelectedNode &&
      !singleSelectedNode.isGroup &&
      singleSelectedNode.favourite
  );

  let isCompact = $derived(
    $settings?.general?.Layout === 'compact'
  );

  let showTreeTitles = $derived(
    $settings?.appearance?.ShowTreeTitles ?? true
  );

  // Tabs for nbTree
  let treeTabs = $derived([
    {
      id: 'connections',
      label: !isCompact && (showTreeTitles || $activeLeftTab === 'connections') ? ' Connections' : '',
      icon: 'asbru-treelist',
      tooltip: 'Connection Tree',
    },
    {
      id: 'favourites',
      label: !isCompact && (showTreeTitles || $activeLeftTab === 'favourites') ? ' Favourites' : '',
      icon: 'asbru-favourite-on',
      tooltip: 'Favourites',
    },
    {
      id: 'history',
      label: !isCompact && (showTreeTitles || $activeLeftTab === 'history') ? ' History' : '',
      icon: 'asbru-history',
      tooltip: 'Connection History',
    },
    ...($clustersVisible
      ? [
          {
            id: 'clusters',
            label: !isCompact && (showTreeTitles || $activeLeftTab === 'clusters') ? ' Clusters' : '',
            icon: 'asbru-cluster-manager2',
            tooltip: 'Clusters',
          },
        ]
      : []),
  ]);

  // Leave the Clusters tab when the cluster functions get hidden.
  $effect(() => {
    if (!$clustersVisible && $activeLeftTab === 'clusters') activeLeftTab.set('connections');
  });

  // Actions
  function handleAddGroup() {
    if (!canAdd) return;
    const parent = singleSelectedNode?.id ?? '';
    addRenameDialog.set({
      open: true,
      isRename: false,
      isGroup: true,
      parentId: parent,
      nodeId: '',
      currentName: '',
      currentTitle: '',
    });
  }

  function handleAddConn() {
    if (!canAdd) return;
    const parent = singleSelectedNode?.id ?? '';
    addRenameDialog.set({
      open: true,
      isRename: false,
      isGroup: false,
      parentId: parent,
      nodeId: '',
      currentName: '',
      currentTitle: '',
    });
  }

  function handleEdit() {
    if (!canEdit) return;
    if (singleSelectedNode && !singleSelectedNode.isGroup) {
      editConnDialog.set({
        open: true,
        connId: singleSelectedNode.id,
        isQuick: false,
      });
    }
  }

  function handleRename() {
    if (!canRename || !singleSelectedNode) return;
    addRenameDialog.set({
      open: true,
      isRename: true,
      isGroup: singleSelectedNode.isGroup,
      parentId: singleSelectedNode.parentId || '',
      nodeId: singleSelectedNode.id,
      currentName: singleSelectedNode.name,
      currentTitle: singleSelectedNode.title || '',
    });
  }

  function handleDelete() {
    if (!canDelete) return;
    deleteConfirmDialog.set({
      open: true,
      nodeIds: $selectedIds,
      count: $selectedIds.length,
    });
  }

  function handleConnect() {
    if ($activeLeftTab === 'connections') {
      const ids: string[] = [];
      for (const id of $selectedIds) {
        const n = findNode(id, $tree);
        if (n && !n.isGroup) ids.push(n.id);
        else if (n && n.isGroup) {
          // Flatten group connections
          const traverse = (node: any) => {
            if (!node.isGroup && node.id) ids.push(node.id);
            if (node.children) node.children.forEach(traverse);
          };
          traverse(n);
        }
      }
      if (ids.length > 0) openConnections(ids, 'tab');
    } else if ($activeLeftTab === 'favourites') {
      if ($selectedIds.length > 0) openConnections($selectedIds, 'tab');
    }
  }

  function handleQuickConnect() {
    editConnDialog.set({
      open: true,
      connId: `quick-${Date.now()}`,
      isQuick: true,
    });
  }

  function handleToggleFavourite() {
    if ($selectedIds.length === 1 && singleSelectedNode && !singleSelectedNode.isGroup) {
      setFavourite(singleSelectedNode.id, !singleSelectedNode.favourite);
    }
  }

  // Search actions
  async function performSearch(backwards = false) {
    if (!$searchQuery.trim()) return;
    const matchId = await search(
      $searchQuery.trim(),
      $searchField,
      $searchLastMatchId,
      backwards
    );
    if (matchId) {
      searchLastMatchId.set(matchId);
      selectedIds.set([matchId]);
    }
  }

  function handleSearchKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      performSearch(e.shiftKey);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      searchVisible.set(false);
    }
  }
</script>

<aside class="command-panel" aria-label="Command Panel">
  <!-- 1. Toolbar hbuttonbox1 -->
  <div class="cp-toolbar" role="toolbar" aria-label="Tree operations toolbar">
    <button
      type="button"
      class="button-cp"
      title="Add a new group in the selected node"
      disabled={!canAdd}
      onclick={handleAddGroup}
    >
      <img src={getIcon('asbru-group-add')} alt="Add Group" width="16" height="16" />
    </button>
    <button
      type="button"
      class="button-cp"
      title="Add a new connection in the selected node"
      disabled={!canAdd}
      onclick={handleAddConn}
    >
      <img src={getIcon('asbru-node-add')} alt="Add Conn" width="16" height="16" />
    </button>
    <button
      type="button"
      class="button-cp"
      title="Edit the currently selected node"
      disabled={!canEdit}
      onclick={handleEdit}
    >
      <img src={getIcon('gtk-edit')} alt="Edit Node" width="16" height="16" />
    </button>
    <button
      type="button"
      class="button-cp"
      title="Rename the currently selected node"
      disabled={!canRename}
      onclick={handleRename}
    >
      <img src={getIcon('gtk-spell-check')} alt="Rename Node" width="16" height="16" />
    </button>
    <button
      type="button"
      class="button-cp"
      title="Delete the selected node(s)"
      disabled={!canDelete}
      onclick={handleDelete}
    >
      <img src={getIcon('gtk-delete')} alt="Delete Node" width="16" height="16" />
    </button>
    {#if isCompact}
      <button
        type="button"
        class="button-cp"
        title="Close Window"
        onclick={requestQuit}
      >
        <img src={getIcon('gtk-close')} alt="Close" width="16" height="16" />
      </button>
    {/if}
  </div>

  <!-- 2. Notebook nbTree with 4 tabs -->
  <div class="cp-tree-notebook">
    <Notebook
      tabs={treeTabs}
      activeTab={$activeLeftTab}
      onchange={(id) => activeLeftTab.set(id as LeftTab)}
    >
      {#snippet children(tabId: string)}
        <div class="tab-page-content">
          {#if tabId === 'connections'}
            <ConnectionTree />
          {:else if tabId === 'favourites'}
            <div class="flat-list">
              {#each collectFavourites($tree) as fav (fav.id)}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class={`flat-list-row ${$selectedIds.includes(fav.id) ? 'is-selected' : ''}`}
                  onclick={() => selectedIds.set([fav.id])}
                  ondblclick={() => openConnections([fav.id], 'tab')}
                >
                  <img
                    src={getIcon(`asbru-method-${fav.method?.toLowerCase() || 'ssh'}`)}
                    alt=""
                    width="16"
                    height="16"
                  />
                  <span class="flat-row-name">{fav.name}</span>
                  <button
                    type="button"
                    class="fav-toggle-btn"
                    title="Remove from favourites"
                    onclick={(e) => {
                      e.stopPropagation();
                      setFavourite(fav.id, false);
                    }}
                  >
                    ★
                  </button>
                </div>
              {/each}
              {#if collectFavourites($tree).length === 0}
                <div class="empty-hint">No favourites added yet</div>
              {/if}
            </div>
          {:else if tabId === 'history'}
            <div class="flat-list history-list">
              {#each $history as h, idx (idx)}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class={`flat-list-row ${$selectedIds.includes(h.id) ? 'is-selected' : ''}`}
                  onclick={() => selectedIds.set([h.id])}
                  ondblclick={() => openConnections([h.id], 'tab')}
                >
                  <img
                    src={getIcon(`asbru-method-${h.method?.toLowerCase() || 'ssh'}`)}
                    alt=""
                    width="16"
                    height="16"
                  />
                  <span class="flat-row-name">{h.name}</span>
                  <span class="flat-row-time">{h.timestamp}</span>
                </div>
              {/each}
              {#if $history.length === 0}
                <div class="empty-hint">No connection history yet</div>
              {/if}
            </div>
          {:else if tabId === 'clusters'}
            <div class="clusters-tab">
              {#if !isCompact}
                <div class="manage-clusters-header">
                  <Button
                    label="Manage Clusters"
                    icon="asbru-cluster-manager2"
                    class="w-full"
                    onclick={() => {}}
                  />
                </div>
              {/if}
              <div class="empty-hint">Clusters Administration</div>
            </div>
          {/if}
        </div>
      {/snippet}
    </Notebook>
  </div>

  <!-- 3. Search area _vboxSearch (hidden by default) -->
  {#if $searchVisible}
    <div class="cp-search-area">
      <div class="search-input-row" onkeydown={handleSearchKeyDown}>
        <Entry
          value={$searchQuery}
          placeholder="Search..."
          onchange={(val) => {
            searchQuery.set(val);
            performSearch();
          }}
        />
        <button
          type="button"
          class="button-cp"
          title="Previous match"
          onclick={() => performSearch(true)}
        >
          <img src={getIcon('gtk-media-previous')} alt="Prev" width="16" height="16" />
        </button>
        <button
          type="button"
          class="button-cp"
          title="Next match"
          onclick={() => performSearch(false)}
        >
          <img src={getIcon('gtk-media-next')} alt="Next" width="16" height="16" />
        </button>
        <button
          type="button"
          class="button-cp"
          title="Close search"
          onclick={() => searchVisible.set(false)}
        >
          <img src={getIcon('gtk-close')} alt="Close" width="16" height="16" />
        </button>
      </div>
      <div class="search-radios-row">
        <RadioGroup
          name="searchField"
          options={[
            { value: 'name', label: 'Name' },
            { value: 'host', label: 'IP / Host' },
            { value: 'desc', label: 'Description' },
          ]}
          value={$searchField}
          onchange={(val) => searchField.set(val as SearchField)}
        />
      </div>
    </div>
  {/if}

  <!-- 4. Row hboxsearchstart -->
  <div class="cp-searchstart-row">
    <button
      type="button"
      class={`button-cp ${$searchVisible ? 'is-active' : ''}`}
      title="Start interactive search for connections"
      onclick={() => searchVisible.update((v) => !v)}
    >
      <img src={getIcon('gtk-find')} alt="Search" width="16" height="16" />
    </button>
    <Button
      label="Connect"
      icon="gtk-connect"
      class="expand-btn connect-btn"
      disabled={!canConnect}
      onclick={handleConnect}
    />
    <button
      type="button"
      class="button-cp"
      title="Start a new connection, without saving it"
      onclick={handleQuickConnect}
    >
      <img src={getIcon('asbru-quick-connect')} alt="Quick Connect" width="16" height="16" />
    </button>
    <button
      type="button"
      class="button-cp"
      title="Add to/remove from favourites connections list"
      disabled={$selectedIds.length !== 1 || !singleSelectedNode || singleSelectedNode.isGroup}
      onclick={handleToggleFavourite}
    >
      <img
        src={getIcon(isFavouriteActive ? 'asbru-favourite-on' : 'asbru-favourite-off')}
        alt="Favourite"
        width="16"
        height="16"
      />
    </button>
  </div>

  <!-- 5. Row hboxclusters -->
  {#if $clustersVisible || isCompact}
    <div class="cp-clusters-row">
      {#if $clustersVisible}
        <Button
          label={isCompact ? '' : 'Clusters'}
          icon="asbru-cluster-manager2"
          tooltip="Open the Clusters Administration Console"
          class="cluster-action-btn"
          onclick={() => clusterAdminDialogOpen.set(true)}
        />
        <Button
          label={isCompact ? '' : 'PCC'}
          icon="gtk-justify-fill"
          tooltip="Open the Power Clusters Controller: execute commands in every clustered terminal from this single window"
          class="cluster-action-btn"
          onclick={() => pccDialogOpen.set(true)}
        />
      {/if}
      {#if isCompact}
        <button
          type="button"
          class="button-cp"
          title="Launch new local shell"
          onclick={() => openLocalShell('tab')}
        >
          <img src={getIcon('asbru-shell')} alt="Local Shell" width="16" height="16" />
        </button>
        <button
          type="button"
          class="button-cp"
          title="Open general preferences"
          onclick={() => prefsDialogOpen.set(true)}
        >
          <img src={getIcon('gtk-preferences')} alt="Prefs" width="16" height="16" />
        </button>
        <button
          type="button"
          class="button-cp"
          title="Quit"
          onclick={requestQuit}
        >
          <img src={getIcon('gtk-quit')} alt="Quit" width="16" height="16" />
        </button>
      {/if}
    </div>
  {/if}
</aside>

<style>
  .command-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
    min-width: 200px;
    background-color: var(--gtk-base-color, #ffffff);
    border-right: 1px solid var(--gtk-border-color, #ccc);
    user-select: none;
    overflow: hidden;
  }

  .cp-toolbar {
    display: flex;
    align-items: center;
    background: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color, #ccc);
    padding: 2px 4px;
    gap: 2px;
  }

  .button-cp {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 3px 5px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .button-cp:hover:not(:disabled) {
    background-color: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }
  .button-cp:active:not(:disabled),
  .button-cp.is-active {
    background-color: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }
  .button-cp:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .cp-tree-notebook {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .tab-page-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .flat-list {
    flex: 1;
    overflow-y: auto;
    background: var(--gtk-base-color);
    font-size: 12px;
  }
  .flat-list-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 6px;
    cursor: pointer;
    border-bottom: 1px solid var(--gtk-border-color);
  }
  .flat-list-row:hover:not(.is-selected) {
    background: var(--gtk-header-bg);
  }
  .flat-list-row.is-selected {
    background: var(--gtk-selected-bg, #4a90d9);
    color: #fff;
  }
  .flat-row-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .flat-row-time {
    font-size: 10px;
    opacity: 0.7;
  }
  .fav-toggle-btn {
    background: transparent;
    border: none;
    color: #f5a623;
    cursor: pointer;
    font-size: 14px;
    padding: 0 4px;
  }
  .empty-hint {
    padding: 16px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-size: 12px;
  }

  .clusters-tab {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .manage-clusters-header {
    padding: 4px;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .cp-search-area {
    background: var(--gtk-base-color);
    border-top: 1px solid var(--gtk-border-color);
    border-bottom: 1px solid var(--gtk-border-color);
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .search-input-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .search-radios-row {
    font-size: 11px;
    margin-top: 2px;
  }

  .cp-searchstart-row {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px;
    background: var(--gtk-bg-color);
    border-top: 1px solid var(--gtk-border-color);
  }
  :global(.expand-btn) {
    flex: 1;
  }

  .cp-clusters-row {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px;
    background: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
  }
  :global(.cluster-action-btn) {
    flex: 1;
  }
</style>
