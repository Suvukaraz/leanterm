// Mouse button reports for the Rio engine. rioterm reports the wheel itself but
// not clicks and drags, and it does not expose which mouse protocol the
// application asked for, so the DECSET/DECRST sequences are tracked here from the
// output stream (chunk boundaries are carried over).

/** What the application wants reported (DECSET 9 / 1000 / 1002 / 1003). */
export type MouseTracking = 'none' | 'x10' | 'normal' | 'button' | 'any';

const TRACKING_MODES: Record<number, MouseTracking> = { 9: 'x10', 1000: 'normal', 1002: 'button', 1003: 'any' };
const SGR_MODE = 1006;

const enum State {
  Ground,
  Escape,
  Csi,
}

export class MouseModes {
  tracking: MouseTracking = 'none';
  sgr = false;

  private state = State.Ground;
  private privateMarker = false;
  private params = '';

  /** Feeds terminal output (the bytes also written to the terminal). */
  feed(data: Uint8Array): void {
    for (let i = 0; i < data.length; i++) {
      const b = data[i];
      switch (this.state) {
        case State.Ground:
          if (b === 0x1b) this.state = State.Escape;
          break;
        case State.Escape:
          if (b === 0x5b) {
            this.state = State.Csi;
            this.privateMarker = false;
            this.params = '';
          } else {
            if (b === 0x63) this.reset(); // RIS (ESC c)
            this.state = b === 0x1b ? State.Escape : State.Ground;
          }
          break;
        case State.Csi:
          if (b === 0x3f && this.params === '' && !this.privateMarker) {
            this.privateMarker = true;
          } else if ((b >= 0x30 && b <= 0x39) || b === 0x3b) {
            if (this.params.length < 64) this.params += String.fromCharCode(b);
          } else if (b >= 0x40 && b <= 0x7e) {
            if (this.privateMarker && (b === 0x68 || b === 0x6c)) this.setModes(b === 0x68);
            this.state = State.Ground;
          } else {
            this.state = b === 0x1b ? State.Escape : State.Ground;
          }
          break;
      }
    }
  }

  reset(): void {
    this.tracking = 'none';
    this.sgr = false;
  }

  private setModes(on: boolean): void {
    for (const p of this.params.split(';')) {
      const mode = Number(p);
      if (mode === SGR_MODE) {
        this.sgr = on;
      } else if (mode in TRACKING_MODES) {
        if (on) this.tracking = TRACKING_MODES[mode];
        else if (this.tracking === TRACKING_MODES[mode]) this.tracking = 'none';
      }
    }
  }
}

export interface MouseReport {
  kind: 'press' | 'release' | 'motion';
  /** DOM button: 0 left, 1 middle, 2 right; -1 for motion without a button. */
  button: number;
  /** Zero-based cell. */
  col: number;
  row: number;
  shift: boolean;
  alt: boolean;
  ctrl: boolean;
}

/** Encodes a report for the current modes; undefined when nothing is to be sent. */
export function encodeMouseReport(modes: MouseModes, r: MouseReport): Uint8Array | undefined {
  const t = modes.tracking;
  if (t === 'none') return undefined;
  if (t === 'x10' && r.kind !== 'press') return undefined;
  if (r.kind === 'motion' && t !== 'any' && !(t === 'button' && r.button >= 0)) return undefined;
  if (r.button > 2) return undefined;

  let code = r.button < 0 ? 3 : r.button;
  if (r.kind === 'release' && !modes.sgr) code = 3;
  if (r.kind === 'motion') code += 32;
  if (t !== 'x10') code += (r.shift ? 4 : 0) + (r.alt ? 8 : 0) + (r.ctrl ? 16 : 0);

  const x = r.col + 1;
  const y = r.row + 1;
  const report = modes.sgr
    ? `\x1b[<${code};${x};${y}${r.kind === 'release' ? 'm' : 'M'}`
    : // Legacy encoding: one raw byte per value, coordinates beyond 223 cannot be expressed.
      `\x1b[M${String.fromCharCode(32 + code, 32 + Math.min(x, 223), 32 + Math.min(y, 223))}`;
  return Uint8Array.from(report, (c) => c.charCodeAt(0));
}
