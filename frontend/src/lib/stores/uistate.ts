// Persists panel layout (tree width/visibility, SFTP panel width, active left tab,
// expanded groups, cluster features, SFTP panels on/off and the geometry of movable
// dialogs) to state/ui.toml via the backend. The window geometry in the same
// file is written by the Go side when the window closes.
import { get } from 'svelte/store';
import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
import {
  commandPanelWidth,
  commandPanelVisible,
  clustersVisible,
  sftpVisible,
  sftpPanelWidth,
  activeLeftTab,
  expandedGroups,
  dialogGeometries,
  type LeftTab,
} from './ui';

let loaded: any = null;
let saveTimer: ReturnType<typeof setTimeout> | undefined;

function scheduleSave(): void {
  if (!loaded) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(async () => {
    try {
      // Re-read so the window geometry written by Go is not overwritten.
      const current: any = (await ConfigService.GetUIState()) ?? loaded;
      current.commandPanelWidth = get(commandPanelWidth);
      current.commandPanelVisible = get(commandPanelVisible);
      current.clustersVisible = get(clustersVisible);
      current.sftpVisible = get(sftpVisible);
      current.sftpPanelWidth = get(sftpPanelWidth);
      current.activeLeftTab = get(activeLeftTab);
      current.expandedGroups = get(expandedGroups);
      current.dialogs = get(dialogGeometries);
      await ConfigService.SaveUIState(current);
    } catch (err) {
      console.warn('Failed to save UI state:', err);
    }
  }, 1000);
}

export async function loadUIState(): Promise<void> {
  try {
    const st: any = await ConfigService.GetUIState();
    if (st) {
      if (st.commandPanelWidth >= 150) commandPanelWidth.set(st.commandPanelWidth);
      commandPanelVisible.set(st.commandPanelVisible !== false);
      clustersVisible.set(st.clustersVisible === true);
      sftpVisible.set(st.sftpVisible !== false);
      sftpPanelWidth.set(st.sftpPanelWidth || 0);
      expandedGroups.set(Array.isArray(st.expandedGroups) ? st.expandedGroups : []);
      dialogGeometries.set(st.dialogs && typeof st.dialogs === 'object' ? st.dialogs : {});
    }
    loaded = st ?? {};
  } catch (err) {
    console.warn('Failed to load UI state:', err);
    loaded = {};
  }
  for (const store of [
    commandPanelWidth,
    commandPanelVisible,
    clustersVisible,
    sftpVisible,
    sftpPanelWidth,
    activeLeftTab,
    expandedGroups,
    dialogGeometries,
  ]) {
    let first = true;
    (store as any).subscribe(() => {
      if (first) {
        first = false;
        return;
      }
      scheduleSave();
    });
  }
}
