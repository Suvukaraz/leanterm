import { writable } from 'svelte/store';

export type LeftTab = 'connections' | 'favourites' | 'history' | 'clusters';
export type SearchField = 'name' | 'host' | 'desc';

export interface AddRenameDialogState {
  open: boolean;
  isRename: boolean;
  isGroup: boolean;
  parentId: string;
  nodeId: string;
  currentName: string;
  currentTitle: string;
}

export interface DeleteConfirmDialogState {
  open: boolean;
  nodeIds: string[];
  count: number;
}

export interface EditConnDialogState {
  open: boolean;
  connId: string;
  isQuick: boolean;
}

export interface WolDialogState {
  open: boolean;
  mac: string;
  host: string;
  port: number;
}

export const commandPanelVisible = writable<boolean>(true);
export const commandPanelWidth = writable<number>(260);
/** Show the cluster features (Clusters tab, buttons, terminal menu entries). */
export const clustersVisible = writable<boolean>(false);
/** Show the SFTP side panels of all terminals (each tab can still toggle its own). */
export const sftpVisible = writable<boolean>(true);
/** Width of the SFTP side panel in px; 0 = use the sftp_browser.width preference. */
export const sftpPanelWidth = writable<number>(0);
/** Groups expanded in the connection tree (restored on start). */
export const expandedGroups = writable<string[]>([]);
export const searchVisible = writable<boolean>(false);
export const searchQuery = writable<string>('');
export const searchField = writable<SearchField>('name');
export const searchLastMatchId = writable<string>('');
export const activeLeftTab = writable<LeftTab>('connections');

export const isLocked = writable<boolean>(false);

/** Incremented to ask for quitting; QuitConfirm shows the confirmations. */
export const quitRequested = writable<number>(0);
export function requestQuit(): void {
  quitRequested.update((n) => n + 1);
}
export const isReadOnly = writable<boolean>(false);

export const addRenameDialog = writable<AddRenameDialogState>({
  open: false,
  isRename: false,
  isGroup: false,
  parentId: '',
  nodeId: '',
  currentName: '',
  currentTitle: '',
});

export const deleteConfirmDialog = writable<DeleteConfirmDialogState>({
  open: false,
  nodeIds: [],
  count: 0,
});

export const editConnDialog = writable<EditConnDialogState>({
  open: false,
  connId: '',
  isQuick: false,
});

export const aboutDialogOpen = writable<boolean>(false);
export const prefsDialogOpen = writable<boolean>(false);
/** Page Preferences opens on next time ('' = the last one). */
export const prefsStartPage = writable<string>('');
export const wolDialog = writable<WolDialogState>({
  open: false,
  mac: '',
  host: '',
  port: 9,
});
export const keybindingsDialogOpen = writable<boolean>(false);
export const clusterAdminDialogOpen = writable<boolean>(false);
export const pccDialogOpen = writable<boolean>(false);
export const bulkEditDialogOpen = writable<{ open: boolean; connIds: string[] }>({
  open: false,
  connIds: [],
});
export const editVariablesDialog = writable<{ open: boolean; connId: string }>({
  open: false,
  connId: '',
});

export interface DialogGeometry {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Remembered position/size of movable dialogs, keyed by Dialog geometryKey. */
export const dialogGeometries = writable<Record<string, DialogGeometry>>({});
