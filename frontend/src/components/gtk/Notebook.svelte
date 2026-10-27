<script lang="ts">
  import type { Snippet } from 'svelte';
  import { getIcon } from '../../lib/theme/icons';

  export interface TabItem {
    id: string;
    label: string;
    icon?: string;
    theme?: string;
    closable?: boolean;
    color?: string; // status color
    bgColor?: string; // custom tab background color
    tooltip?: string;
  }
  export type NotebookTab = TabItem;

  interface Props {
    tabs: TabItem[];
    activeTab?: string;
    position?: 'top' | 'bottom' | 'left' | 'right';
    class?: string;
    children?: Snippet<[string]>;
    onchange?: (tabId: string) => void;
    onclose?: (tabId: string) => void;
    onmiddleclick?: (tabId: string) => void;
    oncontextmenu?: (tabId: string, e: MouseEvent) => void;
    ondblclick?: (tabId: string) => void;
  }

  let {
    tabs = [],
    activeTab = $bindable(''),
    position = 'top',
    class: className = '',
    children,
    onchange,
    onclose,
    onmiddleclick,
    oncontextmenu,
    ondblclick
  }: Props = $props();

  // If no active tab, default to first tab
  $effect(() => {
    if ((!activeTab || !tabs.some(t => t.id === activeTab)) && tabs.length > 0) {
      activeTab = tabs[0].id;
    }
  });

  function selectTab(id: string) {
    activeTab = id;
    if (onchange) onchange(id);
  }

  function handleAuxClick(id: string, e: MouseEvent) {
    if (e.button === 1 && onmiddleclick) {
      e.preventDefault();
      onmiddleclick(id);
    }
  }

  function handleContextMenu(id: string, e: MouseEvent) {
    if (oncontextmenu) {
      e.preventDefault();
      oncontextmenu(id, e);
    }
  }
</script>

<div class={`gtk-notebook pos-${position} ${className}`}>
  <div class="notebook-header" role="tablist">
    {#each tabs as tab}
      <button
        type="button"
        role="tab"
        aria-selected={activeTab === tab.id}
        class={`notebook-tab ${activeTab === tab.id ? 'active' : ''}`}
        style={tab.bgColor ? `background-color: ${tab.bgColor};` : ''}
        title={tab.tooltip || tab.label}
        onclick={() => selectTab(tab.id)}
        onauxclick={(e) => handleAuxClick(tab.id, e)}
        oncontextmenu={(e) => handleContextMenu(tab.id, e)}
        ondblclick={() => ondblclick && ondblclick(tab.id)}
      >
        {#if tab.icon}
          <img
            src={getIcon(tab.icon, tab.theme || 'default')}
            alt=""
            width="16"
            height="16"
            class="tab-icon"
          />
        {/if}
        <span
          class="tab-label"
          style={tab.color ? `color: ${tab.color}; font-weight: bold;` : ''}
        >
          {tab.label}
        </span>
        {#if tab.closable}
          <span
            class="tab-close"
            role="button"
            tabindex="0"
            title="Close Tab"
            onclick={(e) => {
              e.stopPropagation();
              if (onclose) onclose(tab.id);
            }}
            onkeydown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.stopPropagation();
                if (onclose) onclose(tab.id);
              }
            }}
          >
            ×
          </span>
        {/if}
      </button>
    {/each}
  </div>

  <div class="notebook-content">
    {@render children?.(activeTab)}
  </div>
</div>

<style>
  .gtk-notebook {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    overflow: hidden;
    background-color: var(--gtk-bg-color);
  }
  .pos-bottom {
    flex-direction: column-reverse;
  }
  .pos-left {
    flex-direction: row;
  }
  .pos-right {
    flex-direction: row-reverse;
  }

  .notebook-header {
    display: flex;
    flex-shrink: 0;
    background-color: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color);
    overflow-x: auto;
    overflow-y: hidden;
    user-select: none;
  }
  .pos-bottom .notebook-header {
    border-bottom: none;
    border-top: 1px solid var(--gtk-border-color);
  }
  .pos-left .notebook-header,
  .pos-right .notebook-header {
    flex-direction: column;
    overflow-y: auto;
    overflow-x: hidden;
    border-bottom: none;
    border-right: 1px solid var(--gtk-border-color);
  }
  .pos-right .notebook-header {
    border-right: none;
    border-left: 1px solid var(--gtk-border-color);
  }

  .notebook-tab {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    margin: 0;
    background: transparent;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 0.9em;
    color: var(--gtk-fg-color);
    border-right: 1px solid var(--gtk-border-color);
    position: relative;
    white-space: nowrap;
  }
  .notebook-tab:hover:not(.active) {
    background-color: var(--gtk-row-hover);
  }
  .notebook-tab.active {
    background-color: var(--gtk-bg-color);
    color: var(--gtk-fg-color);
    font-weight: 500;
  }
  .pos-top .notebook-tab.active {
    box-shadow: 0 2px 0 var(--gtk-selected-bg) inset;
  }
  .pos-bottom .notebook-tab.active {
    box-shadow: 0 -2px 0 var(--gtk-selected-bg) inset;
  }
  .pos-left .notebook-tab.active {
    box-shadow: 2px 0 0 var(--gtk-selected-bg) inset;
  }
  .pos-right .notebook-tab.active {
    box-shadow: -2px 0 0 var(--gtk-selected-bg) inset;
  }

  .tab-icon {
    flex-shrink: 0;
    object-fit: contain;
  }
  .tab-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tab-close {
    font-size: 1.2em;
    line-height: 0.8;
    color: var(--gtk-muted-fg);
    padding: 1px 3px;
    border-radius: 50%;
    margin-left: 2px;
  }
  .tab-close:hover {
    color: var(--gtk-fg-color);
    background-color: var(--gtk-row-hover);
  }

  .notebook-content {
    flex: 1;
    overflow: hidden;
    position: relative;
    display: flex;
    flex-direction: column;
  }
</style>
