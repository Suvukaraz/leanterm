import { describe, it, expect, vi, beforeEach } from 'vitest';

const sftp = vi.hoisted(() => ({
  OpenForSession: vi.fn(async (tmpID: string) => 'sftp-' + tmpID),
  Getwd: vi.fn(async () => '/home/remote'),
  List: vi.fn(async () => [{ name: 'a.txt', path: '/home/remote/a.txt' }]),
  StatVFS: vi.fn(async () => null),
}));
const local = vi.hoisted(() => ({
  Getwd: vi.fn(async () => '/home/me/project'),
  List: vi.fn(async () => [{ name: 'b.txt', path: '/home/me/project/b.txt' }]),
  StatVFS: vi.fn(async () => null),
  Conflicts: vi.fn(async () => ['b.txt']),
  SetFollow: vi.fn(async () => {}),
  Home: vi.fn(async () => '/home/me'),
}));

vi.mock('../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sftpservice.js', () => sftp);
vi.mock('../../../bindings/github.com/Suvukaraz/leanterm/internal/services/localfsservice.js', () => local);

import { get } from 'svelte/store';
import { sftpViews, openSftp, findConflicts, toggleFollowTerminal, goHome } from './sftp';

describe('file browser backends', () => {
  beforeEach(() => {
    sftpViews.set({});
    vi.clearAllMocks();
  });

  it('opens SSH tabs via SFTP in the remote home folder', async () => {
    await openSftp('t1');
    const v = get(sftpViews)['t1'];
    expect(v.kind).toBe('sftp');
    expect(v.sftpID).toBe('sftp-t1');
    expect(v.startPath).toBe('/home/remote');
    expect(sftp.List).toHaveBeenCalledWith('sftp-t1', '/home/remote', false);
    expect(local.List).not.toHaveBeenCalled();
  });

  it('opens local tabs in the shell working directory', async () => {
    await openSftp('t2', 'local');
    const v = get(sftpViews)['t2'];
    expect(v.kind).toBe('local');
    expect(v.sftpID).toBe('t2');
    expect(v.currentPath).toBe('/home/me/project');
    expect(local.List).toHaveBeenCalledWith('t2', '/home/me/project', false);
    expect(sftp.OpenForSession).not.toHaveBeenCalled();
  });

  it('detects conflicts from the listing (SFTP) or the backend (local)', async () => {
    await openSftp('t1');
    expect(await findConflicts('t1', ['/tmp/a.txt', '/tmp/c.txt'], '/home/remote')).toEqual(['a.txt']);

    await openSftp('t2', 'local');
    expect(await findConflicts('t2', ['/tmp/b.txt'], '/home/me/project')).toEqual(['b.txt']);
    expect(local.Conflicts).toHaveBeenCalledWith('t2', ['/tmp/b.txt'], '/home/me/project');
  });

  it('follows the local shell folder through the backend', async () => {
    await openSftp('t2', 'local');
    toggleFollowTerminal('t2');
    expect(local.SetFollow).toHaveBeenCalledWith('t2', true);
    expect(get(sftpViews)['t2'].followTerminal).toBe(true);
  });

  it('Home goes to the user home (local) or the SFTP login folder', async () => {
    await openSftp('t2', 'local');
    await goHome('t2');
    expect(local.Home).toHaveBeenCalledWith('t2');
    expect(get(sftpViews)['t2'].currentPath).toBe('/home/me');

    await openSftp('t1');
    await goHome('t1');
    expect(get(sftpViews)['t1'].currentPath).toBe('/home/remote');
  });
});
