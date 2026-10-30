// Maps LeanTerm terminal look settings ([terminal.look], optionally overridden per
// connection) to engine-neutral terminal options (Rio and ghostty-web).
import type { FontSetting } from '../fonts';

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

/** Font sizes are stored in points (like GTK/Pango); the engines want CSS pixels. */
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

/** Colours shared by the engines (xterm.js-style names, as used by Rio and ghostty-web). */
export interface TerminalTheme {
  foreground: string;
  background: string;
  cursor: string;
  cursorAccent: string;
  selectionForeground: string;
  selectionBackground: string;
  black: string;
  red: string;
  green: string;
  yellow: string;
  blue: string;
  magenta: string;
  cyan: string;
  white: string;
  brightBlack: string;
  brightRed: string;
  brightGreen: string;
  brightYellow: string;
  brightBlue: string;
  brightMagenta: string;
  brightCyan: string;
  brightWhite: string;
}

export type CursorStyle = 'block' | 'bar' | 'underline';

// Selection highlight, blended over the background: both engines paint on an
// opaque canvas, where a translucent fill would pile up frame after frame.
const SELECTION_TINT = { r: 120, g: 160, b: 220, a: 0.45 };

function parseHex(color: string): [number, number, number] | undefined {
  const m = /^#([0-9a-f]{6})$/i.exec(color.trim());
  if (!m) return undefined;
  const v = parseInt(m[1], 16);
  return [(v >> 16) & 0xff, (v >> 8) & 0xff, v & 0xff];
}

function hex(r: number, g: number, b: number): string {
  return '#' + [r, g, b].map((c) => Math.round(c).toString(16).padStart(2, '0')).join('');
}

/** The selection colour over `background` as an opaque #rrggbb. */
export function selectionColor(background: string): string {
  const [r, g, b] = parseHex(background) ?? [0, 0, 0];
  const { a } = SELECTION_TINT;
  return hex(
    SELECTION_TINT.r * a + r * (1 - a),
    SELECTION_TINT.g * a + g * (1 - a),
    SELECTION_TINT.b * a + b * (1 - a)
  );
}

export function buildTheme(look: TerminalLook): TerminalTheme {
  const p = look.Palette && look.Palette.length === 16 ? look.Palette : DEFAULT_PALETTE;
  const fg = look.Foreground || '#cccccc';
  const bg = look.Background || '#000000';
  return {
    foreground: fg,
    background: bg,
    cursor: fg,
    cursorAccent: bg,
    selectionForeground: fg,
    selectionBackground: selectionColor(bg),
    black: p[0], red: p[1], green: p[2], yellow: p[3],
    blue: p[4], magenta: p[5], cyan: p[6], white: p[7],
    brightBlack: p[8], brightRed: p[9], brightGreen: p[10], brightYellow: p[11],
    brightBlue: p[12], brightMagenta: p[13], brightCyan: p[14], brightWhite: p[15],
  };
}

export function cursorStyle(look: TerminalLook): CursorStyle {
  return look.CursorShape === 'ibeam' ? 'bar' : look.CursorShape === 'underline' ? 'underline' : 'block';
}

/** Font, size, cursor and scrollback for a new terminal. */
export function buildTerminalOptions(look: TerminalLook): {
  fontFamily: string;
  fontSize: number;
  cursorStyle: CursorStyle;
  scrollback: number;
} {
  const pt = look.Font?.Size && look.Font.Size > 0 ? look.Font.Size : DEFAULT_FONT_PT;
  const scrollback = look.ScrollbackLines ?? 5000;
  return {
    fontFamily: fontFamilyCss(look.Font?.Family),
    fontSize: ptToPx(pt),
    cursorStyle: cursorStyle(look),
    scrollback: scrollback < 0 ? 100000 : scrollback,
  };
}
