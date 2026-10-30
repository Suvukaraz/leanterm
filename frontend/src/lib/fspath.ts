// Path handling of the file browsers. SFTP and local Linux/macOS paths are POSIX;
// the local browser on Windows uses drive paths (C:\Users\me) and UNC paths
// (\\server\share), with a virtual top level that lists the drives.

/** Virtual folder above the drive roots of the Windows local browser. */
export const DRIVES = 'This PC';

/** Whether p is a Windows path (drive, UNC or the drive list). */
export function isWindowsPath(p: string): boolean {
  return p === DRIVES || /^[A-Za-z]:([\\/]|$)/.test(p) || /^\\\\[^\\]/.test(p);
}

/** Cleans a typed or joined path: POSIX "/a/b", Windows "C:\a\b" (drive roots keep their "\"). */
export function normalizePath(p: string, win: boolean): string {
  let s = p.trim();
  if (!win) {
    if (!s.startsWith('/')) s = '/' + s;
    if (s.length > 1) s = s.replace(/\/+$/, '');
    return s.replace(/\/{2,}/g, '/');
  }
  if (s === '' || s.toLowerCase() === DRIVES.toLowerCase()) return DRIVES;
  s = s.replace(/\//g, '\\');
  const unc = s.startsWith('\\\\');
  s = (unc ? '\\\\' : '') + s.slice(unc ? 2 : 0).replace(/\\{2,}/g, '\\');
  const drive = /^([A-Za-z]):/.exec(s);
  if (drive) {
    s = drive[1].toUpperCase() + s.slice(1);
    if (s.length === 2) return s + '\\';
  }
  if (isRootPath(s, true)) return s.endsWith('\\') ? s : s + '\\';
  return s.replace(/\\+$/, '');
}

/** Top of the tree: "/" (POSIX) or the drive list (Windows). */
export function rootPath(win: boolean): string {
  return win ? DRIVES : '/';
}

/** "/", a drive root "C:\", a UNC share root "\\server\share\" or the drive list. */
export function isRootPath(p: string, win: boolean): boolean {
  if (!win) return p === '/' || p === '';
  if (p === DRIVES || p === '') return true;
  if (/^[A-Za-z]:\\?$/.test(p)) return true;
  return /^\\\\[^\\]+\\[^\\]+\\?$/.test(p);
}

/** Parent folder, or null at the top. Drive and share roots go up to the drive list. */
export function parentPath(p: string, win: boolean): string | null {
  if (!win) {
    if (p === '/' || p === '') return null;
    const parts = p.split('/').filter(Boolean);
    parts.pop();
    return '/' + parts.join('/');
  }
  if (p === DRIVES || p === '') return null;
  if (isRootPath(p, true)) return DRIVES;
  const trimmed = p.replace(/\\+$/, '');
  const idx = trimmed.lastIndexOf('\\');
  const parent = trimmed.slice(0, idx);
  return /^[A-Za-z]:$/.test(parent) ? parent + '\\' : parent;
}

/** dir + name with the right separator. */
export function joinPath(dir: string, name: string, win: boolean): string {
  if (!win) return dir === '/' || dir === '' ? `/${name}` : `${dir}/${name}`;
  return dir.endsWith('\\') ? dir + name : `${dir}\\${name}`;
}

/** Last path segment (either separator). */
export function baseName(p: string): string {
  const parts = p.split(/[\\/]/).filter(Boolean);
  return parts.length ? parts[parts.length - 1] : p;
}
