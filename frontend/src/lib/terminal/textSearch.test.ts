import { describe, expect, it } from 'vitest';
import { buildMatcher, findMatch, type SearchRow } from './textSearch';

function row(text: string): SearchRow {
  const idx = [...text].map((_, i) => i);
  return { text, start: idx, end: idx };
}

const opts = { caseSensitive: false, wholeWord: false, regex: false };

describe('buildMatcher', () => {
  it('escapes plain text and honours case', () => {
    expect(buildMatcher('a.b', opts)!.test('aXb')).toBe(false);
    expect(buildMatcher('ABC', opts)!.test('xabcx')).toBe(true);
    expect(buildMatcher('ABC', { ...opts, caseSensitive: true })!.test('xabcx')).toBe(false);
  });

  it('supports whole words and regular expressions', () => {
    const word = buildMatcher('log', { ...opts, wholeWord: true })!;
    expect(word.test('syslog')).toBe(false);
    word.lastIndex = 0;
    expect(word.test('a log b')).toBe(true);
    expect(buildMatcher('er+or', { ...opts, regex: true })!.test('errrror')).toBe(true);
  });

  it('rejects empty and invalid patterns', () => {
    expect(buildMatcher('', opts)).toBeUndefined();
    expect(buildMatcher('(', { ...opts, regex: true })).toBeUndefined();
  });
});

describe('findMatch', () => {
  const rows = [row('foo bar'), row('nothing'), row('bar foo bar')];
  const get = (i: number) => rows[i];
  const re = buildMatcher('bar', opts)!;

  it('finds forward from a position and wraps', () => {
    expect(findMatch(get, 3, re, { row: 0, col: -1 }, false)).toEqual({ row: 0, col: 4, endCol: 6 });
    expect(findMatch(get, 3, re, { row: 0, col: 4 }, false)).toEqual({ row: 2, col: 0, endCol: 2 });
    expect(findMatch(get, 3, re, { row: 2, col: 8 }, false)).toEqual({ row: 0, col: 4, endCol: 6 });
  });

  it('finds backward and wraps', () => {
    expect(findMatch(get, 3, re, { row: 2, col: 8 }, true)).toEqual({ row: 2, col: 0, endCol: 2 });
    expect(findMatch(get, 3, re, { row: 2, col: 0 }, true)).toEqual({ row: 0, col: 4, endCol: 6 });
    expect(findMatch(get, 3, re, { row: 0, col: 4 }, true)).toEqual({ row: 2, col: 8, endCol: 10 });
  });

  it('returns the only match again after a full wrap', () => {
    const single = [row('x'), row('only bar')];
    expect(findMatch((i) => single[i], 2, re, { row: 1, col: 5 }, false)).toEqual({ row: 1, col: 5, endCol: 7 });
  });

  it('maps string indexes to cell columns (wide characters)', () => {
    // "日" occupies columns 0-1, "x" column 2.
    const wide: SearchRow = { text: '日x', start: [0, 2], end: [1, 2] };
    const m = findMatch(() => wide, 1, buildMatcher('日x', opts)!, { row: 0, col: -1 }, false);
    expect(m).toEqual({ row: 0, col: 0, endCol: 2 });
  });

  it('returns undefined without matches', () => {
    expect(findMatch(get, 3, buildMatcher('zzz', opts)!, { row: 0, col: -1 }, false)).toBeUndefined();
  });
});
