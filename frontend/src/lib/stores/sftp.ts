import { writable, get } from 'svelte/store';
import * as SFTPService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sftpservice.js';
import type { FileEntry } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/sftpsvc/models.js';

export interface TransferItem {
  id: string;
  name: string;
  direction: 'upload' | 'download';
  progress: number;
  speed: string;
  eta: string;
  status: 'running' | 'completed' | 'failed' | 'cancelled';
  error?: string;
}

export interface SftpViewState {
  sftpID: string;
  currentPath: string;
  history: string[];
  historyIndex: number;
  entries: FileEntry[];
  showHidden: boolean;
  selectedPaths: string[];
  loading: boolean;
  error: string;
  vfsInfo?: { totalSpace: number; freeSpace: number };
  followTerminal: boolean;
  transfers: TransferItem[];
}

export const sftpViews = writable<Record<string, SftpViewState>>({});

export async function openSftp(tmpID: string): Promise<string | null> {
  try {
    const sftpID = await SFTPService.OpenForSession(tmpID);
    if (!sftpID) return null;

    let initialPath = '/';
    try {
      initialPath = (await SFTPService.Getwd(sftpID)) || '/';
    } catch {}

    sftpViews.update((views) => ({
      ...views,
      [tmpID]: {
        sftpID,
        currentPath: initialPath,
        history: [initialPath],
        historyIndex: 0,
        entries: [],
        showHidden: false,
        selectedPaths: [],
        loading: false,
        error: '',
        followTerminal: false,
        transfers: [],
      },
    }));

    await loadDir(tmpID, initialPath);
    return sftpID;
  } catch (err: any) {
    console.error(`Failed to open SFTP for ${tmpID}:`, err);
    sftpViews.update((views) => ({
      ...views,
      [tmpID]: {
        sftpID: '',
        currentPath: '',
        history: [],
        historyIndex: 0,
        entries: [],
        showHidden: false,
        selectedPaths: [],
        loading: false,
        error: err?.message || String(err),
        followTerminal: false,
        transfers: [],
      },
    }));
    return null;
  }
}

export async function loadDir(tmpID: string, targetPath: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;

  // Normalize path
  let norm = targetPath.trim();
  if (!norm.startsWith('/')) {
    norm = '/' + norm;
  }
  // Strip trailing slashes unless root
  if (norm.length > 1 && norm.endsWith('/')) {
    norm = norm.replace(/\/+$/, '');
  }

  sftpViews.update((views) => ({
    ...views,
    [tmpID]: { ...views[tmpID], loading: true, error: '' },
  }));

  try {
    const entries = await SFTPService.List(view.sftpID, norm, view.showHidden);

    sftpViews.update((views) => {
      const curr = views[tmpID];
      if (!curr) return views;

      let history = [...curr.history];
      let historyIndex = curr.historyIndex;

      if (history[historyIndex] !== norm) {
        history = history.slice(0, historyIndex + 1);
        history.push(norm);
        historyIndex = history.length - 1;
      }

      return {
        ...views,
        [tmpID]: {
          ...curr,
          currentPath: norm,
          entries: entries || [],
          history,
          historyIndex,
          selectedPaths: [],
          loading: false,
          error: '',
        },
      };
    });

    // Asynchronously fetch VFS Info
    fetchVFSInfo(tmpID, norm);
  } catch (err: any) {
    sftpViews.update((views) => ({
      ...views,
      [tmpID]: {
        ...views[tmpID],
        loading: false,
        error: err?.message || String(err),
      },
    }));
  }
}

export async function fetchVFSInfo(tmpID: string, dirPath: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;

  try {
    const vfs = await SFTPService.StatVFS(view.sftpID, dirPath);
    if (vfs) {
      sftpViews.update((views) => {
        const curr = views[tmpID];
        if (!curr) return views;
        return {
          ...views,
          [tmpID]: {
            ...curr,
            vfsInfo: {
              totalSpace: vfs.totalSpace || 0,
              freeSpace: vfs.freeSpace || 0,
            },
          },
        };
      });
    }
  } catch {
    // StatVFS is not supported on all servers, safe to ignore
  }
}

export function navigateBack(tmpID: string): void {
  const view = get(sftpViews)[tmpID];
  if (!view || view.historyIndex <= 0) return;
  const newIndex = view.historyIndex - 1;
  const targetPath = view.history[newIndex];
  loadDir(tmpID, targetPath);
}

export function navigateForward(tmpID: string): void {
  const view = get(sftpViews)[tmpID];
  if (!view || view.historyIndex >= view.history.length - 1) return;
  const newIndex = view.historyIndex + 1;
  const targetPath = view.history[newIndex];
  loadDir(tmpID, targetPath);
}

export function navigateUp(tmpID: string): void {
  const view = get(sftpViews)[tmpID];
  if (!view || view.currentPath === '/' || view.currentPath === '') return;

  const parts = view.currentPath.split('/').filter(Boolean);
  parts.pop();
  const parentPath = '/' + parts.join('/');
  loadDir(tmpID, parentPath);
}

export function refresh(tmpID: string): void {
  const view = get(sftpViews)[tmpID];
  if (!view) return;
  loadDir(tmpID, view.currentPath);
}

export function toggleShowHidden(tmpID: string): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: { ...curr, showHidden: !curr.showHidden },
    };
  });
  refresh(tmpID);
}

export function toggleFollowTerminal(tmpID: string, force?: boolean): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    const nextVal = force !== undefined ? force : !curr.followTerminal;
    return {
      ...views,
      [tmpID]: { ...curr, followTerminal: nextVal },
    };
  });
}

export function setSelectedPaths(tmpID: string, paths: string[]): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: { ...curr, selectedPaths: paths },
    };
  });
}

export async function createFolder(tmpID: string, folderName: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || !folderName.trim()) return;

  const p = view.currentPath === '/' ? `/${folderName.trim()}` : `${view.currentPath}/${folderName.trim()}`;
  await SFTPService.Mkdir(view.sftpID, p, false);
  await refresh(tmpID);
}

export async function createEmptyFile(tmpID: string, fileName: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || !fileName.trim()) return;

  const p = view.currentPath === '/' ? `/${fileName.trim()}` : `${view.currentPath}/${fileName.trim()}`;
  await SFTPService.CreateFile(view.sftpID, p);
  await refresh(tmpID);
}

export async function removeItems(tmpID: string, paths: string[]): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || paths.length === 0) return;

  await SFTPService.Remove(view.sftpID, paths, true);
  await refresh(tmpID);
}

export async function renameItem(tmpID: string, fromPath: string, newName: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || !newName.trim()) return;

  const parts = fromPath.split('/');
  parts.pop();
  const toPath = (parts.join('/') || '') + '/' + newName.trim();
  await SFTPService.Rename(view.sftpID, fromPath, toPath);
  await refresh(tmpID);
}

export async function chmodItems(
  tmpID: string,
  paths: string[],
  mode: number,
  recursive = false,
  scope = 'all'
): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || paths.length === 0) return;

  await SFTPService.Chmod(view.sftpID, paths, mode, recursive, scope);
  await refresh(tmpID);
}

export async function downloadFiles(tmpID: string, remotePaths: string[], localDir: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || remotePaths.length === 0) return;

  const transferId = 'dl_' + Date.now() + '_' + Math.random().toString(36).substring(2, 7);
  const name = remotePaths.length === 1 ? remotePaths[0].split('/').pop() || 'file' : `${remotePaths.length} files`;

  const item: TransferItem = {
    id: transferId,
    name,
    direction: 'download',
    progress: 10,
    speed: 'Calculating...',
    eta: '',
    status: 'running',
  };

  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: {
        ...curr,
        transfers: [item, ...curr.transfers],
      },
    };
  });

  try {
    const savedTo = await SFTPService.Download(view.sftpID, remotePaths, localDir);
    sftpViews.update((views) => {
      const curr = views[tmpID];
      if (!curr) return views;
      return {
        ...views,
        [tmpID]: {
          ...curr,
          transfers: curr.transfers.map((t) =>
            t.id === transferId
              ? { ...t, progress: 100, speed: savedTo ? `Saved to ${savedTo}` : 'Done', status: 'completed' }
              : t
          ),
        },
      };
    });
  } catch (err: any) {
    sftpViews.update((views) => {
      const curr = views[tmpID];
      if (!curr) return views;
      return {
        ...views,
        [tmpID]: {
          ...curr,
          transfers: curr.transfers.map((t) =>
            t.id === transferId
              ? { ...t, status: 'failed', error: err?.message || String(err) }
              : t
          ),
        },
      };
    });
    throw err;
  }
}

export async function uploadFiles(tmpID: string, localPaths: string[]): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || localPaths.length === 0) return;

  const transferId = 'ul_' + Date.now() + '_' + Math.random().toString(36).substring(2, 7);
  const name = localPaths.length === 1 ? localPaths[0].split(/[/\\]/).pop() || 'file' : `${localPaths.length} files`;

  const item: TransferItem = {
    id: transferId,
    name,
    direction: 'upload',
    progress: 10,
    speed: 'Calculating...',
    eta: '',
    status: 'running',
  };

  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: {
        ...curr,
        transfers: [item, ...curr.transfers],
      },
    };
  });

  try {
    await SFTPService.Upload(view.sftpID, localPaths, view.currentPath);
    sftpViews.update((views) => {
      const curr = views[tmpID];
      if (!curr) return views;
      return {
        ...views,
        [tmpID]: {
          ...curr,
          transfers: curr.transfers.map((t) =>
            t.id === transferId
              ? { ...t, progress: 100, speed: 'Done', status: 'completed' }
              : t
          ),
        },
      };
    });
    await refresh(tmpID);
  } catch (err: any) {
    sftpViews.update((views) => {
      const curr = views[tmpID];
      if (!curr) return views;
      return {
        ...views,
        [tmpID]: {
          ...curr,
          transfers: curr.transfers.map((t) =>
            t.id === transferId
              ? { ...t, status: 'failed', error: err?.message || String(err) }
              : t
          ),
        },
      };
    });
    throw err;
  }
}

export function cancelTransfer(tmpID: string, transferId: string): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: {
        ...curr,
        transfers: curr.transfers.map((t) =>
          t.id === transferId ? { ...t, status: 'cancelled' } : t
        ),
      },
    };
  });
}

export function dismissTransfer(tmpID: string, transferId: string): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: {
        ...curr,
        transfers: curr.transfers.filter((t) => t.id !== transferId),
      },
    };
  });
}

export function hasRunningTransfers(): boolean {
  const views = get(sftpViews);
  for (const v of Object.values(views)) {
    if (v.transfers && v.transfers.some((t) => t.status === 'running')) {
      return true;
    }
  }
  return false;
}

export async function openWithDefaultApp(tmpID: string, remotePath: string): Promise<string | null> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return null;

  try {
    const localPath = await SFTPService.OpenWithDefaultApp(view.sftpID, remotePath);
    return localPath;
  } catch (err: any) {
    console.error(`Failed to open ${remotePath} with default app:`, err);
    throw err;
  }
}

export async function uploadBack(tmpID: string, localPath: string, remotePath: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;

  try {
    await SFTPService.UploadBack(view.sftpID, localPath, remotePath);
    await refresh(tmpID);
  } catch (err: any) {
    console.error(`Failed to upload back ${localPath} -> ${remotePath}:`, err);
    throw err;
  }
}
