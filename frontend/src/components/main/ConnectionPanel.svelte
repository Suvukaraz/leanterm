<script lang="ts">
  import { fontFaceCss } from '../../lib/fonts';
  import {
    tree,
    selectedIds,
    settings,
    findNode,
    setDescription,
  } from '../../lib/stores/config';
  import {
    sessions,
    activeTab,
    newDataTabs,
    closeSession,
    setActiveTab,
  } from '../../lib/stores/sessions';
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Button from '../gtk/Button.svelte';
  import TerminalTab from '../terminal/TerminalTab.svelte';
  import SplitTab from '../terminal/SplitTab.svelte';
  import TabContextMenu from '../terminal/TabContextMenu.svelte';
  import { splits, isSecondaryInSplit } from '../../lib/stores/splits';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import { getIcon } from '../../lib/theme/icons';

  let selectedNode = $derived.by(() => {
    if ($selectedIds.length === 1 && $selectedIds[0] !== '') {
      return findNode($selectedIds[0], $tree);
    }
    return null;
  });

  let isRootOrEmpty = $derived(
    $selectedIds.length === 0 || ($selectedIds.length === 1 && $selectedIds[0] === '')
  );

  let descText = $state('');
  let currentDescNodeId = $state('');
  let descTimer: any;

  // Sync description when selected node changes
  $effect(() => {
    if (selectedNode) {
      if (currentDescNodeId !== selectedNode.id) {
        currentDescNodeId = selectedNode.id;
        descText = selectedNode.description || '';
      }
    } else {
      currentDescNodeId = '';
      descText = '';
    }
  });

  function handleDescInput(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    descText = target.value;
    clearTimeout(descTimer);
    descTimer = setTimeout(() => {
      if (selectedNode && selectedNode.id) {
        setDescription(selectedNode.id, descText);
      }
    }, 500);
  }

  function handleDescBlur() {
    clearTimeout(descTimer);
    if (selectedNode && selectedNode.id) {
      setDescription(selectedNode.id, descText);
    }
  }

  let tabsPosition = $derived.by<'top' | 'bottom' | 'left' | 'right'>(() => {
    const pos = $settings?.terminal?.TabsPosition;
    if (pos === 'bottom' || pos === 'left' || pos === 'right') return pos;
    return 'top';
  });

  let showStatistics = $derived(
    $settings?.appearance?.ShowStatistics ?? true
  );

  let showScreenshots = $derived(
    $settings?.appearance?.ShowScreenshots ?? false
  );

  let infoFontCss = $derived.by(() => {
    const f: any = $settings?.appearance?.InfoFont;
    const family = f?.Family && !/^mono(space)?$/i.test(f.Family) ? `"${f.Family}", monospace` : 'monospace';
    const size = f?.Size > 0 ? `${f.Size}pt` : '10pt';
    return `font-family: ${family}; font-size: ${size}; ${fontFaceCss(f)};`;
  });

  let connStats = $state({
    totalConnections: 0,
    totalSeconds: 0,
    firstStart: '',
    lastStart: '',
    lastStop: '',
  });

  $effect(() => {
    if (selectedNode && selectedNode.id && !selectedNode.isGroup) {
      SessionService.GetStats(selectedNode.id).then((st: any) => {
        if (st) {
          connStats = {
            totalConnections: st.totalConnections || 0,
            totalSeconds: st.totalSeconds || 0,
            firstStart: st.firstStart || '',
            lastStart: st.lastStart || '',
            lastStop: st.lastStop || '',
          };
        }
      }).catch(() => {});
    } else {
      connStats = {
        totalConnections: 0,
        totalSeconds: 0,
        firstStart: '',
        lastStart: '',
        lastStop: '',
      };
    }
  });

  function formatDuration(totalSeconds: number): string {
    const pad = (n: number) => (n < 10 ? '0' + n : n.toString());
    const h = Math.floor(totalSeconds / 3600);
    const m = Math.floor((totalSeconds % 3600) / 60);
    const s = totalSeconds % 60;
    return `${pad(h)}:${pad(m)}:${pad(s)}`;
  }

  function formatLastConn(dateStr: string): string {
    if (!dateStr || dateStr.startsWith('0001')) return 'Never';
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return 'Never';
    return d.toLocaleString();
  }

  async function handleResetStats() {
    if (selectedNode && selectedNode.id) {
      await SessionService.ResetStats(selectedNode.id);
      connStats = {
        totalConnections: 0,
        totalSeconds: 0,
        firstStart: '',
        lastStart: '',
        lastStop: '',
      };
    }
  }

  // Context menu state for tabs
  let tabContextMenuOpen = $state(false);
  let tabContextMenuX = $state(0);
  let tabContextMenuY = $state(0);
  let tabContextMenuId = $state('');

  function handleTabContextMenu(tabId: string, e: MouseEvent) {
    if (tabId === 'info') return;
    tabContextMenuId = tabId;
    tabContextMenuX = e.clientX;
    tabContextMenuY = e.clientY;
    tabContextMenuOpen = true;
  }

  let sessionTabIds = $derived(
    $sessions.filter((s) => !isSecondaryInSplit(s.tmpID, $splits)).map((s) => s.tmpID)
  );

  // Tab list: Info tab + active terminal sessions
  let tabs = $derived.by<TabItem[]>(() => {
    const list: TabItem[] = [
      {
        id: 'info',
        label: 'Info ',
        icon: 'gtk-info',
        closable: false,
        tooltip: 'Connection Information and Notes',
      },
    ];

    for (const s of $sessions) {
      // If this session is the secondary inside an active split, it is embedded in the primary tab
      if (isSecondaryInSplit(s.tmpID, $splits)) {
        continue;
      }

      let icon = 'asbru-method-ssh';
      if (s.kind === 'telnet') icon = 'asbru-method-telnet';
      else if (s.kind === 'local') icon = 'asbru-shell';
      else if (s.kind === 'sftp') icon = 'asbru-method-sftp';

      const isSplit = s.tmpID in $splits;
      const label = isSplit ? `${s.title || 'Terminal'} [Split]` : (s.title || 'Terminal');

      const colors: any = $settings?.terminal?.Colors ?? {};
      let color = '';
      if ($newDataTabs.has(s.tmpID) && s.state === 'connected') {
        color = colors.NewData || '#0088ff';
      } else if (s.state === 'connected') {
        color = colors.Connected || '#0cba00';
      } else if (s.state === 'disconnected' || s.state === 'closed') {
        color = colors.Disconnected || '#ff0000';
      }

      list.push({
        id: s.tmpID,
        label,
        icon,
        closable: true,
        color,
        tooltip: `${s.title} (${s.kind}) - ${s.state}${isSplit ? ' (Split View)' : ''}`,
      });
    }

    return list;
  });
</script>

<div class="connection-panel-container">
  <Notebook
    {tabs}
    activeTab={$activeTab}
    position={tabsPosition}
    onchange={(id) => setActiveTab(id)}
    onclose={(id) => closeSession(id)}
    oncontextmenu={handleTabContextMenu}
  >
    {#snippet children(tabId: string)}
      {#if tabId === 'info'}
        <div class="info-page">
          {#if isRootOrEmpty}
            <div class="welcome-box">
              <h1 class="welcome-title">Welcome to LeanTerm</h1>
              <p class="welcome-subtitle">LeanTerm Connection Manager is ready for your sessions.</p>
              <div class="welcome-body">
                <img class="welcome-logo" src={getIcon('asbru-app-big')} alt="LeanTerm logo" />
                <ul class="welcome-list">
                  <li>Select or create a connection in the left tree and click Connect or double-click to start.</li>
                  <li>Use Local shell to open an instant terminal.</li>
                  <li>Click Preferences to customize colors, keys, fonts, and security settings.</li>
                </ul>
              </div>
            </div>
          {:else if selectedNode}
            <div class="node-info-view">
              <div class="node-header">
                <img
                  src={getIcon(
                    selectedNode.isGroup
                      ? 'asbru-group'
                      : `asbru-method-${selectedNode.method?.toLowerCase() || 'ssh'}`
                  )}
                  alt=""
                  width="20"
                  height="20"
                />
                <span class="node-name"><b>{selectedNode.name}</b></span>
                {#if selectedNode.title}
                  <span class="node-title-sub">({selectedNode.title})</span>
                {/if}
              </div>

              <!-- Editable description -->
              <div class="desc-container">
                <label for="connection-notes-textarea" class="desc-label"><b>Notes & Description:</b></label>
                <textarea
                  id="connection-notes-textarea"
                  class="desc-textarea"
                  style={infoFontCss}
                  placeholder="Insert your comments for this connection here ..."
                  value={descText}
                  oninput={handleDescInput}
                  onblur={handleDescBlur}
                ></textarea>
              </div>

              <!-- Statistics Frame -->
              {#if showStatistics}
                <Frame label="STATISTICS " class="stats-frame">
                  <div class="stats-grid">
                    <span class="stat-name">Total connections:</span>
                    <span class="stat-value">{connStats.totalConnections}</span>
                    <span class="stat-name">Total connected time:</span>
                    <span class="stat-value">{formatDuration(connStats.totalSeconds)}</span>
                    <span class="stat-name">Last connection:</span>
                    <span class="stat-value">{formatLastConn(connStats.lastStart)}</span>
                  </div>
                  <div class="stats-actions">
                    <Button label="Reset Statistics" icon="gtk-undo" onclick={handleResetStats} />
                  </div>
                </Frame>
              {/if}

              <!-- Screenshots Frame -->
              {#if showScreenshots}
                <Frame label="SCREENSHOTS " class="screenshots-frame">
                  {#if ((selectedNode as any)?.screenshots || []).length > 0}
                    <div class="screenshots-grid">
                      {#each (selectedNode as any)?.screenshots || [] as shot}
                        <div class="shot-thumb">
                          <img src={`file://${shot}`} alt="Screenshot" />
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="screenshots-empty">
                      <span>No screenshots saved yet. Use 'Take Screenshot' in terminal context menu.</span>
                    </div>
                  {/if}
                  <div class="screenshots-actions">
                    <Button
                      label="Open Screenshots Folder"
                      icon="gtk-open"
                      onclick={() => SystemService.OpenPath('screenshots')}
                    />
                  </div>
                </Frame>
              {/if}
            </div>
          {/if}
        </div>
      {/if}
      <!-- Terminal tabs stay mounted while hidden: switching tabs must not
           recreate xterm.js, the SFTP browser state or replay the output. -->
      {#each sessionTabIds as id (id)}
        <div class="session-page" class:hidden-page={id !== tabId}>
          {#if id in $splits}
            <SplitTab split={$splits[id]} active={id === tabId} />
          {:else}
            <TerminalTab tmpID={id} active={id === tabId} />
          {/if}
        </div>
      {/each}
    {/snippet}
  </Notebook>

  <TabContextMenu
    open={tabContextMenuOpen}
    x={tabContextMenuX}
    y={tabContextMenuY}
    tabId={tabContextMenuId}
    oncloseMenu={() => (tabContextMenuOpen = false)}
  />
</div>

<style>
  .session-page {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
  }
  .session-page.hidden-page {
    visibility: hidden;
    pointer-events: none;
    z-index: -1;
  }
  .connection-panel-container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--gtk-base-color);
  }

  .info-page {
    width: 100%;
    height: 100%;
    padding: 12px;
    box-sizing: border-box;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .welcome-box {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    margin: 8px;
    padding: 32px 24px;
    background: var(--gtk-base-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 4px;
    font-family: var(--gtk-mono-font);
    color: var(--gtk-fg-color);
    text-align: center;
  }
  .welcome-title {
    margin: 0;
    font-size: clamp(28px, 5vw, 56px);
    font-weight: bold;
    line-height: 1.1;
  }
  .welcome-subtitle {
    margin: 0 0 24px 0;
    font-size: clamp(14px, 1.7vw, 20px);
    font-weight: bold;
  }
  .welcome-body {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: center;
    gap: 16px 40px;
    max-width: 900px;
  }
  .welcome-logo {
    width: clamp(120px, 18vw, 200px);
    height: auto;
    flex-shrink: 0;
  }
  .welcome-list {
    flex: 1 1 320px;
    max-width: 520px;
    margin: 0;
    padding-left: 20px;
    text-align: left;
    font-size: clamp(13px, 1.4vw, 16px);
    line-height: 1.4;
  }
  .welcome-list li {
    margin: 0 0 20px 0;
  }
  .welcome-list li:last-child {
    margin-bottom: 0;
  }

  .node-info-view {
    display: flex;
    flex-direction: column;
    gap: 12px;
    height: 100%;
  }

  .node-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--gtk-border-color);
  }
  .node-name {
    font-size: 15px;
  }
  .node-title-sub {
    font-size: 12px;
    color: var(--gtk-muted-fg);
  }

  .desc-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-height: 120px;
  }
  .desc-label {
    font-size: 12px;
    color: var(--gtk-muted-fg);
  }
  .desc-textarea {
    flex: 1;
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    padding: 8px;
    font-size: 12px;
    resize: none;
    outline: none;
    background: var(--gtk-base-color);
  }
  .desc-textarea:focus {
    background: var(--gtk-base-color);
    border-color: var(--gtk-accent);
  }

  :global(.stats-frame) {
    margin-top: 8px;
  }
  .stats-grid {
    display: grid;
    grid-template-columns: 160px 1fr;
    row-gap: 4px;
    font-size: 12px;
    padding: 4px 0;
  }
  .stat-name {
    color: var(--gtk-muted-fg);
  }
  .stat-value {
    font-weight: 500;
  }
  .stats-actions,
  .screenshots-actions {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }

  :global(.screenshots-frame) {
    margin-top: 8px;
  }
  .screenshots-empty {
    font-size: 12px;
    color: var(--gtk-muted-fg);
    padding: 8px 0;
  }
  .screenshots-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px 0;
  }
  .shot-thumb {
    width: 140px;
    height: 90px;
    border: 1px solid var(--border-color, #444);
    border-radius: 4px;
    overflow: hidden;
    background: #000;
  }
  .shot-thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
</style>
