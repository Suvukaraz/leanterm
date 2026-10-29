<script lang="ts">
  import { fontFaceCss } from '../../lib/fonts';
  import {
    tree,
    selectedIds,
    settings,
    clipboard,
    findNode,
    moveNodes,
    setProtected,
    copyNodes,
    cutNodes,
    pasteNodes,
    cloneNode,
  } from '../../lib/stores/config';
  import {
    addRenameDialog,
    deleteConfirmDialog,
    editConnDialog,
    keybindingsDialogOpen,
    wolDialog,
    bulkEditDialogOpen,
    editVariablesDialog,
    expandedGroups,
  } from '../../lib/stores/ui';
  import { openConnections } from '../../lib/stores/sessions';
  import TreeView, { type TreeItem } from '../gtk/TreeView.svelte';
  import ContextMenu, { type MenuItem } from '../gtk/ContextMenu.svelte';

  let treeViewRef = $state<any>();

  // Context menu state
  let menuOpen = $state(false);
  let menuX = $state(0);
  let menuY = $state(0);
  let menuItems = $state<MenuItem[]>([]);

  function handleSelect(ids: string[]) {
    selectedIds.set(ids);
  }

  function handleActivate(item: TreeItem) {
    if (!item.isGroup && item.id) {
      openConnections([item.id], 'tab');
    }
  }

  function handleMiddleClick(item: TreeItem) {
    if (!item.isGroup && item.id) {
      openConnections([item.id], 'window');
    }
  }

  async function handleReorder(draggedIds: string[], targetParentId: string) {
    try {
      await moveNodes(draggedIds, targetParentId);
    } catch (err) {
      console.error('Failed to move nodes:', err);
    }
  }

  function getAllChildConnectionIds(node: TreeItem): string[] {
    const ids: string[] = [];
    function traverse(n: TreeItem) {
      if (!n.isGroup && n.id) {
        ids.push(n.id);
      }
      if (n.children) {
        for (const child of n.children) {
          if (child) traverse(child);
        }
      }
    }
    traverse(node);
    return ids;
  }

  // Font for the connection list (empty family = system font, size 0 = default size).
  let treeFontCss = $derived.by(() => {
    const f: any = $settings?.appearance?.TreeFont;
    if (!f?.Family && !(f?.Size > 0)) return '';
    const family = !f?.Family ? '' : /^mono(space)?$/i.test(f.Family) ? 'font-family: monospace;' : `font-family: "${String(f.Family).replace(/"/g, '')}", sans-serif;`;
    const size = f?.Size > 0 ? `--tree-font-size: ${f.Size}pt;` : '';
    return `${family} ${size} ${fontFaceCss(f)};`;
  });

  function handleContextMenu(item: TreeItem, e: MouseEvent) {
    e.preventDefault();
    menuX = e.clientX;
    menuY = e.clientY;

    const currentSelectedIds = $selectedIds.includes(item.id)
      ? $selectedIds
      : [item.id];
    selectedIds.set(currentSelectedIds);

    const selNodes = currentSelectedIds
      .map((id) => findNode(id, $tree))
      .filter(Boolean) as TreeItem[];
    const isSingle = selNodes.length === 1;
    const isRoot = isSingle && selNodes[0].id === '';
    const isGroup = isSingle && selNodes[0].isGroup;
    const isConn = isSingle && !selNodes[0].isGroup;
    const anyProtected = selNodes.some((n) => n.protected);
    const allProtected = selNodes.length > 0 && selNodes.every((n) => n.protected);
    const canPaste = (isRoot || isGroup) && $clipboard.ids.length > 0 && !anyProtected;

    const targetNode = selNodes[0] || item;

    const items: MenuItem[] = [
      {
        label: 'Show Keybindings',
        icon: 'gtk-help',
        action: () => keybindingsDialogOpen.set(true),
      },
      {
        separator: true,
      },
      {
        label: 'Expand all',
        icon: 'gtk-add',
        tooltip: 'Expand ALL (including sub-nodes)',
        action: () => treeViewRef?.expandAll(),
      },
      {
        label: 'Collapse all',
        icon: 'gtk-remove',
        action: () => treeViewRef?.collapseAll(),
      },
      {
        separator: true,
      },
      {
        label: allProtected
          ? 'Un-Protect'
          : anyProtected
            ? 'Toggle Protected state'
            : 'Protect',
        icon: allProtected ? 'asbru-unprotected' : 'asbru-protected',
        disabled: isRoot,
        tooltip: 'Protect or not this node, in order to avoid any changes (Edit, Delete, Rename, ...)',
        action: () => setProtected(currentSelectedIds, !allProtected),
      },
      {
        label: 'Edit connection(s)',
        icon: 'gtk-edit',
        disabled: isRoot,
        tooltip: "Edit this connection's data",
        action: () => {
          if (isConn) {
            editConnDialog.set({ open: true, connId: targetNode.id, isQuick: false });
          }
        },
      },
      {
        label: 'Copy Password',
        icon: 'gtk-copy',
        disabled: !isConn,
        action: () => {
          // Will copy password from secrets vault in phase 1.3/backend
        },
      },
      {
        separator: true,
      },
    ];

    if (isGroup && !isRoot) {
      items.push({
        label: 'Start Group',
        icon: 'asbru-group-connect',
        tooltip: 'Start all connections of this group',
        action: () => {
          const ids = getAllChildConnectionIds(targetNode);
          if (ids.length > 0) openConnections(ids, 'tab');
        },
      });
    }

    // Add connection / group
    items.push(
      {
        label: `Add Connection under '${targetNode.name}'`,
        icon: 'asbru-node-add',
        disabled: (!isRoot && !isGroup) || anyProtected,
        action: () => {
          addRenameDialog.set({
            open: true,
            isRename: false,
            isGroup: false,
            parentId: targetNode.id,
            nodeId: '',
            currentName: '',
            currentTitle: '',
          });
        },
      },
      {
        label: `Add Group under '${targetNode.name}'`,
        icon: 'asbru-group-add',
        disabled: (!isRoot && !isGroup) || anyProtected,
        action: () => {
          addRenameDialog.set({
            open: true,
            isRename: false,
            isGroup: true,
            parentId: targetNode.id,
            nodeId: '',
            currentName: '',
            currentTitle: '',
          });
        },
      },
      {
        separator: true,
      },
      {
        label: 'Rename Group/Connection',
        icon: 'gtk-spell-check',
        disabled: isRoot || anyProtected || !isSingle,
        action: () => {
          addRenameDialog.set({
            open: true,
            isRename: true,
            isGroup: targetNode.isGroup,
            parentId: targetNode.parentId || '',
            nodeId: targetNode.id,
            currentName: targetNode.name,
            currentTitle: targetNode.title || '',
          });
        },
      },
      {
        label: 'Delete...',
        icon: 'gtk-delete',
        disabled: isRoot || anyProtected,
        action: () => {
          deleteConfirmDialog.set({
            open: true,
            nodeIds: currentSelectedIds,
            count: currentSelectedIds.length,
          });
        },
      },
      {
        separator: true,
      },
      {
        label: 'Clone connection',
        icon: 'gtk-copy',
        disabled: !isConn || anyProtected,
        action: () => cloneNode(targetNode.id),
      },
      {
        label: 'Copy node(s)',
        icon: 'gtk-copy',
        disabled: isRoot,
        action: () => copyNodes(currentSelectedIds),
      },
      {
        label: 'Cut node(s)',
        icon: 'gtk-cut',
        disabled: isRoot || anyProtected,
        action: () => cutNodes(currentSelectedIds),
      },
      {
        label: 'Paste node(s)',
        icon: 'gtk-paste',
        disabled: !canPaste,
        action: () => pasteNodes(targetNode.id),
      },
      {
        separator: true,
      },
      {
        label: 'Edit Local Variables...',
        icon: 'gtk-edit',
        disabled: !isConn,
        action: () => {
          editVariablesDialog.set({
            open: true,
            connId: targetNode.id,
          });
        },
      },
      {
        label: 'Bulk Edit...',
        icon: 'gtk-edit',
        disabled: currentSelectedIds.length < 1,
        action: () => {
          bulkEditDialogOpen.set({
            open: true,
            connIds: currentSelectedIds,
          });
        },
      },
      {
        label: 'Wake On LAN...',
        icon: 'asbru-wol',
        disabled: !isConn,
        action: () => {
          wolDialog.set({
            open: true,
            mac: '',
            host: targetNode.host || '',
            port: 9,
          });
        },
      },
      {
        label: 'Start',
        icon: 'gtk-execute',
        disabled: isRoot,
        submenu: [1, 2, 3, 4, 5, 6, 7, 8, 9].map((n) => ({
          label: `${n} instance${n > 1 ? 's' : ''}`,
          action: () => {
            const ids = isGroup
              ? getAllChildConnectionIds(targetNode)
              : [targetNode.id];
            if (ids.length > 0) {
              openConnections(ids, 'tab', '', n);
            }
          },
        })),
      }
    );

    menuItems = items;
    menuOpen = true;
  }
</script>

<div class="connection-tree-container" style={treeFontCss}>
  {#if $tree}
    <TreeView
      bind:this={treeViewRef}
      root={$tree}
      bind:selectedIds={$selectedIds}
      protectedColor={$settings?.appearance?.ProtectedColor || '#ffb022'}
      protectedTarget={($settings?.appearance?.ProtectedColorTarget as any) || 'background'}
      unprotectedColor={$settings?.appearance?.UnprotectedColor || ''}
      unprotectedTarget={($settings?.appearance?.UnprotectedColorTarget as any) || 'foreground'}
      expandedIds={$expandedGroups}
      onexpandchange={(ids) => expandedGroups.set(ids)}
      showTooltips={$settings?.appearance?.ShowTreeTooltips ?? false}
      showLines={$settings?.appearance?.ShowTreeLines ?? false}
      onselect={handleSelect}
      onactivate={handleActivate}
      onmiddleclick={handleMiddleClick}
      oncontextmenu={handleContextMenu}
      onreorder={handleReorder}
    />
  {/if}

  {#if menuOpen}
    <ContextMenu
      items={menuItems}
      x={menuX}
      y={menuY}
      onclose={() => (menuOpen = false)}
    />
  {/if}
</div>

<style>
  .connection-tree-container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
</style>
