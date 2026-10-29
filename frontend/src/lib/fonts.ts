// Font settings shared by all font choosers ({ Family, Size, Style, Features } like
// the Go FontConfig): installed fonts, style names → CSS and OpenType features.
import * as SystemService from '../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

export interface FontSetting {
  Family?: string;
  /** Points. */
  Size?: number;
  /** Fontconfig style name, e.g. "Bold", "Light Italic" ("" = Regular). */
  Style?: string;
  /** Comma-separated OpenType features, e.g. "liga, calt, -dlig, ss01=2". */
  Features?: string;
}

export interface FontFamily {
  name: string;
  styles: string[];
  monospace: boolean;
}

export interface ParsedStyle {
  weight: number;
  italic: boolean;
  /** CSS font-stretch keyword, or "" for normal width. */
  stretch: string;
}

// Longest names first so that "ExtraBold" is not read as "Bold".
const WEIGHTS: Array<[RegExp, number]> = [
  [/(extra|ultra)[\s-]?light/, 200],
  [/(extra|ultra)[\s-]?bold/, 800],
  [/(semi|demi)[\s-]?bold/, 600],
  [/hairline|thin/, 100],
  [/light/, 300],
  [/medium/, 500],
  [/black|heavy/, 900],
  [/bold/, 700],
];

const STRETCHES: Array<[RegExp, string]> = [
  [/(extra|ultra)[\s-]?condensed/, 'extra-condensed'],
  [/(semi|demi)[\s-]?condensed/, 'semi-condensed'],
  [/condensed|narrow/, 'condensed'],
  [/(extra|ultra)[\s-]?expanded/, 'extra-expanded'],
  [/(semi|demi)[\s-]?expanded/, 'semi-expanded'],
  [/expanded|wide/, 'expanded'],
];

export function parseStyle(style: string | undefined): ParsedStyle {
  const s = (style ?? '').toLowerCase();
  const weight = WEIGHTS.find(([re]) => re.test(s))?.[1] ?? 400;
  const stretch = STRETCHES.find(([re]) => re.test(s))?.[1] ?? '';
  return { weight, italic: /italic|oblique|kursiv/.test(s), stretch };
}

/** Sorts style names like a font dialog: by width, weight, then upright before italic. */
export function sortStyles(styles: string[]): string[] {
  const order = ['extra-condensed', 'condensed', 'semi-condensed', '', 'semi-expanded', 'expanded', 'extra-expanded'];
  return [...styles].sort((a, b) => {
    const pa = parseStyle(a);
    const pb = parseStyle(b);
    return (
      order.indexOf(pa.stretch) - order.indexOf(pb.stretch) ||
      pa.weight - pb.weight ||
      Number(pa.italic) - Number(pb.italic) ||
      a.localeCompare(b)
    );
  });
}

/** The "Regular" face of a family (or the closest one) when the chosen style is not available. */
export function defaultStyle(styles: string[]): string {
  if (styles.length === 0) return 'Regular';
  return sortStyles(styles).reduce((best, s) => {
    const p = parseStyle(s);
    const q = parseStyle(best);
    const score = (x: ParsedStyle) => Math.abs(x.weight - 400) + (x.italic ? 1000 : 0) + (x.stretch ? 500 : 0);
    return score(p) < score(q) ? s : best;
  });
}

/**
 * OpenType features as CSS font-feature-settings ("normal" when empty).
 * Accepts "liga", "+liga", "-liga", "liga=0", "ss01=2"; invalid entries are skipped.
 */
export function fontFeaturesCss(features: string | undefined): string {
  const out: string[] = [];
  for (const raw of (features ?? '').split(/[,;\s]+/)) {
    const m = /^([+-]?)([A-Za-z0-9]{4})(?:=(\d+))?$/.exec(raw.trim());
    if (!m) continue;
    const value = m[1] === '-' ? 0 : m[3] !== undefined ? Number(m[3]) : 1;
    out.push(`"${m[2]}" ${value}`);
  }
  return out.length ? out.join(', ') : 'normal';
}

/** Normalises user input in the features field ("liga,calt" → "liga, calt"). */
export function normalizeFeatures(features: string): string {
  return features
    .split(/[,;\s]+/)
    .map((f) => f.trim())
    .filter((f) => /^[+-]?[A-Za-z0-9]{4}(=\d+)?$/.test(f))
    .join(', ');
}

/** CSS declarations for weight, style, stretch and features of a font setting. */
export function fontFaceCss(font: FontSetting | null | undefined): string {
  const p = parseStyle(font?.Style);
  return [
    `font-weight: ${p.weight}`,
    `font-style: ${p.italic ? 'italic' : 'normal'}`,
    `font-stretch: ${p.stretch || 'normal'}`,
    `font-feature-settings: ${fontFeaturesCss(font?.Features)}`,
  ].join('; ');
}

/** Button label like KDE's font requester: "Hack Bold 11" (Regular is left out). */
export function fontLabel(font: FontSetting | null | undefined): string {
  if (!font?.Family) return '(Default system font)';
  const style = font.Style && !/^(regular|book|normal|roman)$/i.test(font.Style) ? ` ${font.Style}` : '';
  return `${font.Family}${style}${font.Size ? ` ${font.Size}` : ''}`;
}

// Used when fontconfig is not available (e.g. browser preview).
const FALLBACK: FontFamily[] = [
  ...['Monospace', 'DejaVu Sans Mono', 'Ubuntu Mono', 'Noto Sans Mono', 'Liberation Mono', 'Fira Code', 'JetBrains Mono',
    'Hack', 'Source Code Pro', 'Cascadia Code', 'Courier New'].map((name) => ({ name, styles: ['Regular', 'Bold', 'Italic', 'Bold Italic'], monospace: true })),
  ...['Sans', 'Serif', 'Cantarell', 'Ubuntu', 'Noto Sans', 'DejaVu Sans', 'Liberation Sans', 'Inter', 'Roboto'].map((name) => ({
    name,
    styles: ['Regular', 'Bold', 'Italic', 'Bold Italic'],
    monospace: false,
  })),
].sort((a, b) => a.name.localeCompare(b.name));

// Fontconfig aliases that are always valid family names.
const GENERIC: FontFamily[] = [
  { name: 'Monospace', styles: ['Regular', 'Bold', 'Italic', 'Bold Italic'], monospace: true },
  { name: 'Sans', styles: ['Regular', 'Bold', 'Italic', 'Bold Italic'], monospace: false },
  { name: 'Serif', styles: ['Regular', 'Bold', 'Italic', 'Bold Italic'], monospace: false },
];

let cache: Promise<FontFamily[]> | undefined;

/** Installed font families (fontconfig), loaded once per window. */
export function listFonts(): Promise<FontFamily[]> {
  cache ??= Promise.resolve()
    .then(() => SystemService.ListFonts())
    .then((list) => {
      const families = (list ?? []).filter((f) => f?.name) as FontFamily[];
      if (families.length === 0) return FALLBACK;
      const names = new Set(families.map((f) => f.name.toLowerCase()));
      return [...GENERIC.filter((g) => !names.has(g.name.toLowerCase())), ...families].sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
      );
    })
    .catch(() => {
      cache = undefined;
      return FALLBACK;
    });
  return cache;
}
