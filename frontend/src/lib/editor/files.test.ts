import { describe, expect, it } from 'vitest';
import { Text } from '@codemirror/state';
import { chunkRanges, classifySize, displayPath, fileKey, formatSize } from './files';

describe('chunkRanges', () => {
  it('covers the whole document', () => {
    const doc = Text.of(['abcdefghij']);
    expect(chunkRanges(doc, 4)).toEqual([[0, 4], [4, 8], [8, 10]]);
    expect(chunkRanges(Text.of(['']), 4)).toEqual([]);
  });

  it('never splits a surrogate pair', () => {
    // "a😀b": the emoji takes positions 1-2 (two UTF-16 code units).
    const doc = Text.of(['a😀b']);
    const ranges = chunkRanges(doc, 2);
    expect(ranges).toEqual([[0, 1], [1, 3], [3, 4]]);
    expect(ranges.map(([f, t]) => doc.sliceString(f, t)).join('')).toBe('a😀b');
  });
});

describe('sizes and names', () => {
  it('classifies sizes against the limits', () => {
    const MB = 1024 * 1024;
    expect(classifySize(5 * MB, 10, 100)).toBe('normal');
    expect(classifySize(10 * MB, 10, 100)).toBe('normal');
    expect(classifySize(11 * MB, 10, 100)).toBe('large');
    expect(classifySize(101 * MB, 10, 100)).toBe('huge');
  });

  it('formats sizes', () => {
    expect(formatSize(512)).toBe('512 B');
    expect(formatSize(1536)).toBe('1.5 KB');
    expect(formatSize(250 * 1024 * 1024)).toBe('250 MB');
  });

  it('names files', () => {
    const remote = { kind: 'sftp' as const, sftpId: 't1', host: 'web1', path: '/etc/hosts' };
    expect(displayPath(remote)).toBe('web1:/etc/hosts');
    expect(fileKey(remote)).not.toBe(fileKey({ ...remote, sftpId: 't2' }));
    expect(displayPath({ kind: 'local', path: '/tmp/a' })).toBe('/tmp/a');
  });
});
