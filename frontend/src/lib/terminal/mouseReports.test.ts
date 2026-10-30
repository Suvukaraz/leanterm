import { describe, expect, it } from 'vitest';
import { MouseModes, encodeMouseReport, type MouseReport } from './mouseReports';

const bytes = (s: string) => new TextEncoder().encode(s);
const text = (b: Uint8Array | undefined) => (b ? String.fromCharCode(...b) : undefined);
const press = (o: Partial<MouseReport> = {}): MouseReport => ({
  kind: 'press',
  button: 0,
  col: 4,
  row: 1,
  shift: false,
  alt: false,
  ctrl: false,
  ...o,
});

describe('MouseModes', () => {
  it('tracks DECSET/DECRST mouse modes, also split across chunks', () => {
    const m = new MouseModes();
    m.feed(bytes('hello \x1b[?100'));
    expect(m.tracking).toBe('none');
    m.feed(bytes('2;1006h'));
    expect(m.tracking).toBe('button');
    expect(m.sgr).toBe(true);
    m.feed(bytes('\x1b[?1000l'));
    expect(m.tracking).toBe('button');
    m.feed(bytes('\x1b[?1002l\x1b[?1006l'));
    expect(m.tracking).toBe('none');
    expect(m.sgr).toBe(false);
  });

  it('ignores non-private modes and resets on RIS', () => {
    const m = new MouseModes();
    m.feed(bytes('\x1b[1000h'));
    expect(m.tracking).toBe('none');
    m.feed(bytes('\x1b[?1003h\x1b[?1006h'));
    expect(m.tracking).toBe('any');
    m.feed(bytes('\x1bc'));
    expect(m.tracking).toBe('none');
    expect(m.sgr).toBe(false);
  });
});

describe('encodeMouseReport', () => {
  it('sends nothing without tracking', () => {
    expect(encodeMouseReport(new MouseModes(), press())).toBeUndefined();
  });

  it('encodes SGR press, release and drag', () => {
    const m = new MouseModes();
    m.feed(bytes('\x1b[?1002h\x1b[?1006h'));
    expect(text(encodeMouseReport(m, press()))).toBe('\x1b[<0;5;2M');
    expect(text(encodeMouseReport(m, press({ kind: 'release', button: 2 })))).toBe('\x1b[<2;5;2m');
    expect(text(encodeMouseReport(m, press({ kind: 'motion', ctrl: true })))).toBe('\x1b[<48;5;2M');
    expect(encodeMouseReport(m, press({ kind: 'motion', button: -1 }))).toBeUndefined();
  });

  it('encodes legacy reports as raw bytes', () => {
    const m = new MouseModes();
    m.feed(bytes('\x1b[?1000h'));
    expect(Array.from(encodeMouseReport(m, press({ col: 250, row: 0 }))!)).toEqual([0x1b, 0x5b, 0x4d, 32, 255, 33]);
    expect(text(encodeMouseReport(m, press({ kind: 'release', button: 1 })))).toBe('\x1b[M#%"');
    expect(encodeMouseReport(m, press({ kind: 'motion' }))).toBeUndefined();
  });

  it('X10 mode reports presses only, without modifiers', () => {
    const m = new MouseModes();
    m.feed(bytes('\x1b[?9h'));
    expect(text(encodeMouseReport(m, press({ shift: true })))).toBe('\x1b[M %"');
    expect(encodeMouseReport(m, press({ kind: 'release' }))).toBeUndefined();
  });
});
