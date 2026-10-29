<script lang="ts">
  // One CodeMirror view per tab. Inactive tabs stay mounted (hidden), so scroll
  // position, folds and undo history survive tab switches.
  import { onDestroy, onMount } from 'svelte';
  import { EditorView } from '@codemirror/view';
  import type { EditorState } from '@codemirror/state';

  interface Props {
    editorState: EditorState;
    active: boolean;
    onready: (view: EditorView) => void;
    ondestroy: (view: EditorView) => void;
  }

  let { editorState, active, onready, ondestroy }: Props = $props();

  let host: HTMLDivElement;
  let view = $state.raw<EditorView | undefined>(undefined);

  onMount(() => {
    view = new EditorView({ state: editorState, parent: host });
    onready(view);
  });

  // A hidden editor cannot measure itself; refresh and focus when shown again.
  $effect(() => {
    if (active && view) {
      const v = view;
      requestAnimationFrame(() => {
        v.requestMeasure();
        v.focus();
      });
    }
  });

  onDestroy(() => {
    if (!view) return;
    ondestroy(view);
    view.destroy();
  });
</script>

<div class="editor-pane" class:hidden={!active} bind:this={host}></div>

<style>
  .editor-pane {
    position: absolute;
    inset: 0;
  }
  .editor-pane.hidden {
    display: none;
  }
  .editor-pane :global(.cm-editor) {
    height: 100%;
  }
  .editor-pane :global(.cm-editor.cm-focused) {
    outline: none;
  }
</style>
