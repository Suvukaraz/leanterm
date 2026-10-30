// Location histories for KWrite's Go menu: "Previous/Next Location" (jumps such as
// go to line, search, clicks far away, tab switches) and "Previous/Next Editing
// Location" (places where text was changed). Entries span all tabs and follow edits.
import type { ChangeDesc } from '@codemirror/state';

export interface EditorLocation {
  tab: number;
  pos: number;
}

const MAX_ENTRIES = 100;

export class LocationHistory {
  private list: EditorLocation[] = [];
  private index = -1;

  /** `same` decides whether two locations count as one (e.g. same tab and line). */
  constructor(private readonly same: (a: EditorLocation, b: EditorLocation) => boolean) {}

  get entries(): readonly EditorLocation[] {
    return this.list;
  }

  get position(): number {
    return this.index;
  }

  /** Adds a location after the current one; locations after it are dropped (like a browser). */
  push(loc: EditorLocation) {
    this.list = this.list.slice(0, this.index + 1);
    const last = this.list[this.list.length - 1];
    if (last && this.same(last, loc)) this.list[this.list.length - 1] = loc;
    else this.list.push(loc);
    if (this.list.length > MAX_ENTRIES) this.list = this.list.slice(this.list.length - MAX_ENTRIES);
    this.index = this.list.length - 1;
  }

  /** Records a jump from `from` to `to`. */
  jump(from: EditorLocation, to: EditorLocation) {
    this.push(from);
    this.push(to);
  }

  /**
   * Records an edit at `loc`: an edit on the latest entry's line replaces it,
   * otherwise it is appended (edits never truncate the history).
   */
  edit(loc: EditorLocation) {
    const last = this.list[this.list.length - 1];
    if (last && this.same(last, loc)) this.list[this.list.length - 1] = loc;
    else this.list.push(loc);
    if (this.list.length > MAX_ENTRIES) this.list = this.list.slice(this.list.length - MAX_ENTRIES);
    this.index = this.list.length - 1;
  }

  /**
   * The location to go back to from `current`. If the cursor moved away from the
   * current entry, that entry comes first and `current` is kept after it, so
   * forward() returns there.
   */
  back(current: EditorLocation | undefined): EditorLocation | undefined {
    if (this.index < 0) return undefined;
    const entry = this.list[this.index];
    if (current && !this.same(entry, current)) {
      this.list = [...this.list.slice(0, this.index + 1), current];
      return entry;
    }
    if (this.index === 0) return undefined;
    this.index--;
    return this.list[this.index];
  }

  forward(): EditorLocation | undefined {
    if (this.index >= this.list.length - 1) return undefined;
    this.index++;
    return this.list[this.index];
  }

  /** Keeps positions of `tab` valid after a document change. */
  map(tab: number, changes: ChangeDesc) {
    for (const loc of this.list) if (loc.tab === tab) loc.pos = changes.mapPos(loc.pos);
  }

  removeTab(tab: number) {
    const before = this.list.slice(0, this.index + 1).filter((l) => l.tab === tab).length;
    this.list = this.list.filter((l) => l.tab !== tab);
    this.index = Math.min(this.list.length - 1, Math.max(this.list.length ? 0 : -1, this.index - before));
  }

  clear() {
    this.list = [];
    this.index = -1;
  }
}
