<script lang="ts">
  // KWrite-like status bar. Line ending, encoding and language are buttons that
  // switch the value (line ending) or open a menu at the button.
  interface Props {
    line: number;
    column: number;
    selected: number;
    overwrite: boolean;
    indent: string;
    encoding: string;
    lineEnding: 'lf' | 'crlf';
    mixedLineEndings: boolean;
    language: string;
    source: string;
    message?: string;
    ontoggleLineEnding: () => void;
    ontoggleOverwrite: () => void;
    onencoding: (anchor: DOMRect) => void;
    onlanguage: (anchor: DOMRect) => void;
  }

  let {
    line,
    column,
    selected,
    overwrite,
    indent,
    encoding,
    lineEnding,
    mixedLineEndings,
    language,
    source,
    message = '',
    ontoggleLineEnding,
    ontoggleOverwrite,
    onencoding,
    onlanguage,
  }: Props = $props();

  const rect = (e: MouseEvent) => (e.currentTarget as HTMLElement).getBoundingClientRect();
</script>

<div class="editor-statusbar">
  <span class="sb-item sb-pos">Line {line}, Column {column}{selected > 0 ? ` (${selected} selected)` : ''}</span>
  <span class="sb-message" title={message}>{message}</span>
  {#if source}
    <span class="sb-item sb-source" title={source}>{source}</span>
  {/if}
  <button type="button" class="sb-item sb-button" title="Toggle insert/overwrite (Insert)" onclick={ontoggleOverwrite}>
    {overwrite ? 'OVR' : 'INS'}
  </button>
  <span class="sb-item">{indent}</span>
  <button type="button" class="sb-item sb-button" title="Encoding" onclick={(e) => onencoding(rect(e))}>
    {encoding.toUpperCase()}
  </button>
  <button
    type="button"
    class="sb-item sb-button"
    class:warn={mixedLineEndings}
    title={mixedLineEndings
      ? 'The file has mixed line endings; saving converts all of them. Click to switch.'
      : 'Line endings: click to switch between Unix (LF) and Windows (CRLF)'}
    onclick={ontoggleLineEnding}
  >
    {lineEnding === 'crlf' ? 'Windows (CRLF)' : 'Unix (LF)'}
  </button>
  <button type="button" class="sb-item sb-button sb-lang" title="Syntax highlighting mode" onclick={(e) => onlanguage(rect(e))}>
    {language}
  </button>
</div>

<style>
  .editor-statusbar {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 1px 4px;
    background: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
    font-size: 0.9em;
    flex-shrink: 0;
    min-height: 24px;
    user-select: none;
  }
  .sb-item {
    padding: 2px 8px;
    white-space: nowrap;
    color: var(--gtk-fg-color);
  }
  .sb-pos {
    min-width: 150px;
  }
  .sb-message {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--gtk-muted-fg);
    padding: 0 8px;
  }
  .sb-source {
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--gtk-muted-fg);
  }
  .sb-button {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    font-size: inherit;
  }
  .sb-button:hover {
    background: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }
  .sb-button.warn {
    color: var(--gtk-warning-fg);
  }
</style>
