// KWrite-style scrollbar minimap: the right edge shows the document as small
// coloured blocks (one per character, in the syntax colours) with the visible
// part marked. It replaces the vertical scrollbar: click to jump, drag the
// marked area to scroll, the wheel scrolls the editor.
import { EditorView, ViewPlugin, type PluginValue, type ViewUpdate } from '@codemirror/view';
import type { Extension } from '@codemirror/state';
import { highlightingFor, syntaxTree } from '@codemirror/language';
import { highlightTree, type Highlighter } from '@lezer/highlight';

const WIDTH = 80;
const PAD = 4;
/** Pixels per line and per character column in the minimap. */
const LINE = 3;
const CHAR = 1;
const MAX_COLS = Math.floor((WIDTH - 2 * PAD) / CHAR);

interface Geometry {
  /** Height of the minimap in CSS pixels. */
  height: number;
  lines: number;
  /** First visible line (0-based, fractional) and number of visible lines. */
  top: number;
  visible: number;
  /** Minimap scroll offset in pixels (when the document is taller than the minimap). */
  offset: number;
}

class Minimap implements PluginValue {
  dom: HTMLDivElement;
  canvas: HTMLCanvasElement;
  slider: HTMLDivElement;
  probe: HTMLSpanElement;
  colors = new Map<string, string>();
  geo: Geometry = { height: 0, lines: 1, top: 0, visible: 0, offset: 0 };
  drag: { y: number; top: number } | null = null;
  /** The text graphics must be redrawn (document, colours or selection changed). */
  dirty = true;
  measureReq: { key: Minimap; read: () => Geometry & { mapTop: number }; write: (m: Geometry & { mapTop: number }) => void };

  constructor(readonly view: EditorView) {
    this.dom = document.createElement('div');
    this.dom.className = 'cm-minimap';
    this.dom.setAttribute('aria-hidden', 'true');
    this.canvas = document.createElement('canvas');
    this.slider = document.createElement('div');
    this.slider.className = 'cm-minimap-slider';
    this.probe = document.createElement('span');
    this.probe.style.display = 'none';
    this.dom.append(this.canvas, this.slider, this.probe);
    view.dom.appendChild(this.dom);

    this.measureReq = { key: this, read: () => this.read(), write: (m) => this.write(m) };
    this.dom.addEventListener('mousedown', this.onMouseDown);
    this.dom.addEventListener('wheel', this.onWheel, { passive: false });
    view.scrollDOM.addEventListener('scroll', this.schedule, { passive: true });
    this.schedule();
  }

  update(u: ViewUpdate) {
    const reconfigured = u.transactions.some((tr) => tr.reconfigured);
    if (reconfigured) this.colors.clear();
    const content = reconfigured || u.docChanged || u.selectionSet || syntaxTree(u.startState) !== syntaxTree(u.state);
    if (content) this.dirty = true;
    if (content || u.viewportChanged || u.geometryChanged || u.heightChanged) this.schedule();
  }

  destroy() {
    this.endDrag();
    this.view.scrollDOM.removeEventListener('scroll', this.schedule);
    this.dom.remove();
  }

  schedule = () => this.view.requestMeasure(this.measureReq);

  // ---- Geometry -------------------------------------------------------------------------------

  /** Document line (0-based, fractional) at a height in document coordinates. */
  lineAtHeight(h: number): number {
    const { view } = this;
    const doc = view.state.doc;
    const block = view.lineBlockAtHeight(Math.max(0, h));
    const first = doc.lineAt(block.from).number;
    const last = doc.lineAt(block.to).number;
    const frac = block.height > 0 ? Math.min(1, Math.max(0, (h - block.top) / block.height)) : 0;
    return first - 1 + frac * (last - first + 1);
  }

  /** Height in document coordinates of a (fractional, 0-based) line. */
  heightOfLine(line: number): number {
    const doc = this.view.state.doc;
    const n = Math.min(doc.lines - 1, Math.max(0, Math.floor(line)));
    const block = this.view.lineBlockAt(doc.line(n + 1).from);
    return block.top + Math.max(0, line - n) * block.height;
  }

  /** Top of the visible area in document coordinates. */
  visibleTop(): number {
    return this.view.scrollDOM.getBoundingClientRect().top - this.view.documentTop;
  }

  read(): Geometry & { mapTop: number } {
    const scroller = this.view.scrollDOM;
    const lines = this.view.state.doc.lines;
    const height = scroller.offsetHeight;
    const visTop = this.visibleTop();
    const top = this.lineAtHeight(visTop);
    const visible = Math.max(1, this.lineAtHeight(visTop + scroller.clientHeight) - top);
    const total = lines * LINE;
    let offset = 0;
    if (total > height) {
      offset = (top / Math.max(1, lines - visible)) * (total - height);
      offset = Math.min(total - height, Math.max(0, offset));
    }
    return { height, lines, top, visible, offset, mapTop: scroller.offsetTop };
  }

  write(m: Geometry & { mapTop: number }) {
    const prev = this.geo;
    this.geo = m;
    if (m.height <= 0) return; // hidden tab
    // Only touch what changed: style writes during scrolling cost layout work.
    const top = `${m.mapTop}px`;
    const height = `${m.height}px`;
    if (this.dom.style.top !== top) this.dom.style.top = top;
    if (this.dom.style.height !== height) this.dom.style.height = height;
    this.slider.style.transform = `translateY(${m.top * LINE - m.offset}px)`;
    const sliderHeight = `${Math.max(LINE * 2, m.visible * LINE)}px`;
    if (this.slider.style.height !== sliderHeight) this.slider.style.height = sliderHeight;
    if (this.dirty || prev.offset !== m.offset || prev.height !== m.height || prev.lines !== m.lines) {
      this.dirty = false;
      this.draw();
    }
  }

  // ---- Drawing --------------------------------------------------------------------------------

  colorOf(cls: string): string {
    let color = this.colors.get(cls);
    if (color === undefined) {
      this.probe.className = cls;
      color = getComputedStyle(this.probe).color;
      this.colors.set(cls, color);
    }
    return color;
  }

  draw() {
    const { view, canvas, geo } = this;
    const { state } = view;
    const doc = state.doc;
    const dpr = window.devicePixelRatio || 1;
    const w = Math.round(WIDTH * dpr);
    const h = Math.round(geo.height * dpr);
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
    canvas.style.width = `${WIDTH}px`;
    canvas.style.height = `${geo.height}px`;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, WIDTH, geo.height);

    const first = Math.floor(geo.offset / LINE);
    const last = Math.min(doc.lines, Math.ceil((geo.offset + geo.height) / LINE));
    if (last <= first) return;
    const from = doc.line(first + 1).from;
    const to = doc.line(last).to;
    const yOf = (lineIdx: number) => lineIdx * LINE - geo.offset;

    // Selections as a background tint.
    const selColor = getComputedStyle(this.dom).getPropertyValue('--gtk-editor-selection').trim();
    if (selColor) {
      ctx.fillStyle = selColor;
      for (const r of state.selection.ranges) {
        if (r.empty || r.to < from || r.from > to) continue;
        const a = doc.lineAt(Math.max(r.from, from)).number - 1;
        const b = doc.lineAt(Math.min(r.to, to)).number;
        ctx.fillRect(0, yOf(a), WIDTH, (b - a) * LINE);
      }
    }

    // Syntax colours of the drawn range (only as far as the parser got).
    const spans: Array<{ from: number; to: number; cls: string }> = [];
    const highlighter: Highlighter = { style: (tags) => highlightingFor(state, tags) };
    highlightTree(syntaxTree(state), highlighter, (f, t, cls) => spans.push({ from: f, to: t, cls }), from, to);

    const textColor = getComputedStyle(view.contentDOM).color;
    const tabSize = state.tabSize;
    let si = 0;
    for (let i = first; i < last; i++) {
      const line = doc.line(i + 1);
      const text = line.text.length > MAX_COLS * 4 ? line.text.slice(0, MAX_COLS * 4) : line.text;
      const y = yOf(i);
      let col = 0;
      let runStart = -1;
      let runColor = '';
      const flush = (end: number) => {
        if (runStart < 0) return;
        ctx.fillStyle = runColor;
        ctx.fillRect(PAD + runStart * CHAR, y, (Math.min(end, MAX_COLS) - runStart) * CHAR, LINE - 1);
        runStart = -1;
      };
      for (let j = 0; j < text.length && col < MAX_COLS; j++) {
        const ch = text.charCodeAt(j);
        if (ch === 9 || ch === 32) {
          flush(col);
          col = ch === 9 ? (Math.floor(col / tabSize) + 1) * tabSize : col + 1;
          continue;
        }
        const pos = line.from + j;
        while (si < spans.length && spans[si].to <= pos) si++;
        const color = si < spans.length && spans[si].from <= pos ? this.colorOf(spans[si].cls) : textColor;
        if (color !== runColor) {
          flush(col);
          runColor = color;
        }
        if (runStart < 0) runStart = col;
        col++;
      }
      flush(col);
    }
  }

  // ---- Mouse --------------------------------------------------------------------------------

  /** Scrolls the editor so that `line` (0-based, fractional) is the first visible line. */
  scrollToLine(line: number) {
    const { geo } = this;
    const top = Math.max(0, Math.min(line, geo.lines - geo.visible));
    const scroller = this.view.scrollDOM;
    scroller.scrollTop += this.heightOfLine(top) - this.visibleTop();
  }

  onMouseDown = (e: MouseEvent) => {
    if (e.button !== 0) return;
    e.preventDefault();
    const { geo } = this;
    const y = e.clientY - this.dom.getBoundingClientRect().top;
    const sliderTop = geo.top * LINE - geo.offset;
    let top = geo.top;
    if (y < sliderTop || y > sliderTop + geo.visible * LINE) {
      // Jump: centre the clicked line, then keep dragging from there.
      top = (y + geo.offset) / LINE - geo.visible / 2;
      this.scrollToLine(top);
      top = Math.max(0, Math.min(top, geo.lines - geo.visible));
    }
    this.drag = { y, top };
    this.dom.classList.add('cm-minimap-dragging');
    window.addEventListener('mousemove', this.onMouseMove);
    window.addEventListener('mouseup', this.endDrag);
  };

  onMouseMove = (e: MouseEvent) => {
    if (!this.drag) return;
    const { geo } = this;
    const y = e.clientY - this.dom.getBoundingClientRect().top;
    // The slider moves (height - visible*LINE) pixels over (lines - visible) lines when
    // the minimap scrolls itself, and LINE pixels per line otherwise.
    const free = geo.height - geo.visible * LINE;
    const perPixel = geo.lines * LINE > geo.height && free > 0 ? (geo.lines - geo.visible) / free : 1 / LINE;
    this.scrollToLine(this.drag.top + (y - this.drag.y) * perPixel);
  };

  endDrag = () => {
    this.drag = null;
    this.dom.classList.remove('cm-minimap-dragging');
    window.removeEventListener('mousemove', this.onMouseMove);
    window.removeEventListener('mouseup', this.endDrag);
  };

  onWheel = (e: WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaMode === 1 ? this.view.defaultLineHeight : e.deltaMode === 2 ? this.view.scrollDOM.clientHeight : 1;
    this.view.scrollDOM.scrollBy({ top: e.deltaY * factor, left: e.deltaX * factor });
  };
}

const v = (name: string) => `var(--gtk-${name})`;

const minimapTheme = EditorView.theme({
  '&.cm-minimap-on .cm-scroller': { marginRight: `${WIDTH}px` },
  // The minimap takes over from the vertical scrollbar (the horizontal one stays).
  '&.cm-minimap-on .cm-scroller::-webkit-scrollbar': { width: '0' },
  '.cm-minimap': {
    position: 'absolute',
    right: '0',
    width: `${WIDTH}px`,
    backgroundColor: v('base-color'),
    borderLeft: `1px solid ${v('border-color')}`,
    boxSizing: 'border-box',
    overflow: 'hidden',
    cursor: 'default',
    userSelect: 'none',
  },
  '.cm-minimap canvas': { display: 'block' },
  '.cm-minimap-slider': {
    position: 'absolute',
    top: '0',
    left: '0',
    right: '0',
    willChange: 'transform',
    backgroundColor: v('editor-minimap-slider'),
    borderTop: `1px solid ${v('scrollbar')}`,
    borderBottom: `1px solid ${v('scrollbar')}`,
    opacity: '0.75',
    pointerEvents: 'none',
  },
  '.cm-minimap:hover .cm-minimap-slider, .cm-minimap-dragging .cm-minimap-slider': { opacity: '1' },
});

export function minimap(): Extension {
  return [
    ViewPlugin.fromClass(Minimap),
    EditorView.editorAttributes.of({ class: 'cm-minimap-on' }),
    minimapTheme,
  ];
}
