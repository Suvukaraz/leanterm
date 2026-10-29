import { describe, it, expect } from 'vitest';
import { ptToPx, fontFamilyCss, mergeLook, wordSeparator, buildTerminalOptions } from './xtermOptions';

describe('xtermOptions', () => {
  it('converts points to CSS pixels', () => {
    expect(ptToPx(9)).toBe(12);
    expect(ptToPx(11)).toBeCloseTo(14.7, 1);
  });

  it('maps generic monospace names to a fallback stack', () => {
    expect(fontFamilyCss('Monospace')).toContain('monospace');
    expect(fontFamilyCss('Fira Code').startsWith('"Fira Code"')).toBe(true);
  });

  it('lets personal looks override only the keys they set', () => {
    const merged = mergeLook(
      { Font: { Family: 'Monospace', Size: 11 }, Foreground: '#cccccc', ScrollbackLines: 5000 },
      { Font: { Family: '', Size: 14 }, Foreground: '', ScrollbackLines: -2 }
    );
    expect(merged.Font).toEqual({ Family: 'Monospace', Size: 14 });
    expect(merged.Foreground).toBe('#cccccc');
    expect(merged.ScrollbackLines).toBe(5000);
  });

  it('keeps style and features with the family they were chosen for', () => {
    const global = { Font: { Family: 'Hack', Size: 11, Style: 'Bold', Features: 'liga' } };
    expect(mergeLook(global, { Font: { Family: '', Size: 14 } }).Font).toEqual({ Family: 'Hack', Size: 14, Style: 'Bold', Features: 'liga' });
    expect(mergeLook(global, { Font: { Family: 'Fira Code', Size: 0, Style: '' } }).Font).toEqual({
      Family: 'Fira Code',
      Size: 11,
      Style: '',
    });
  });

  it('maps the font style to xterm weights', () => {
    const opts = buildTerminalOptions({ Font: { Family: 'Hack', Size: 11, Style: 'Light' } });
    expect(opts.fontWeight).toBe(300);
    expect(opts.fontWeightBold).toBe(700);
    expect(buildTerminalOptions({ Font: { Style: 'Bold' } }).fontWeightBold).toBe(900);
  });

  it('treats configured word characters as part of words', () => {
    const sep = wordSeparator('-.:_/');
    expect(sep).not.toContain('-');
    expect(sep).not.toContain('/');
    expect(sep).toContain(' ');
  });

  it('builds options with the configured font size in pixels', () => {
    const opts = buildTerminalOptions({ Font: { Family: 'Monospace', Size: 12 }, ScrollbackLines: -1 });
    expect(opts.fontSize).toBe(16);
    expect(opts.scrollback).toBe(100000);
  });
});
