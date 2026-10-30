<script lang="ts">
  import { baseName } from '../../lib/fspath';
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    open: boolean;
    paths: string[];
    initialMode?: number;
    onapply: (mode: number, recursive: boolean, scope: string) => void;
    onclose: () => void;
  }

  let { open, paths, initialMode = 0o755, onapply, onclose }: Props = $props();

  let uRead = $state(true);
  let uWrite = $state(true);
  let uExec = $state(true);

  let gRead = $state(true);
  let gWrite = $state(false);
  let gExec = $state(true);

  let oRead = $state(true);
  let oWrite = $state(false);
  let oExec = $state(true);

  let octalText = $state('0755');
  let recursive = $state(false);
  let scope = $state('all'); // 'all', 'dirs', 'files'

  $effect(() => {
    if (open) {
      const mode = initialMode & 0o777;
      uRead = (mode & 0o400) !== 0;
      uWrite = (mode & 0o200) !== 0;
      uExec = (mode & 0o100) !== 0;

      gRead = (mode & 0o040) !== 0;
      gWrite = (mode & 0o020) !== 0;
      gExec = (mode & 0o010) !== 0;

      oRead = (mode & 0o004) !== 0;
      oWrite = (mode & 0o002) !== 0;
      oExec = (mode & 0o001) !== 0;

      updateOctalFromBits();
      recursive = false;
      scope = 'all';
    }
  });

  function updateOctalFromBits() {
    let u = (uRead ? 4 : 0) + (uWrite ? 2 : 0) + (uExec ? 1 : 0);
    let g = (gRead ? 4 : 0) + (gWrite ? 2 : 0) + (gExec ? 1 : 0);
    let o = (oRead ? 4 : 0) + (oWrite ? 2 : 0) + (oExec ? 1 : 0);
    octalText = `0${u}${g}${o}`;
  }

  function handleOctalChange(e: Event) {
    const val = (e.target as HTMLInputElement).value.trim();
    octalText = val;
    const num = parseInt(val, 8);
    if (!isNaN(num)) {
      uRead = (num & 0o400) !== 0;
      uWrite = (num & 0o200) !== 0;
      uExec = (num & 0o100) !== 0;

      gRead = (num & 0o040) !== 0;
      gWrite = (num & 0o020) !== 0;
      gExec = (num & 0o010) !== 0;

      oRead = (num & 0o004) !== 0;
      oWrite = (num & 0o002) !== 0;
      oExec = (num & 0o001) !== 0;
    }
  }

  function handleCheckboxToggle() {
    updateOctalFromBits();
  }

  function handleApply() {
    const mode = parseInt(octalText, 8);
    if (isNaN(mode)) return;
    onapply(mode, recursive, scope);
    onclose();
  }

  let title = $derived(
    paths.length === 1
      ? `Change Permissions: ${baseName(paths[0]) || paths[0]}`
      : `Change Permissions for ${paths.length} items`
  );
</script>

<Dialog {open} {title} width={400} onclose={onclose}>
  <div class="permissions-dialog-content">
    <div class="matrix-box">
      <table class="perm-table">
        <thead>
          <tr>
            <th></th>
            <th>Read</th>
            <th>Write</th>
            <th>Execute</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td class="role-col"><b>Owner</b></td>
            <td><input type="checkbox" bind:checked={uRead} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={uWrite} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={uExec} onchange={handleCheckboxToggle} /></td>
          </tr>
          <tr>
            <td class="role-col"><b>Group</b></td>
            <td><input type="checkbox" bind:checked={gRead} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={gWrite} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={gExec} onchange={handleCheckboxToggle} /></td>
          </tr>
          <tr>
            <td class="role-col"><b>Others</b></td>
            <td><input type="checkbox" bind:checked={oRead} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={oWrite} onchange={handleCheckboxToggle} /></td>
            <td><input type="checkbox" bind:checked={oExec} onchange={handleCheckboxToggle} /></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="octal-row">
      <label for="octal-input"><b>Octal value:</b></label>
      <input
        id="octal-input"
        type="text"
        class="octal-input"
        value={octalText}
        oninput={handleOctalChange}
      />
    </div>

    <hr class="separator" />

    <div class="recursive-section">
      <label class="checkbox-label">
        <input type="checkbox" bind:checked={recursive} />
        <span>Apply recursively</span>
      </label>

      {#if recursive}
        <div class="scope-options">
          <label class="radio-label">
            <input type="radio" bind:group={scope} value="all" />
            <span>Files and folders</span>
          </label>
          <label class="radio-label">
            <input type="radio" bind:group={scope} value="dirs" />
            <span>Folders only</span>
          </label>
          <label class="radio-label">
            <input type="radio" bind:group={scope} value="files" />
            <span>Files only</span>
          </label>
        </div>
      {/if}
    </div>
  </div>

  {#snippet actions()}
    <Button label="Cancel" onclick={onclose} />
    <Button label="Apply" isDefault onclick={handleApply} />
  {/snippet}
</Dialog>

<style>
  .permissions-dialog-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
    font-size: 13px;
    color: var(--gtk-fg, #2e3436);
  }

  .matrix-box {
    background: var(--gtk-base, #ffffff);
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 4px;
    padding: 8px;
  }

  .perm-table {
    width: 100%;
    border-collapse: collapse;
    text-align: center;
  }

  .perm-table th {
    font-weight: 600;
    padding: 6px;
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
  }

  .perm-table td {
    padding: 6px;
  }

  .perm-table .role-col {
    text-align: left;
    padding-left: 8px;
  }

  .octal-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .octal-input {
    width: 80px;
    font-family: monospace;
    font-size: 14px;
    padding: 4px 8px;
    background: var(--gtk-base, #ffffff);
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 3px;
    color: var(--gtk-fg, #2e3436);
  }

  .separator {
    border: none;
    border-top: 1px solid var(--gtk-border, #e0e0e0);
    margin: 4px 0;
  }

  .recursive-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .checkbox-label,
  .radio-label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
  }

  .scope-options {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-left: 24px;
  }
</style>
