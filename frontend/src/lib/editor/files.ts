// Loading and saving editor documents through EditorService, in chunks so that
// large files do not travel as one huge message.
import type { Text } from '@codemirror/state';
import * as EditorService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/editorservice.js';

export interface EditorFile {
  kind: 'local' | 'sftp';
  sftpId?: string;
  /** Saved connection; lets a remote file continue after a reconnect. */
  connId?: string;
  host?: string;
  path: string;
}

export interface FileInfo {
  exists: boolean;
  isDir: boolean;
  size: number;
  modTime: number;
  mode: number;
}

export interface LoadedFile {
  text: string;
  size: number;
  modTime: number;
  encoding: string;
  lineEnding: 'lf' | 'crlf';
  mixedLineEndings: boolean;
}

export interface SaveOptions {
  encoding: string;
  lineEnding: 'lf' | 'crlf';
  expectedModTime: number;
  expectedSize: number;
  force: boolean;
}

export interface SaveResult {
  conflict: boolean;
  size: number;
  modTime: number;
}

/** UTF-16 code units per save chunk. */
const SAVE_CHUNK = 2 * 1024 * 1024;

export const fileKey = (f: EditorFile) => `${f.kind}:${f.sftpId ?? ''}:${f.path}`;

export function displayPath(f: EditorFile): string {
  return f.kind === 'sftp' ? `${f.host || 'remote'}:${f.path}` : f.path;
}

export function baseName(path: string): string {
  const parts = path.replace(/\\/g, '/').split('/');
  return parts[parts.length - 1] || path;
}

export async function statFile(file: EditorFile): Promise<FileInfo> {
  return (await EditorService.Stat(file as any)) as FileInfo;
}

/**
 * Reads a file. Returns null when it looks binary and allowBinary is false.
 * onProgress receives the number of characters received so far.
 */
export async function loadFile(
  file: EditorFile,
  encoding = '',
  allowBinary = false,
  onProgress?: (chars: number) => void
): Promise<LoadedFile | null> {
  const res: any = await EditorService.Load(file as any, encoding, allowBinary);
  if (res.binary) return null;
  const parts: string[] = [];
  let received = 0;
  try {
    for (;;) {
      const chunk: any = await EditorService.ReadChunk(res.loadId);
      parts.push(chunk.text);
      received += chunk.text.length;
      onProgress?.(received);
      if (chunk.done) break;
    }
  } catch (err) {
    EditorService.Discard(res.loadId).catch(() => {});
    throw err;
  }
  return {
    text: parts.join(''),
    size: res.size,
    modTime: res.modTime,
    encoding: res.encoding,
    lineEnding: res.lineEnding === 'crlf' ? 'crlf' : 'lf',
    mixedLineEndings: res.mixedLineEndings,
  };
}

/** Splits [0, length) into chunks that never cut a surrogate pair. */
export function chunkRanges(doc: Pick<Text, 'length' | 'sliceString'>, size = SAVE_CHUNK): Array<[number, number]> {
  const out: Array<[number, number]> = [];
  let from = 0;
  while (from < doc.length) {
    let to = Math.min(doc.length, from + size);
    if (to < doc.length) {
      const last = doc.sliceString(to - 1, to).charCodeAt(0);
      if (last >= 0xd800 && last <= 0xdbff) to--;
    }
    out.push([from, to]);
    from = to;
  }
  return out;
}

/** Writes a document (see SaveOptions for the conflict check). */
export async function saveFile(file: EditorFile, doc: Text, opts: SaveOptions): Promise<SaveResult> {
  const id = await EditorService.BeginSave();
  try {
    for (const [from, to] of chunkRanges(doc)) {
      await EditorService.AppendSave(id, doc.sliceString(from, to));
    }
  } catch (err) {
    EditorService.Discard(id).catch(() => {});
    throw err;
  }
  return (await EditorService.CommitSave(id, file as any, opts as any)) as SaveResult;
}

export type SizeClass = 'normal' | 'large' | 'huge';

/** Large files need one confirmation, huge ones two (and start without highlighting). */
export function classifySize(bytes: number, largeMB: number, hugeMB: number): SizeClass {
  const mb = bytes / (1024 * 1024);
  if (mb > hugeMB) return 'huge';
  if (mb > largeMB) return 'large';
  return 'normal';
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = bytes;
  let i = -1;
  do {
    v /= 1024;
    i++;
  } while (v >= 1024 && i < units.length - 1);
  return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}
