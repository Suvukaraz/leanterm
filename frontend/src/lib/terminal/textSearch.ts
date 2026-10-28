// Plain-text search over terminal rows, used by engines without a search addon
// (ghostty-web). Rows are searched one by one; matches never span rows.
import type { SearchOptions } from './engine';

/** One terminal row as text; start/end give the cell columns covered by text[i]. */
export interface SearchRow {
  text: string;
  start: number[];
  end: number[];
}

/** A match in absolute buffer coordinates; endCol is inclusive. */
export interface SearchMatch {
  row: number;
  col: number;
  endCol: number;
}

export interface SearchPosition {
  row: number;
  col: number;
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** Builds the matcher for the find bar options; undefined for an empty or invalid pattern. */
export function buildMatcher(text: string, opts: SearchOptions): RegExp | undefined {
  if (!text) return undefined;
  let source = opts.regex ? text : escapeRegExp(text);
  if (opts.wholeWord) source = `(?<![\\w])(?:${source})(?![\\w])`;
  try {
    return new RegExp(source, opts.caseSensitive ? 'g' : 'gi');
  } catch {
    return undefined;
  }
}

function rowMatches(row: SearchRow, rowIndex: number, re: RegExp): SearchMatch[] {
  const out: SearchMatch[] = [];
  re.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(row.text)) !== null) {
    if (m[0].length === 0) {
      re.lastIndex++;
      continue;
    }
    out.push({ row: rowIndex, col: row.start[m.index], endCol: row.end[m.index + m[0].length - 1] });
  }
  return out;
}

/**
 * Finds the next (or previous) match after (before) `from`, wrapping around the
 * buffer once. Forward search with col -1 includes matches at the start of `from.row`.
 */
export function findMatch(
  getRow: (row: number) => SearchRow | undefined,
  rowCount: number,
  re: RegExp,
  from: SearchPosition,
  backward: boolean
): SearchMatch | undefined {
  if (rowCount <= 0) return undefined;
  const startRow = Math.max(0, Math.min(rowCount - 1, from.row));
  for (let step = 0; step <= rowCount; step++) {
    const r = backward
      ? (((startRow - step) % rowCount) + rowCount) % rowCount
      : (startRow + step) % rowCount;
    const row = getRow(r);
    if (!row) continue;
    let matches = rowMatches(row, r, re);
    if (step === 0) {
      matches = matches.filter((m) => (backward ? m.col < from.col : m.col > from.col));
    } else if (step === rowCount) {
      // Back at the start row after wrapping: the part skipped in step 0.
      matches = matches.filter((m) => (backward ? m.col >= from.col : m.col <= from.col));
    }
    if (matches.length > 0) return backward ? matches[matches.length - 1] : matches[0];
  }
  return undefined;
}
