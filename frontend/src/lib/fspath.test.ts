import { describe, expect, it } from 'vitest';
import { DRIVES, baseName, isRootPath, isWindowsPath, joinPath, normalizePath, parentPath } from './fspath';

describe('POSIX paths', () => {
  it('normalizes, joins and goes up', () => {
    expect(normalizePath('home/u/', false)).toBe('/home/u');
    expect(normalizePath('/a//b', false)).toBe('/a/b');
    expect(normalizePath('/', false)).toBe('/');
    expect(joinPath('/', 'etc', false)).toBe('/etc');
    expect(joinPath('/etc', 'hosts', false)).toBe('/etc/hosts');
    expect(parentPath('/etc/ssh', false)).toBe('/etc');
    expect(parentPath('/etc', false)).toBe('/');
    expect(parentPath('/', false)).toBeNull();
    expect(isRootPath('/', false)).toBe(true);
  });
});

describe('Windows paths', () => {
  it('detects Windows paths', () => {
    expect(isWindowsPath('C:\\Users\\me')).toBe(true);
    expect(isWindowsPath('d:/data')).toBe(true);
    expect(isWindowsPath('\\\\server\\share')).toBe(true);
    expect(isWindowsPath(DRIVES)).toBe(true);
    expect(isWindowsPath('/home/me')).toBe(false);
  });

  it('normalizes drive and UNC paths', () => {
    expect(normalizePath('c:', true)).toBe('C:\\');
    expect(normalizePath('C:/Users/me/', true)).toBe('C:\\Users\\me');
    expect(normalizePath('C:\\\\Users\\\\me', true)).toBe('C:\\Users\\me');
    expect(normalizePath('\\\\server\\share', true)).toBe('\\\\server\\share\\');
    expect(normalizePath('\\\\server\\share\\dir\\', true)).toBe('\\\\server\\share\\dir');
    expect(normalizePath('', true)).toBe(DRIVES);
    expect(normalizePath('this pc', true)).toBe(DRIVES);
  });

  it('goes up to the drive list', () => {
    expect(parentPath('C:\\Users\\me', true)).toBe('C:\\Users');
    expect(parentPath('C:\\Users', true)).toBe('C:\\');
    expect(parentPath('C:\\', true)).toBe(DRIVES);
    expect(parentPath('\\\\server\\share\\dir', true)).toBe('\\\\server\\share');
    expect(parentPath('\\\\server\\share\\', true)).toBe(DRIVES);
    expect(parentPath(DRIVES, true)).toBeNull();
  });

  it('joins and names', () => {
    expect(joinPath('C:\\', 'Users', true)).toBe('C:\\Users');
    expect(joinPath('C:\\Users', 'me', true)).toBe('C:\\Users\\me');
    expect(baseName('C:\\Users\\me\\file.txt')).toBe('file.txt');
    expect(baseName('/etc/hosts')).toBe('hosts');
  });
});
