<script lang="ts">
  interface Props {
    value?: number; // 0-100
    pulse?: boolean;
    class?: string;
  }

  let { value = 0, pulse = false, class: className = '' }: Props = $props();
</script>

<div class={`gtk-progressbar ${pulse ? 'is-pulse' : ''} ${className}`}>
  {#if pulse}
    <div class="progress-pulse-bar"></div>
  {:else}
    <div class="progress-fill" style={`width: ${Math.max(0, Math.min(100, value))}%;`}></div>
  {/if}
</div>

<style>
  .gtk-progressbar {
    width: 100px;
    height: 10px;
    background-color: var(--gtk-btn-border);
    border-radius: 5px;
    overflow: hidden;
    position: relative;
    display: inline-block;
  }
  .progress-fill {
    height: 100%;
    background-color: var(--gtk-selected-bg);
    transition: width 0.2s ease-out;
  }
  .is-pulse .progress-pulse-bar {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 30%;
    background-color: var(--gtk-selected-bg);
    border-radius: 5px;
    animation: pulse-anim 1.2s infinite ease-in-out;
  }
  @keyframes pulse-anim {
    0% {
      left: -30%;
    }
    100% {
      left: 100%;
    }
  }
</style>
