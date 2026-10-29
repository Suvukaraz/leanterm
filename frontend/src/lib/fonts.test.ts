import { describe, expect, it } from 'vitest';
import { defaultStyle, fontFaceCss, fontFeaturesCss, fontLabel, normalizeFeatures, parseStyle, sortStyles } from './fonts';

describe('font styles', () => {
  it('parses fontconfig style names', () => {
    expect(parseStyle('Regular')).toEqual({ weight: 400, italic: false, stretch: '' });
    expect(parseStyle('Bold Italic')).toEqual({ weight: 700, italic: true, stretch: '' });
    expect(parseStyle('ExtraBold')).toMatchObject({ weight: 800 });
    expect(parseStyle('SemiBold')).toMatchObject({ weight: 600 });
    expect(parseStyle('ExtraLight Oblique')).toEqual({ weight: 200, italic: true, stretch: '' });
    expect(parseStyle('Condensed Bold')).toEqual({ weight: 700, italic: false, stretch: 'condensed' });
    expect(parseStyle('')).toEqual({ weight: 400, italic: false, stretch: '' });
  });

  it('sorts styles and picks the regular face', () => {
    expect(sortStyles(['Bold Italic', 'Bold', 'Italic', 'Light', 'Regular'])).toEqual([
      'Light', 'Regular', 'Italic', 'Bold', 'Bold Italic',
    ]);
    expect(defaultStyle(['Bold', 'Book', 'Oblique'])).toBe('Book');
    expect(defaultStyle([])).toBe('Regular');
  });
});

describe('font features', () => {
  it('maps the feature list to font-feature-settings', () => {
    expect(fontFeaturesCss('liga, calt')).toBe('"liga" 1, "calt" 1');
    expect(fontFeaturesCss('-liga,ss01=2 +zero')).toBe('"liga" 0, "ss01" 2, "zero" 1');
    expect(fontFeaturesCss('bogus, x')).toBe('normal');
    expect(fontFeaturesCss('')).toBe('normal');
    expect(normalizeFeatures(' liga,calt;; bad ')).toBe('liga, calt');
  });

  it('builds CSS and labels', () => {
    expect(fontFaceCss({ Family: 'Hack', Size: 11, Style: 'Bold Italic', Features: 'liga' })).toBe(
      'font-weight: 700; font-style: italic; font-stretch: normal; font-feature-settings: "liga" 1'
    );
    expect(fontLabel({ Family: 'Hack', Size: 11, Style: 'Regular' })).toBe('Hack 11');
    expect(fontLabel({ Family: 'Hack', Size: 11, Style: 'Bold' })).toBe('Hack Bold 11');
    expect(fontLabel({ Family: '', Size: 0 })).toBe('(Default system font)');
  });
});
