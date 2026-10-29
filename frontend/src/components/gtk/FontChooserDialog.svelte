<script lang="ts">
  // Font chooser modelled on KDE's "Select Font" dialog: family, style and size
  // lists, OpenType features, an editable sample and a monospace filter. Used by
  // FontButton for every font setting of LeanTerm.
  import { onMount, tick } from 'svelte';
  import Dialog from './Dialog.svelte';
  import Button from './Button.svelte';
  import CheckButton from './CheckButton.svelte';
  import {
    defaultStyle,
    fontFaceCss,
    listFonts,
    normalizeFeatures,
    sortStyles,
    type FontFamily,
    type FontSetting,
  } from '../../lib/fonts';

  interface Props {
    font: FontSetting | null | undefined;
    /** Start with "Show only monospaced fonts" checked (terminal and editor fonts). */
    monospace?: boolean;
    onselect: (font: Required<FontSetting>) => void;
    oncancel: () => void;
  }

  let { font, monospace = false, onselect, oncancel }: Props = $props();

  const SIZES = [6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24, 26, 28, 32, 36, 40, 48, 56, 64, 72];
  const SAMPLE = 'The quick brown fox jumps over the lazy dog — 0123456789 {}[]()<>=!/\\|@#$%&*';

  // The dialog edits a copy; props are only read when it opens.
  // svelte-ignore state_referenced_locally
  const initial = font;
  let families = $state.raw<FontFamily[]>([]);
  let loading = $state(true);
  // svelte-ignore state_referenced_locally
  let onlyMono = $state(monospace);
  let family = $state(initial?.Family || 'Monospace');
  let style = $state(initial?.Style || '');
  let size = $state(initial?.Size && initial.Size > 0 ? initial.Size : 11);
  let features = $state(initial?.Features ?? '');
  let sample = $state(SAMPLE);

  let familyListEl = $state<HTMLElement>();
  let styleListEl = $state<HTMLElement>();
  let sizeListEl = $state<HTMLElement>();

  // The current family stays in the list even if it is not installed (or not monospace).
  let shownFamilies = $derived.by(() => {
    const list = onlyMono ? families.filter((f) => f.monospace) : families;
    if (family && !list.some((f) => f.name.toLowerCase() === family.toLowerCase())) {
      const known = families.find((f) => f.name.toLowerCase() === family.toLowerCase());
      return [known ?? { name: family, styles: ['Regular'], monospace: true }, ...list];
    }
    return list;
  });
  let current = $derived(shownFamilies.find((f) => f.name.toLowerCase() === family.toLowerCase()));
  let styles = $derived(sortStyles(current?.styles?.length ? current.styles : ['Regular']));
  let effectiveStyle = $derived(styles.includes(style) ? style : defaultStyle(styles));
  let sizeValid = $derived(Number.isFinite(size) && size >= 6 && size <= 72);

  let sampleCss = $derived(
    `font-family: "${family.replace(/"/g, '')}", monospace; font-size: ${sizeValid ? size : 11}pt; ` +
      fontFaceCss({ Style: effectiveStyle, Features: features })
  );

  onMount(() => {
    void listFonts().then(async (list) => {
      families = list;
      loading = false;
      await tick();
      revealSelection();
    });
  });

  function revealSelection() {
    for (const el of [familyListEl, styleListEl, sizeListEl]) {
      el?.querySelector<HTMLElement>('[aria-selected="true"]')?.scrollIntoView({ block: 'center' });
    }
  }

  function selectFamily(name: string) {
    family = name;
    void tick().then(() => styleListEl?.querySelector<HTMLElement>('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }));
  }

  // ---- Keyboard: arrows, Home/End, PageUp/PageDown and type-ahead in the lists --------------

  let typeAhead = '';
  let typeAheadTimer: ReturnType<typeof setTimeout> | undefined;

  function listKeys<T>(e: KeyboardEvent, items: T[], index: number, label: (item: T) => string, choose: (item: T) => void) {
    if (items.length === 0) return;
    let next = -1;
    const page = 8;
    if (e.key === 'ArrowDown') next = Math.min(items.length - 1, index + 1);
    else if (e.key === 'ArrowUp') next = Math.max(0, index - 1);
    else if (e.key === 'PageDown') next = Math.min(items.length - 1, index + page);
    else if (e.key === 'PageUp') next = Math.max(0, index - page);
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = items.length - 1;
    else if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey) {
      clearTimeout(typeAheadTimer);
      typeAhead += e.key.toLowerCase();
      typeAheadTimer = setTimeout(() => (typeAhead = ''), 800);
      next = items.findIndex((it) => label(it).toLowerCase().startsWith(typeAhead));
    }
    if (next < 0) return;
    e.preventDefault();
    choose(items[next]);
    void tick().then(() =>
      (e.currentTarget as HTMLElement | null)?.querySelector<HTMLElement>('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
    );
  }

  function confirm() {
    if (!sizeValid) return;
    onselect({
      Family: family.trim(),
      Size: Math.round(size),
      Style: /^(regular|book|normal|roman)$/i.test(effectiveStyle) ? '' : effectiveStyle,
      Features: normalizeFeatures(features),
    });
  }
</script>

<Dialog
  title="Select Font"
  width={760}
  height={560}
  movable
  resizable
  minWidth={520}
  minHeight={420}
  geometryKey="font-chooser"
  open={true}
  onclose={oncancel}
>
  <div class="font-chooser">
    <div class="lists">
      <div class="column family-column">
        <span class="column-label" id="font-family-label">Font:</span>
        <div
          class="listbox"
          role="listbox"
          tabindex="0"
          aria-labelledby="font-family-label"
          bind:this={familyListEl}
          onkeydown={(e) =>
            listKeys(e, shownFamilies, shownFamilies.findIndex((f) => f === current), (f) => f.name, (f) => selectFamily(f.name))}
        >
          {#if loading}
            <div class="list-hint">Loading fonts…</div>
          {:else}
            {#each shownFamilies as f (f.name)}
              <div
                class="item"
                role="option"
                tabindex="-1"
                aria-selected={f === current}
                onclick={() => selectFamily(f.name)}
                onkeydown={() => {}}
              >
                {f.name}
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <div class="column style-column">
        <span class="column-label" id="font-style-label">Font style:</span>
        <div
          class="listbox"
          role="listbox"
          tabindex="0"
          aria-labelledby="font-style-label"
          bind:this={styleListEl}
          onkeydown={(e) => listKeys(e, styles, styles.indexOf(effectiveStyle), (s) => s, (s) => (style = s))}
        >
          {#each styles as s (s)}
            <div
              class="item"
              role="option"
              tabindex="-1"
              aria-selected={s === effectiveStyle}
              onclick={() => (style = s)}
              onkeydown={() => {}}
            >
              {s}
            </div>
          {/each}
        </div>
      </div>

      <div class="column size-column">
        <label class="column-label" for="font-size-input">Size:</label>
        <input
          id="font-size-input"
          class="size-input"
          class:invalid={!sizeValid}
          type="number"
          min="6"
          max="72"
          step="1"
          bind:value={size}
        />
        <div
          class="listbox"
          role="listbox"
          tabindex="0"
          aria-label="Size"
          bind:this={sizeListEl}
          onkeydown={(e) => listKeys(e, SIZES, SIZES.indexOf(Math.round(size)), (n) => String(n), (n) => (size = n))}
        >
          {#each SIZES as n (n)}
            <div
              class="item"
              role="option"
              tabindex="-1"
              aria-selected={n === Math.round(size)}
              onclick={() => (size = n)}
              onkeydown={() => {}}
            >
              {n}
            </div>
          {/each}
        </div>
      </div>
    </div>

    <label class="row-label" for="font-features-input">Comma-separated list of font features (e.g. liga or calt):</label>
    <input
      id="font-features-input"
      type="text"
      class="features-input"
      placeholder="liga, calt, -dlig, ss01=2"
      spellcheck="false"
      bind:value={features}
    />

    <label class="row-label" for="font-sample">Sample:</label>
    <textarea id="font-sample" class="sample" spellcheck="false" style={sampleCss} bind:value={sample}></textarea>

    <CheckButton bind:checked={onlyMono} label="Show only monospaced fonts" />
  </div>

  {#snippet actions()}
    <Button label="OK" icon="gtk-ok" isDefault={true} disabled={!sizeValid || !family.trim()} onclick={confirm} />
    <Button label="Cancel" icon="gtk-cancel" onclick={oncancel} />
  {/snippet}
</Dialog>

<style>
  .font-chooser {
    display: flex;
    flex-direction: column;
    gap: 6px;
    height: 100%;
    min-height: 0;
    box-sizing: border-box;
  }

  .lists {
    display: grid;
    grid-template-columns: minmax(0, 2.4fr) minmax(0, 1fr) minmax(0, 1.4fr);
    gap: 8px;
    flex: 1 1 auto;
    min-height: 170px;
  }

  .column {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-height: 0;
  }

  .column-label,
  .row-label {
    font-size: 0.95em;
  }

  .row-label {
    margin-top: 6px;
  }

  .listbox {
    flex: 1 1 0;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    background: var(--gtk-base-color);
    color: var(--gtk-text-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    outline: none;
    user-select: none;
  }

  .listbox:focus-visible {
    border-color: var(--gtk-accent);
  }

  .item {
    padding: 3px 6px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: default;
  }

  .item:hover {
    background: var(--gtk-editor-active-line);
  }

  .item[aria-selected='true'] {
    background: var(--gtk-selected-bg, var(--gtk-accent));
    color: var(--gtk-selected-fg, #ffffff);
  }

  .listbox:not(:focus) .item[aria-selected='true'] {
    opacity: 0.85;
  }

  .list-hint {
    padding: 6px;
    color: var(--gtk-muted-fg);
  }

  .size-input,
  .features-input {
    box-sizing: border-box;
    width: 100%;
  }

  .size-input.invalid {
    outline: 1px solid var(--gtk-error-fg, #e01b24);
  }

  .sample {
    flex: 0 0 auto;
    height: 96px;
    min-height: 48px;
    resize: vertical;
    box-sizing: border-box;
    width: 100%;
    padding: 6px 8px;
    background: var(--gtk-base-color);
    color: var(--gtk-text-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    line-height: 1.3;
  }
</style>
