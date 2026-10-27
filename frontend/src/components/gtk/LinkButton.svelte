<script lang="ts">
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    label?: string;
    url?: string;
    tooltip?: string;
    class?: string;
  }

  let {
    label = 'Help',
    url = 'https://github.com/Suvukaraz/leanterm/wiki',
    tooltip = 'Open Online Help',
    class: className = ''
  }: Props = $props();

  function openLink() {
    window.open(url, '_blank');
  }
</script>

<button
  type="button"
  class={`gtk-linkbutton button-cp ${className}`}
  title={tooltip}
  onclick={openLink}
>
  <img src={getIcon('gtk-help')} alt="" width="16" height="16" />
  <span class="link-label">{label}</span>
</button>

<style>
  .gtk-linkbutton {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--gtk-link-fg);
    cursor: pointer;
    font-size: 0.9em;
    text-decoration: underline;
  }
  .link-label {
    white-space: nowrap;
  }
</style>
