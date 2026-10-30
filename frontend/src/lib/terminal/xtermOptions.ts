// Maps LeanTerm terminal look settings ([terminal.look], optionally overridden per
// connection) to xterm.js options.
import type { FontWeight, ITerminalOptions, ITheme } from '@xterm/xterm';
import { parseStyle, type FontSetting } from '../fonts';

export interface TerminalLook {
  Font?: FontSetting;
  Foreground?: string;
  Background?: string;
  Bold?: string;
  BoldLikeText?: boolean;
  BoldIsBright?: boolean;
  CursorShape?: string;
  ScrollbackLines?: number;
  Transparency?: number;
  WordChars?: string;
  Palette?: string[];
}

export const DEFAULT_FONT_PT = 11;
// Linux fonts first, then the Windows ones (Cascadia Mono since Windows 11, Consolas before).
const MONO_FALLBACK =
  '"DejaVu Sans Mono", "Ubuntu Mono", "Noto Sans Mono", "Liberation Mono", "Cascadia Mono", Consolas, "Courier New", monospace';

const DEFAULT_PALETTE = [
  '#000000', '#cc0000', '#4e9a06', '#c4a000', '#3465a4', '#75507b', '#06989a', '#d3d7cf',
  '#555753', '#ef2929', '#8ae234', '#fce94f', '#729fcf', '#ad7fa8', '#34e2e2', '#eeeeec',
];

/** Font sizes are stored in points (like GTK/Pango); xterm.js wants CSS pixels. */
export function ptToPx(pt: number): number {
  return Math.round(pt * (96 / 72) * 10) / 10;
}

/** Builds a CSS font-family list; generic names like "Monospace" map to the fallback stack. */
export function fontFamilyCss(family: string | undefined): string {
  const f = (family ?? '').trim();
  if (!f || /^mono(space)?$/i.test(f)) return MONO_FALLBACK;
  return `"${f.replace(/"/g, '')}", ${MONO_FALLBACK}`;
}

/** Merges a connection's personal look over the global one (missing keys inherit). */
export function mergeLook(global: TerminalLook | undefined, personal: TerminalLook | undefined | null): TerminalLook {
  const g = global ?? {};
  if (!personal) return g;
  const out: TerminalLook = { ...g };
  for (const [k, v] of Object.entries(personal)) {
    if (v === undefined || v === null || v === '') continue;
    if (k === 'Font') {
      const font = v as TerminalLook['Font'];
      // Style and features belong to the family they were chosen for.
      const own = !!font?.Family;
      out.Font = {
        Family: font?.Family || g.Font?.Family,
        Size: font?.Size && font.Size > 0 ? font.Size : g.Font?.Size,
        Style: own ? font?.Style : g.Font?.Style,
        Features: own ? font?.Features : g.Font?.Features,
      };
    } else if (k === 'Palette') {
      if (Array.isArray(v) && v.length === 16) out.Palette = v as string[];
    } else if (k === 'ScrollbackLines') {
      if (typeof v === 'number' && v !== -2) out.ScrollbackLines = v;
    } else {
      (out as any)[k] = v;
    }
  }
  return out;
}

function withAlpha(hex: string, alpha: number): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m || alpha >= 1) return hex;
  const a = Math.round(Math.max(0, Math.min(1, alpha)) * 255)
    .toString(16)
    .padStart(2, '0');
  return `#${m[1]}${a}`;
}

export function buildTheme(look: TerminalLook): ITheme {
  const p = look.Palette && look.Palette.length === 16 ? look.Palette : DEFAULT_PALETTE;
  const fg = look.Foreground || '#cccccc';
  const bg = look.Background || '#000000';
  const transparency = look.Transparency ?? 0;
  return {
    foreground: fg,
    background: withAlpha(bg, 1 - transparency),
    cursor: fg,
    cursorAccent: bg,
    selectionBackground: 'rgba(120, 160, 220, 0.45)',
    black: p[0], red: p[1], green: p[2], yellow: p[3],
    blue: p[4], magenta: p[5], cyan: p[6], white: p[7],
    brightBlack: p[8], brightRed: p[9], brightGreen: p[10], brightYellow: p[11],
    brightBlue: p[12], brightMagenta: p[13], brightCyan: p[14], brightWhite: p[15],
  };
}

/** xterm wants separator characters; the setting lists characters that belong to words. */
export function wordSeparator(wordChars: string | undefined): string {
  const keep = new Set((wordChars ?? '-.:_/').split(''));
  const candidates = ' ()[]{}\'"`,;<>|=&!?*@#$%^+~\\\t';
  return candidates.split('').filter((c) => !keep.has(c)).join('');
}

/** Normal and bold weight for xterm.js from the font style (xterm has no italic/stretch option). */
export function fontWeights(font: FontSetting | undefined): { fontWeight: FontWeight; fontWeightBold: FontWeight } {
  const { weight } = parseStyle(font?.Style);
  return { fontWeight: weight as FontWeight, fontWeightBold: Math.min(900, Math.max(700, weight + 300)) as FontWeight };
}

export function buildTerminalOptions(look: TerminalLook): ITerminalOptions {
  const pt = look.Font?.Size && look.Font.Size > 0 ? look.Font.Size : DEFAULT_FONT_PT;
  const cursor = look.CursorShape === 'ibeam' ? 'bar' : look.CursorShape === 'underline' ? 'underline' : 'block';
  const scrollback = look.ScrollbackLines ?? 5000;
  return {
    fontFamily: fontFamilyCss(look.Font?.Family),
    fontSize: ptToPx(pt),
    ...fontWeights(look.Font),
    lineHeight: 1.0,
    cursorBlink: true,
    cursorStyle: cursor,
    scrollback: scrollback < 0 ? 100000 : scrollback,
    drawBoldTextInBrightColors: !!look.BoldIsBright,
    allowTransparency: (look.Transparency ?? 0) > 0,
    wordSeparator: wordSeparator(look.WordChars),
    theme: buildTheme(look),
    allowProposedApi: true,
    macOptionIsMeta: true,
    smoothScrollDuration: 0,
    // Right click opens LeanTerm's own menu; selecting a word there only causes
    // an extra repaint (visible as flicker) and differs from the GTK original.
    rightClickSelectsWord: false,
  };
}
