import { writable, get } from 'svelte/store';
// State and actions of the file browser panels, keyed by session tmpID. SSH tabs
// browse via SFTP, local shell tabs the local file system (see fsBackend.ts).
import * as SFTPService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sftpservice.js';
import * as LocalFSService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/localfsservice.js';
import type { FileEntry } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/sftpsvc/models.js';
import { fsBackend, type BrowserKind } from './fsBackend';
import { DRIVES, baseName, isWindowsPath, joinPath, normalizePath, parentPath, rootPath } from '../fspath';

export interface TransferItem {
  id: string;
  name: string;
  direction: 'upload' | 'download' | 'copy' | 'move';
  progress: number;
  speed: string;
  eta: string;
  status: 'running' | 'completed' | 'failed' | 'cancelled';
  error?: string;
}

export interface SftpViewState {
  kind: BrowserKind;
  sftpID: string;
  /** Windows paths (local browser on Windows): drive letters, "\\" and the drive list. */
  win: boolean;
  /** Folder the browser started in ("Open start folder"). */
  startPath: string;
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

export async function openSftp(tmpID: string, kind: BrowserKind = 'sftp'): Promise<string | null> {
  const backend = fsBackend(kind);
  try {
    const sftpID = await backend.open(tmpID);
    if (!sftpID) return null;

    let initialPath = '';
    try {
      initialPath = (await backend.getwd(sftpID)) || '';
    } catch {}
    const win = kind === 'local' && isWindowsPath(initialPath);
    initialPath = normalizePath(initialPath || rootPath(win), win);

    sftpViews.update((views) => ({
      ...views,
      [tmpID]: {
        kind,
        sftpID,
        win,
        startPath: initialPath,
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
        kind,
        sftpID: '',
        win: false,
        startPath: '',
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

  const norm = normalizePath(targetPath, view.win);

  sftpViews.update((views) => ({
    ...views,
    [tmpID]: { ...views[tmpID], loading: true, error: '' },
  }));

  try {
    const entries = await fsBackend(view.kind).list(view.sftpID, norm, view.showHidden);

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

    // Asynchronously fetch VFS Info (the drive list has none)
    if (norm !== DRIVES) fetchVFSInfo(tmpID, norm);
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
    const vfs = await fsBackend(view.kind).statVFS(view.sftpID, dirPath);
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
    // Free space is not available on all servers, safe to ignore
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
  if (!view) return;
  const parent = parentPath(view.currentPath, view.win);
  if (parent !== null) loadDir(tmpID, parent);
}

/** Navigates to the home folder (Home button). */
export async function goHome(tmpID: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;
  let target = rootPath(view.win);
  try {
    target = (await fsBackend(view.kind).home(view.sftpID)) || target;
  } catch {}
  await loadDir(tmpID, target);
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
  const view = get(sftpViews)[tmpID];
  if (!view) return;
  const nextVal = force !== undefined ? force : !view.followTerminal;
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: { ...curr, followTerminal: nextVal },
    };
  });
  const backend = fsBackend(view.kind);
  backend.setFollow(view.sftpID, nextVal).catch(() => {});
  // A local shell's folder is always known: jump there right away.
  if (nextVal && view.kind === 'local') goToTerminalFolder(tmpID);
}

/** Navigates to the shell's working directory (local tabs) or the last folder the
 *  terminal reported via OSC 7 (SSH tabs). */
export async function goToTerminalFolder(tmpID: string, lastReported = ''): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;
  let target = lastReported;
  if (view.kind === 'local') {
    try {
      target = await fsBackend('local').getwd(view.sftpID);
    } catch {}
  }
  if (target && target !== view.currentPath) await loadDir(tmpID, target);
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

  const p = joinPath(view.currentPath, folderName.trim(), view.win);
  await fsBackend(view.kind).mkdir(view.sftpID, p);
  await refresh(tmpID);
}

export async function createEmptyFile(tmpID: string, fileName: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || !fileName.trim()) return;

  const p = joinPath(view.currentPath, fileName.trim(), view.win);
  await fsBackend(view.kind).createFile(view.sftpID, p);
  await refresh(tmpID);
}

export async function removeItems(tmpID: string, paths: string[]): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || paths.length === 0) return;

  await fsBackend(view.kind).remove(view.sftpID, paths);
  await refresh(tmpID);
}

export async function renameItem(tmpID: string, fromPath: string, newName: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || !newName.trim()) return;

  const dir = parentPath(fromPath, view.win) ?? rootPath(view.win);
  const toPath = joinPath(dir, newName.trim(), view.win);
  await fsBackend(view.kind).rename(view.sftpID, fromPath, toPath);
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

  await fsBackend(view.kind).chmod(view.sftpID, paths, mode, recursive, scope);
  await refresh(tmpID);
}

export async function downloadFiles(tmpID: string, remotePaths: string[], localDir: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || remotePaths.length === 0) return;

  const transferId = 'dl_' + Date.now() + '_' + Math.random().toString(36).substring(2, 7);
  const name = remotePaths.length === 1 ? baseName(remotePaths[0]) || 'file' : `${remotePaths.length} files`;

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

export async function openWithDefaultApp(tmpID: string, remotePath: string): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID) return;

  try {
    await fsBackend(view.kind).openWithDefaultApp(view.sftpID, remotePath);
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

/** Names of paths that already exist in dstDir (local tabs ask the backend, SFTP
 *  uploads go into the listed current folder). */
export async function findConflicts(tmpID: string, paths: string[], dstDir: string): Promise<string[]> {
  const view = get(sftpViews)[tmpID];
  if (!view || !view.sftpID || paths.length === 0) return [];
  if (view.kind === 'local') {
    return (await LocalFSService.Conflicts(view.sftpID, paths, dstDir)) || [];
  }
  const existing = new Set(view.entries.map((e) => e.name));
  return paths.map((p) => p.split(/[/\\]/).pop() || p).filter((n) => existing.has(n));
}

function updateTransfer(tmpID: string, transferId: string, patch: Partial<TransferItem>): void {
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return {
      ...views,
      [tmpID]: {
        ...curr,
        transfers: curr.transfers.map((t) => (t.id === transferId ? { ...t, ...patch } : t)),
      },
    };
  });
}

/** Copies or moves local files into dstDir (local tabs), shown in the transfer queue. */
export async function copyOrMoveLocal(
  tmpID: string,
  op: 'copy' | 'move',
  paths: string[],
  dstDir: string,
  overwrite: boolean
): Promise<void> {
  const view = get(sftpViews)[tmpID];
  if (!view || view.kind !== 'local' || paths.length === 0) return;

  const transferId = op + '_' + Date.now() + '_' + Math.random().toString(36).substring(2, 7);
  const item: TransferItem = {
    id: transferId,
    name: paths.length === 1 ? baseName(paths[0]) || 'file' : `${paths.length} items`,
    direction: op,
    progress: 10,
    speed: op === 'copy' ? 'Copying...' : 'Moving...',
    eta: '',
    status: 'running',
  };
  sftpViews.update((views) => {
    const curr = views[tmpID];
    if (!curr) return views;
    return { ...views, [tmpID]: { ...curr, transfers: [item, ...curr.transfers] } };
  });

  try {
    if (op === 'copy') {
      await LocalFSService.Copy(view.sftpID, paths, dstDir, overwrite);
    } else {
      await LocalFSService.Move(view.sftpID, paths, dstDir, overwrite);
    }
    updateTransfer(tmpID, transferId, { progress: 100, speed: `Done → ${dstDir}`, status: 'completed' });
  } catch (err: any) {
    updateTransfer(tmpID, transferId, { status: 'failed', error: err?.message || String(err) });
    throw err;
  } finally {
    await refresh(tmpID);
  }
}
