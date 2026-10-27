<script lang="ts">
  import Paned from '../gtk/Paned.svelte';
  import TerminalTab from './TerminalTab.svelte';
  import type { TabSplit } from '../../lib/stores/splits';
  import { settings } from '../../lib/stores/config';

  interface Props {
    split: TabSplit;
    active?: boolean;
  }

  let { split, active = true }: Props = $props();

  let force50 = $derived(
    $settings?.general?.ForceSplit50 ?? false
  );
</script>

<div class="split-tab-wrapper">
  <Paned
    orientation={split.orientation}
    bind:splitPercent={split.ratio}
    {force50}
    minSizeFirst={200}
    minSizeSecond={200}
  >
    {#snippet first()}
      <div class="split-pane-content">
        <TerminalTab tmpID={split.primaryTmpID} {active} />
      </div>
    {/snippet}

    {#snippet second()}
      <div class="split-pane-content">
        <TerminalTab tmpID={split.secondaryTmpID} active={false} />
      </div>
    {/snippet}
  </Paned>
</div>

<style>
  .split-tab-wrapper {
    width: 100%;
    height: 100%;
    display: flex;
    overflow: hidden;
  }

  .split-pane-content {
    width: 100%;
    height: 100%;
    overflow: hidden;
  }
</style>
