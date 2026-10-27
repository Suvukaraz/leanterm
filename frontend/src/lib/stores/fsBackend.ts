// One interface for the two file browser backends: SFTP (remote, SSH tabs) and the
// local file system (local shell tabs). The browser store and components only talk
// to FsBackend, so both share the same UI.
import * as SFTPService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sftpservice.js';
import * as LocalFSService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/localfsservice.js';
import type { FileEntry } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/sftpsvc/models.js';

export type BrowserKind = 'sftp' | 'local';

export interface FsBackend {
  kind: BrowserKind;
  /** Prepares the browser of a session and returns the id for the other calls. */
  open(tmpID: string): Promise<string>;
  /** Start folder: SFTP home, or the working directory of the local shell. */
  getwd(id: string): Promise<string>;
  /** Home folder: the SFTP login folder, or the local user's home. */
  home(id: string): Promise<string>;
  list(id: string, path: string, showHidden: boolean): Promise<FileEntry[]>;
  statVFS(id: string, path: string): Promise<{ totalSpace: number; freeSpace: number } | null>;
  mkdir(id: string, path: string): Promise<void>;
  createFile(id: string, path: string): Promise<void>;
  remove(id: string, paths: string[]): Promise<void>;
  rename(id: string, from: string, to: string): Promise<void>;
  chmod(id: string, paths: string[], mode: number, recursive: boolean, scope: string): Promise<void>;
  /** Opens a file in its default application (SFTP: via a temporary copy). */
  openWithDefaultApp(id: string, path: string): Promise<void>;
  /** Turns "follow terminal folder" on/off where the backend has to track it. */
  setFollow(id: string, on: boolean): Promise<void>;
  /** Releases backend resources when the panel goes away. */
  close(id: string): Promise<void>;
}

const sftpBackend: FsBackend = {
  kind: 'sftp',
  open: (tmpID) => SFTPService.OpenForSession(tmpID),
  getwd: (id) => SFTPService.Getwd(id),
  // The SFTP server starts each session in the user's home folder.
  home: (id) => SFTPService.Getwd(id),
  list: async (id, p, hidden) => (await SFTPService.List(id, p, hidden)) || [],
  statVFS: (id, p) => SFTPService.StatVFS(id, p),
  mkdir: (id, p) => SFTPService.Mkdir(id, p, false),
  createFile: (id, p) => SFTPService.CreateFile(id, p),
  remove: (id, paths) => SFTPService.Remove(id, paths, true),
  rename: (id, from, to) => SFTPService.Rename(id, from, to),
  chmod: (id, paths, mode, recursive, scope) => SFTPService.Chmod(id, paths, mode, recursive, scope),
  openWithDefaultApp: async (id, p) => {
    await SFTPService.OpenWithDefaultApp(id, p);
  },
  // SSH sessions report folder changes via OSC 7 on their own.
  setFollow: async () => {},
  close: async () => {},
};

const localBackend: FsBackend = {
  kind: 'local',
  // The local browser needs no connection; the session's tmpID is the id.
  open: async (tmpID) => tmpID,
  getwd: (id) => LocalFSService.Getwd(id),
  home: (id) => LocalFSService.Home(id),
  list: async (id, p, hidden) => (await LocalFSService.List(id, p, hidden)) || [],
  statVFS: (id, p) => LocalFSService.StatVFS(id, p),
  mkdir: (id, p) => LocalFSService.Mkdir(id, p, false),
  createFile: (id, p) => LocalFSService.CreateFile(id, p),
  remove: (id, paths) => LocalFSService.Remove(id, paths, true),
  rename: (id, from, to) => LocalFSService.Rename(id, from, to),
  chmod: (id, paths, mode, recursive, scope) => LocalFSService.Chmod(id, paths, mode, recursive, scope),
  openWithDefaultApp: (id, p) => LocalFSService.OpenWithDefaultApp(id, p),
  setFollow: (id, on) => LocalFSService.SetFollow(id, on),
  close: (id) => LocalFSService.Close(id),
};

export function fsBackend(kind: BrowserKind | undefined): FsBackend {
  return kind === 'local' ? localBackend : sftpBackend;
}
