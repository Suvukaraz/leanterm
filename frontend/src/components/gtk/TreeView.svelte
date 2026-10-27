<script lang="ts">
  import { getIcon } from '../../lib/theme/icons';

  export interface TreeItem {
    id: string;
    parentId?: string;
    name: string;
    title?: string;
    isGroup: boolean;
    method?: string;
    host?: string;
    port?: number;
    description?: string;
    protected?: boolean;
    favourite?: boolean;
    children?: (TreeItem | null)[];
    expanded?: boolean;
  }

  interface Props {
    root: TreeItem;
    selectedIds?: string[];
    protectedColor?: string;
    protectedTarget?: 'foreground' | 'background';
    unprotectedColor?: string;
    unprotectedTarget?: 'foreground' | 'background';
    showTooltips?: boolean;
    showLines?: boolean;
    class?: string;
    onselect?: (ids: string[]) => void;
    onactivate?: (item: TreeItem) => void; // double-click or Enter on connection
    onmiddleclick?: (item: TreeItem) => void;
    oncontextmenu?: (item: TreeItem, e: MouseEvent) => void;
    onreorder?: (draggedIds: string[], targetParentId: string) => void;
    /** Groups expanded initially (e.g. restored from the last session). */
    expandedIds?: string[];
    onexpandchange?: (expandedIds: string[]) => void;
  }

  let {
    root,
    selectedIds = $bindable([]),
    protectedColor = '#ffb022',
    protectedTarget = 'background',
    unprotectedColor = '',
    unprotectedTarget = 'foreground',
    showTooltips = false,
    showLines = false,
    class: className = '',
    onselect,
    onactivate,
    onmiddleclick,
    oncontextmenu,
    onreorder,
    expandedIds = [],
    onexpandchange,
  }: Props = $props();

  let expandedMap = $state<Record<string, boolean>>(
    Object.fromEntries(expandedIds.map((id) => [id, true]))
  );

  function notifyExpanded() {
    onexpandchange?.(Object.keys(expandedMap).filter((id) => id !== '' && expandedMap[id]));
  }

  function setExpanded(id: string, on: boolean) {
    if (!!expandedMap[id] === on) return;
    expandedMap[id] = on;
    notifyExpanded();
  }

  // Visible rows in display order (for arrow-key navigation).
  function visibleItems(): TreeItem[] {
    const out: TreeItem[] = [];
    const walk = (node: TreeItem) => {
      out.push(node);
      if (node.isGroup && expandedMap[node.id] && node.children) {
        for (const c of node.children) if (c) walk(c);
      }
    };
    if (root) walk(root);
    return out;
  }

  let containerEl = $state<HTMLElement>();

  function selectAndReveal(item: TreeItem) {
    selectedIds = [item.id];
    onselect?.(selectedIds);
    queueMicrotask(() => {
      const row = containerEl?.querySelector<HTMLElement>(`[data-node-id="${CSS.escape(item.id)}"]`);
      row?.scrollIntoView({ block: 'nearest' });
    });
  }

  // Ensure root is expanded initially
  $effect(() => {
    if (root && root.id !== undefined && expandedMap[root.id] === undefined) {
      expandedMap[root.id] = true;
    }
  });

  function toggleExpand(id: string, e?: MouseEvent) {
    if (e) e.stopPropagation();
    setExpanded(id, !expandedMap[id]);
  }

  function handleRowClick(item: TreeItem, e: MouseEvent) {
    if (e.ctrlKey || e.metaKey) {
      // Toggle in multi-select
      if (selectedIds.includes(item.id)) {
        selectedIds = selectedIds.filter((id) => id !== item.id);
      } else {
        selectedIds = [...selectedIds, item.id];
      }
    } else if (e.shiftKey && selectedIds.length > 0) {
      // Range select (simplified: add to selection)
      if (!selectedIds.includes(item.id)) {
        selectedIds = [...selectedIds, item.id];
      }
    } else {
      selectedIds = [item.id];
    }
    if (onselect) onselect(selectedIds);
  }

  function handleDblClick(item: TreeItem) {
    if (item.isGroup) {
      toggleExpand(item.id);
    } else {
      if (onactivate) onactivate(item);
    }
  }

  function handleAuxClick(item: TreeItem, e: MouseEvent) {
    if (e.button === 1 && !item.isGroup && onmiddleclick) {
      e.preventDefault();
      onmiddleclick(item);
    }
  }

  function handleContextMenu(item: TreeItem, e: MouseEvent) {
    e.preventDefault();
    if (!selectedIds.includes(item.id)) {
      selectedIds = [item.id];
      if (onselect) onselect(selectedIds);
    }
    if (oncontextmenu) oncontextmenu(item, e);
  }

  function getItemIcon(item: TreeItem): string {
    if (item.id === '') {
      return getIcon('asbru-group');
    }
    if (item.isGroup) {
      return expandedMap[item.id]
        ? getIcon('asbru-group-open')
        : getIcon('asbru-group-closed');
    }
    // Method icon
    const m = item.method?.toLowerCase() || 'ssh';
    return getIcon(`asbru-method-${m}`);
  }

  function getNodeStyle(item: TreeItem): string {
    if (item.protected) {
      if (protectedTarget === 'foreground') {
        return `color: ${protectedColor}; font-weight: ${item.isGroup ? 'bold' : 'normal'};`;
      } else {
        return `background-color: ${protectedColor}; color: #000; font-weight: ${item.isGroup ? 'bold' : 'normal'}; border-radius: 2px; padding: 0 3px;`;
      }
    }
    if (unprotectedColor) {
      if (unprotectedTarget === 'foreground') {
        return `color: ${unprotectedColor}; font-weight: ${item.isGroup ? 'bold' : 'normal'};`;
      } else {
        return `background-color: ${unprotectedColor}; font-weight: ${item.isGroup ? 'bold' : 'normal'}; border-radius: 2px; padding: 0 3px;`;
      }
    }
    return item.isGroup ? 'font-weight: bold;' : '';
  }

  function getTooltipText(item: TreeItem): string {
    if (!showTooltips) return '';
    if (item.isGroup) return item.name;
    let parts = [item.name];
    if (item.method) parts.push(`Method: ${item.method}`);
    if (item.host) parts.push(`Host: ${item.host}${item.port ? `:${item.port}` : ''}`);
    if (item.description) parts.push(item.description);
    return parts.join('\n');
  }

  function findItemById(parent: TreeItem, id: string): TreeItem | null {
    if (parent.id === id) return parent;
    if (parent.children) {
      for (const child of parent.children) {
        if (!child) continue;
        const found = findItemById(child, id);
        if (found) return found;
      }
    }
    return null;
  }

  // Keyboard navigation
  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      if (selectedIds.length === 1) {
        const item = findItemById(root, selectedIds[0]);
        if (item) handleDblClick(item);
      }
    } else if (e.key === 'ArrowRight') {
      if (selectedIds.length === 1) {
        e.preventDefault();
        setExpanded(selectedIds[0], true);
      }
    } else if (e.key === 'ArrowLeft') {
      if (selectedIds.length === 1) {
        e.preventDefault();
        const item = findItemById(root, selectedIds[0]);
        if (item?.isGroup && expandedMap[item.id] && item.id !== '') {
          setExpanded(item.id, false);
        } else if (item?.parentId !== undefined) {
          const parent = findItemById(root, item.parentId ?? '');
          if (parent) selectAndReveal(parent);
        }
      }
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Home' || e.key === 'End') {
      e.preventDefault();
      const items = visibleItems();
      if (items.length === 0) return;
      const cur = items.findIndex((i) => i.id === selectedIds[selectedIds.length - 1]);
      let next = 0;
      if (e.key === 'Home') next = 0;
      else if (e.key === 'End') next = items.length - 1;
      else if (cur < 0) next = 0;
      else next = Math.max(0, Math.min(items.length - 1, cur + (e.key === 'ArrowDown' ? 1 : -1)));
      selectAndReveal(items[next]);
    }
  }

  let dragOverId = $state<string | null>(null);

  export function expandAll() {
    function traverse(node: TreeItem) {
      if (node.isGroup) {
        expandedMap[node.id] = true;
      }
      if (node.children) {
        for (const child of node.children) {
          if (child) traverse(child);
        }
      }
    }
    if (root) traverse(root);
    notifyExpanded();
  }

  export function collapseAll() {
    expandedMap = {};
    if (root) expandedMap[root.id] = true;
    notifyExpanded();
  }

  function handleDragStart(item: TreeItem, e: DragEvent) {
    if (item.id === '' || item.protected) {
      e.preventDefault();
      return;
    }
    const idsToDrag = selectedIds.includes(item.id) ? selectedIds : [item.id];
    if (e.dataTransfer) {
      e.dataTransfer.setData('application/json', JSON.stringify({ ids: idsToDrag }));
      e.dataTransfer.setData('text/plain', item.id);
      e.dataTransfer.setData('application/x-leanterm-conn-id', item.id);
      e.dataTransfer.effectAllowed = 'copyMove';
    }
  }

  function handleDragOver(item: TreeItem, e: DragEvent) {
    if (!item.isGroup && item.id !== '') return;
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'move';
    }
    dragOverId = item.id;
  }

  function handleDragLeave() {
    dragOverId = null;
  }

  function handleDrop(item: TreeItem, e: DragEvent) {
    e.preventDefault();
    dragOverId = null;
    if (!item.isGroup && item.id !== '') return;
    const raw = e.dataTransfer?.getData('application/json');
    if (!raw) return;
    try {
      const data = JSON.parse(raw);
      if (data && Array.isArray(data.ids) && onreorder) {
        onreorder(data.ids, item.id);
      }
    } catch {
      // Ignore invalid drag data
    }
  }
</script>

<!-- Recursive tree node snippet -->
{#snippet renderNode(item: TreeItem, depth: number)}
  {@const isSelected = selectedIds.includes(item.id)}
  {@const isExpanded = expandedMap[item.id] ?? false}
  {@const hasChildren = item.children && item.children.length > 0}

  <div
    class={`tree-row ${isSelected ? 'is-selected' : ''} ${dragOverId === item.id ? 'drag-over' : ''}`}
    style={`padding-left: ${depth * 16 + 4}px;`}
    role="treeitem"
    data-node-id={item.id}
    aria-selected={isSelected}
    aria-expanded={item.isGroup ? isExpanded : undefined}
    title={getTooltipText(item)}
    draggable={item.id !== ''}
    ondragstart={(e) => handleDragStart(item, e)}
    ondragover={(e) => handleDragOver(item, e)}
    ondragleave={handleDragLeave}
    ondrop={(e) => handleDrop(item, e)}
    onclick={(e) => handleRowClick(item, e)}
    ondblclick={() => handleDblClick(item)}
    onauxclick={(e) => handleAuxClick(item, e)}
    oncontextmenu={(e) => handleContextMenu(item, e)}
  >
    <!-- Expander arrow -->
    <span
      class="tree-expander"
      onclick={(e) => item.isGroup && toggleExpand(item.id, e)}
    >
      {#if item.isGroup}
        <span class={`expander-arrow ${isExpanded ? 'open' : ''}`}>▶</span>
      {:else}
        <span class="expander-spacer"></span>
      {/if}
    </span>

    <!-- Icon -->
    <img src={getItemIcon(item)} alt="" width="16" height="16" class="tree-icon" />

    <!-- Label -->
    <span class="tree-name" style={getNodeStyle(item)}>
      {item.name}
    </span>

    {#if item.favourite}
      <span class="fav-indicator" title="Favourite">★</span>
    {/if}
  </div>

  {#if item.isGroup && isExpanded && item.children}
    {#each (item.children || []).filter((c): c is TreeItem => c !== null) as child (child.id)}
      {@render renderNode(child, depth + 1)}
    {/each}
  {/if}
{/snippet}

<div
  bind:this={containerEl}
  class={`gtk-treeview ${showLines ? 'show-lines' : ''} ${className}`}
  role="tree"
  tabindex="0"
  onkeydown={handleKeyDown}
>
  {@render renderNode(root, 0)}
</div>

<style>
  .gtk-treeview {
    width: 100%;
    height: 100%;
    overflow-y: auto;
    overflow-x: auto;
    background-color: var(--gtk-base-color);
    color: var(--gtk-text-color);
    border: 1px solid var(--gtk-border-color);
    box-sizing: border-box;
    font-size: 0.9em;
    user-select: none;
    outline: none;
  }

  .tree-row {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 22px;
    cursor: pointer;
    white-space: nowrap;
    border: 1px solid transparent;
  }

  .tree-row:hover:not(.is-selected) {
    background-color: var(--gtk-row-hover);
  }

  .tree-row.is-selected {
    background-color: var(--gtk-selected-bg);
    color: var(--gtk-selected-fg);
  }

  .tree-row.drag-over {
    outline: 2px dashed var(--gtk-selected-bg, #4a90d9);
    background-color: rgba(74, 144, 217, 0.15);
  }

  .tree-expander {
    width: 14px;
    height: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    flex-shrink: 0;
  }

  .expander-arrow {
    font-size: 0.7em;
    color: var(--gtk-muted-fg);
    transition: transform 0.15s ease-out;
    display: inline-block;
  }
  .expander-arrow.open {
    transform: rotate(90deg);
  }
  .is-selected .expander-arrow {
    color: #fff;
  }

  .expander-spacer {
    width: 14px;
  }

  .tree-icon {
    flex-shrink: 0;
    pointer-events: none;
    object-fit: contain;
  }

  .tree-name {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .fav-indicator {
    color: #f5a623;
    font-size: 0.9em;
    margin-left: auto;
    padding-right: 6px;
  }

  .show-lines .tree-row {
    border-bottom: 1px dotted var(--gtk-border-color);
  }
</style>
