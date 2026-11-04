import { describe, it, expect, vi, beforeEach } from 'vitest';

// Resize calls that are still "in the backend"; each one finishes when resolved.
const pending = vi.hoisted(() => [] as Array<{ size: string; done: () => void }>);
const session = vi.hoisted(() => ({
  Resize: vi.fn(
    (_tmpID: string, cols: number, rows: number) =>
      new Promise<void>((resolve) => pending.push({ size: `${cols}x${rows}`, done: resolve }))
  ),
}));

vi.mock('../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js', () => session);
vi.mock('@wailsio/runtime', () => ({ Events: { On: () => () => {} } }));

import { TerminalStream } from './terminalStream';

async function settle() {
  for (let i = 0; i < 5; i++) await Promise.resolve();
}

describe('TerminalStream.resize', () => {
  beforeEach(() => {
    pending.length = 0;
    vi.clearAllMocks();
  });

  it('sends one size at a time and ends with the newest one', async () => {
    const s = new TerminalStream('t1', { write: () => {} });
    s.resize(80, 24);
    s.resize(70, 24); // dragging the window: more sizes while the first call runs
    s.resize(62, 24);
    expect(session.Resize).toHaveBeenCalledTimes(1);

    pending.shift()!.done();
    await settle();
    // The intermediate 70x24 is skipped, the newest size follows.
    expect(session.Resize).toHaveBeenCalledTimes(2);
    expect(pending.map((p) => p.size)).toEqual(['62x24']);

    pending.shift()!.done();
    await settle();
    expect(session.Resize).toHaveBeenCalledTimes(2);
    expect(session.Resize.mock.calls.at(-1)).toEqual(['t1', 62, 24]);
  });

  it('ignores empty sizes and sizes after destroy', async () => {
    const s = new TerminalStream('t2', { write: () => {} });
    s.resize(0, 24);
    s.destroy();
    s.resize(80, 24);
    await settle();
    expect(session.Resize).not.toHaveBeenCalled();
  });
});
