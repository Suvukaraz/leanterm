<script lang="ts">
  import MacrosTab from '../edit/MacrosTab.svelte';

  interface Props {
    settings: any;
    isRemote?: boolean;
  }

  let { settings = $bindable(), isRemote = false }: Props = $props();

  const title = $derived(
    isRemote
      ? 'Global common remote commands list'
      : 'Global common local commands list'
  );
</script>

<div class="global-commands-page">
  {#if isRemote}
    <MacrosTab
      {title}
      isRemote={true}
      bind:items={settings.macros.Remote}
    />
  {:else}
    <MacrosTab
      {title}
      isRemote={false}
      bind:items={settings.macros.Local}
    />
  {/if}
</div>

<style>
  .global-commands-page {
    height: 100%;
    overflow-y: auto;
  }
</style>
