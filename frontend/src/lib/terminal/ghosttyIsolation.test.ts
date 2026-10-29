// Why ghosttyEngine.ts gives every terminal its own WASM instance: ghostty-web 0.4.0
// does not zero memory a freed terminal hands back, so a resize of another terminal
// in the same instance shows text of the closed one (seen as random leftovers).
import { describe, expect, it } from 'vitest';
import { Ghostty, type GhosttyTerminal } from 'ghostty-web';

const rowText = (t: GhosttyTerminal, row: number) =>
  (t.getLine(row) ?? []).map((c) => (c.codepoint ? String.fromCodePoint(c.codepoint) : ' ')).join('');

/** Fills and frees a terminal in `first`, then resizes a new one in `second`; counts leftover rows. */
function leftoverRows(first: Ghostty, second: Ghostty): number {
  const old = first.createTerminal(100, 30);
  for (let i = 0; i < 2500; i++) old.write(`OLD ${i} ${'é─█'.repeat(i % 25)}\r\n`);
  old.resize(120, 40);
  old.resize(90, 25);
  old.free();

  const t = second.createTerminal(90, 25);
  let bad = 0;
  for (let i = 0; i < 260; i++) {
    t.write(`NEW ${i}\r\n`);
    if (i === 200) t.resize(110, 30);
    if (i > 200) for (let r = 0; r < 30; r++) if (rowText(t, r).includes('OLD')) bad++;
  }
  t.free();
  return bad;
}

describe('ghostty-web memory isolation', () => {
  it('a terminal in its own WASM instance never shows text of a closed one', async () => {
    expect(leftoverRows(await Ghostty.load(), await Ghostty.load())).toBe(0);
  });
});
