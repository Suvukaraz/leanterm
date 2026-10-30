import { beforeAll, describe, expect, it } from 'vitest';
import { Terminal, initWasm } from 'rioterm';
import { nextMatch, rioSearchPattern } from './rioEngine';

const opts = { caseSensitive: false, wholeWord: false, regex: false };

describe('rioSearchPattern', () => {
  it('escapes plain text and makes the case explicit', () => {
    expect(rioSearchPattern('', opts)).toBeUndefined();
    expect(rioSearchPattern('a.b(c)', opts)).toBe('(?i)a\\.b\\(c\\)');
    expect(rioSearchPattern('Foo', { ...opts, caseSensitive: true })).toBe('(?-i)Foo');
    expect(rioSearchPattern('a|b', { ...opts, regex: true })).toBe('(?i)(?:a|b)');
  });

  it('adds word boundaries only next to word characters', () => {
    expect(rioSearchPattern('foo', { ...opts, wholeWord: true })).toBe('(?i)(?-u:\\b)foo(?-u:\\b)');
    expect(rioSearchPattern('-v', { ...opts, wholeWord: true })).toBe('(?i)-v(?-u:\\b)');
  });
});

describe('nextMatch', () => {
  const m = (line: number, col: number) => ({ startLine: line, startCol: col, endLine: line, endCol: col + 1 });
  const matches = [m(0, 5), m(2, 0), m(2, 8)];

  it('steps forward and backward with wrap-around', () => {
    expect(nextMatch(matches, { line: 2, col: 0 }, false, false)).toEqual(m(2, 8));
    expect(nextMatch(matches, { line: 2, col: 8 }, false, false)).toEqual(m(0, 5));
    expect(nextMatch(matches, { line: 2, col: 0 }, true, false)).toEqual(m(0, 5));
    expect(nextMatch(matches, { line: 0, col: 5 }, true, false)).toEqual(m(2, 8));
  });

  it('keeps the current match when inclusive (incremental typing)', () => {
    expect(nextMatch(matches, { line: 2, col: 0 }, false, true)).toEqual(m(2, 0));
    expect(nextMatch(matches, { line: 2, col: 0 }, true, true)).toEqual(m(2, 0));
  });
});

describe('patterns against the Rio core', () => {
  let t: Terminal;
  beforeAll(async () => {
    // Browsers fetch the binary next to the glue; here it is read from the frontend folder.
    // (Untyped import: the project has no Node type declarations.)
    const fs: { readFileSync(path: string): Uint8Array } = await import(/* @vite-ignore */ 'node:fs' as string);
    await initWasm(fs.readFileSync('node_modules/rioterm/wasm/librio_wasm_bg.wasm'));
    t = new Terminal({ cols: 60, rows: 4 });
    t.write('foo-bar Foo xfoo foox a.b axb (x) -v\r\n');
  });

  const cols = (text: string, o = {}) => t.search(rioSearchPattern(text, { ...opts, ...o })!).map((x) => x.startCol);

  it('honours case sensitivity in both directions (no smart case)', () => {
    expect(cols('foo')).toEqual([0, 8, 13, 17]);
    expect(cols('foo', { caseSensitive: true })).toEqual([0, 13, 17]);
    expect(cols('Foo', { caseSensitive: false })).toEqual([0, 8, 13, 17]);
  });

  it('matches whole words and literal metacharacters', () => {
    expect(cols('foo', { wholeWord: true })).toEqual([0, 8]);
    expect(cols('a.b')).toEqual([22]);
    expect(cols('(x)')).toEqual([30]);
    expect(cols('-v', { wholeWord: true })).toEqual([34]);
  });
});
