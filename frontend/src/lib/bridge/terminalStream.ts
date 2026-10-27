import { Events } from '@wailsio/runtime';
import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
import type { Terminal } from '@xterm/xterm';

export function base64ToUint8Array(b64: string): Uint8Array {
  const binaryString = window.atob(b64);
  const len = binaryString.length;
  const bytes = new Uint8Array(len);
  for (let i = 0; i < len; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes;
}

export function uint8ArrayToBase64(bytes: Uint8Array): string {
  let binary = '';
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return window.btoa(binary);
}

export interface StreamPacket {
  seq: number;
  b64: string;
}

const encoder = new TextEncoder();
const INPUT_BATCH_MS = 4;
const PASTE_CHUNK = 4096;

/**
 * Connects one xterm.js instance to a backend session.
 *
 * Output: subscribes to `term:data:<tmpID>` *before* requesting the replay, buffers
 * packets until the replay is written and then drops everything already contained
 * in it (seq <= lastSeq). Writes are acknowledged for backend flow control.
 *
 * Input: keystrokes are queued and sent strictly in order, one RPC at a time,
 * coalescing whatever arrives meanwhile (fast typing / pastes stay ordered).
 */
export class TerminalStream {
  private tmpID: string;
  private term: Terminal;
  private unsubscribeEvent?: () => void;
  private lastSeq = 0;
  private replayDone = false;
  private early: StreamPacket[] = [];
  private lastAckTime = 0;
  private pendingAckSeq = 0;
  private ackTimer?: ReturnType<typeof setTimeout>;
  private destroyed = false;

  private inputQueue: Uint8Array[] = [];
  private inputTimer?: ReturnType<typeof setTimeout>;
  private sending = false;

  /** Called for every output packet (used for the "new data" tab colour). */
  public ondata?: () => void;

  constructor(tmpID: string, term: Terminal) {
    this.tmpID = tmpID;
    this.term = term;
  }

  public async connect(): Promise<void> {
    const eventName = `term:data:${this.tmpID}`;
    this.unsubscribeEvent = Events.On(eventName, (event: any) => {
      const packet = (event?.data ?? event) as StreamPacket;
      if (!packet || typeof packet.seq !== 'number') return;
      if (!this.replayDone) {
        this.early.push(packet);
        return;
      }
      this.handlePacket(packet);
    });

    try {
      const attachRes: any = await SessionService.Attach(this.tmpID);
      if (this.destroyed) return;
      if (attachRes?.replayB64) {
        this.term.write(base64ToUint8Array(attachRes.replayB64));
      }
      this.lastSeq = attachRes?.lastSeq || 0;
    } catch (err) {
      console.warn(`Attach replay for ${this.tmpID}:`, err);
    }

    this.replayDone = true;
    const early = this.early;
    this.early = [];
    for (const p of early) this.handlePacket(p);
  }

  private handlePacket(packet: StreamPacket): void {
    if (packet.seq <= this.lastSeq) {
      // Already part of the replay; still acknowledge so flow control never stalls.
      this.scheduleAck(packet.seq);
      return;
    }
    this.lastSeq = packet.seq;
    this.term.write(base64ToUint8Array(packet.b64), () => this.scheduleAck(packet.seq));
    this.ondata?.();
  }

  private scheduleAck(seq: number): void {
    this.pendingAckSeq = Math.max(this.pendingAckSeq, seq);
    const elapsed = Date.now() - this.lastAckTime;
    if (elapsed >= 100) {
      this.sendAck();
    } else if (!this.ackTimer) {
      this.ackTimer = setTimeout(() => {
        this.ackTimer = undefined;
        this.sendAck();
      }, 100 - elapsed);
    }
  }

  private sendAck(): void {
    if (this.pendingAckSeq === 0) return;
    this.lastAckTime = Date.now();
    SessionService.Ack(this.tmpID, this.pendingAckSeq).catch((err) => {
      console.warn(`Ack failed for ${this.tmpID}:`, err);
    });
  }

  public write(data: string | Uint8Array): void {
    if (this.destroyed) return;
    const bytes = typeof data === 'string' ? encoder.encode(data) : data;
    if (bytes.length === 0) return;
    this.inputQueue.push(bytes);
    if (!this.inputTimer && !this.sending) {
      this.inputTimer = setTimeout(() => {
        this.inputTimer = undefined;
        void this.flushInput();
      }, INPUT_BATCH_MS);
    }
  }

  private async flushInput(): Promise<void> {
    if (this.sending) return;
    this.sending = true;
    try {
      while (this.inputQueue.length > 0) {
        const total = this.inputQueue.reduce((n, b) => n + b.length, 0);
        const merged = new Uint8Array(total);
        let off = 0;
        for (const b of this.inputQueue) {
          merged.set(b, off);
          off += b.length;
        }
        this.inputQueue = [];
        for (let i = 0; i < merged.length; i += PASTE_CHUNK) {
          const chunk = merged.subarray(i, i + PASTE_CHUNK);
          try {
            await SessionService.Write(this.tmpID, uint8ArrayToBase64(chunk));
          } catch (err) {
            console.warn(`Write failed for ${this.tmpID}:`, err);
          }
        }
      }
    } finally {
      this.sending = false;
    }
  }

  public resize(cols: number, rows: number): void {
    if (cols <= 0 || rows <= 0) return;
    SessionService.Resize(this.tmpID, cols, rows).catch((err) => {
      console.warn(`Resize failed for ${this.tmpID}:`, err);
    });
  }

  public destroy(): void {
    this.destroyed = true;
    this.unsubscribeEvent?.();
    this.unsubscribeEvent = undefined;
    clearTimeout(this.ackTimer);
    clearTimeout(this.inputTimer);
    this.ackTimer = undefined;
    if (this.pendingAckSeq > 0) this.sendAck();
  }
}
