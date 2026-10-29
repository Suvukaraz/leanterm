// Geometry of movable/resizable in-window dialogs (gtk/Dialog.svelte). All values
// are CSS pixels relative to the main window's viewport.

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Size {
  w: number;
  h: number;
}

/** Edge or corner a resize starts from. */
export type ResizeEdge = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';

export const RESIZE_EDGES: ResizeEdge[] = ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'];

/**
 * Keeps a dialog fully inside the viewport: the size is limited to [min, viewport]
 * and the position so that no part (in particular the title bar) leaves the window.
 */
export function clampRect(r: Rect, viewport: Size, min: Size): Rect {
  const w = Math.round(Math.max(Math.min(min.w, viewport.w), Math.min(r.w, viewport.w)));
  const h = Math.round(Math.max(Math.min(min.h, viewport.h), Math.min(r.h, viewport.h)));
  const x = Math.round(Math.max(0, Math.min(r.x, viewport.w - w)));
  const y = Math.round(Math.max(0, Math.min(r.y, viewport.h - h)));
  return { x, y, w, h };
}

/** Moves the dialog by (dx, dy) from where the drag started. */
export function moveRect(start: Rect, dx: number, dy: number, viewport: Size, min: Size): Rect {
  return clampRect({ ...start, x: start.x + dx, y: start.y + dy }, viewport, min);
}

/**
 * Resizes from an edge/corner. The opposite edges stay where they are, also when
 * the minimum size or the viewport border stops the drag.
 */
export function resizeRect(start: Rect, edge: ResizeEdge, dx: number, dy: number, viewport: Size, min: Size): Rect {
  let left = start.x;
  let top = start.y;
  let right = start.x + start.w;
  let bottom = start.y + start.h;

  if (edge.includes('w')) left = Math.max(0, Math.min(left + dx, right - min.w));
  if (edge.includes('e')) right = Math.min(viewport.w, Math.max(right + dx, left + min.w));
  if (edge.includes('n')) top = Math.max(0, Math.min(top + dy, bottom - min.h));
  if (edge.includes('s')) bottom = Math.min(viewport.h, Math.max(bottom + dy, top + min.h));

  return clampRect({ x: left, y: top, w: right - left, h: bottom - top }, viewport, min);
}

/** Centres a dialog of the given size in the viewport. */
export function centerRect(size: Size, viewport: Size, min: Size): Rect {
  return clampRect(
    { x: (viewport.w - size.w) / 2, y: (viewport.h - size.h) / 2, w: size.w, h: size.h },
    viewport,
    min
  );
}
