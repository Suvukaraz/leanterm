import { describe, expect, it } from 'vitest';
import { centerRect, clampRect, moveRect, resizeRect } from './dialogGeometry';

const vp = { w: 1000, h: 800 };
const min = { w: 300, h: 200 };

describe('clampRect', () => {
  it('keeps the dialog inside the viewport', () => {
    expect(clampRect({ x: -50, y: 900, w: 400, h: 300 }, vp, min)).toEqual({ x: 0, y: 500, w: 400, h: 300 });
  });

  it('limits the size to the viewport and the minimum', () => {
    expect(clampRect({ x: 0, y: 0, w: 2000, h: 50 }, vp, min)).toEqual({ x: 0, y: 0, w: 1000, h: 200 });
  });

  it('shrinks below the minimum only when the viewport is smaller', () => {
    expect(clampRect({ x: 10, y: 10, w: 400, h: 300 }, { w: 250, h: 150 }, min)).toEqual({ x: 0, y: 0, w: 250, h: 150 });
  });
});

describe('moveRect', () => {
  it('moves and stops at the viewport border', () => {
    const start = { x: 100, y: 100, w: 400, h: 300 };
    expect(moveRect(start, 50, -20, vp, min)).toEqual({ x: 150, y: 80, w: 400, h: 300 });
    expect(moveRect(start, 900, 900, vp, min)).toEqual({ x: 600, y: 500, w: 400, h: 300 });
  });
});

describe('resizeRect', () => {
  const start = { x: 100, y: 100, w: 400, h: 300 };

  it('resizes from the south-east corner', () => {
    expect(resizeRect(start, 'se', 60, 40, vp, min)).toEqual({ x: 100, y: 100, w: 460, h: 340 });
  });

  it('keeps the right/bottom edge fixed when resizing from the north-west', () => {
    expect(resizeRect(start, 'nw', -50, -30, vp, min)).toEqual({ x: 50, y: 70, w: 450, h: 330 });
    // Minimum size reached: left/top stop, right/bottom stay at 500/400.
    expect(resizeRect(start, 'nw', 300, 300, vp, min)).toEqual({ x: 200, y: 200, w: 300, h: 200 });
  });

  it('does not grow beyond the viewport', () => {
    expect(resizeRect(start, 'e', 2000, 0, vp, min)).toEqual({ x: 100, y: 100, w: 900, h: 300 });
    expect(resizeRect(start, 'n', 0, -500, vp, min)).toEqual({ x: 100, y: 0, w: 400, h: 400 });
  });
});

describe('centerRect', () => {
  it('centres the dialog', () => {
    expect(centerRect({ w: 400, h: 200 }, vp, min)).toEqual({ x: 300, y: 300, w: 400, h: 200 });
  });
});
