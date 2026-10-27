<script lang="ts">
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    items: any[];
    disabled?: boolean;
  }

  let { items = $bindable([]), disabled = false }: Props = $props();

  function addRule() {
    items = [
      ...items,
      {
        ID: '',
        Enabled: true,
        Pattern: '',
        Send: '',
        Hidden: false,
        AppendReturn: true,
        OnMatch: 'next',
        OnMatchGoto: 1,
        OnFail: 'stop',
        OnFailGoto: 1,
        Timeout: 10,
      },
    ];
  }

  function removeRule(index: number) {
    items = items.filter((_, i) => i !== index);
  }

  function moveUp(index: number) {
    if (index <= 0) return;
    const updated = [...items];
    const temp = updated[index];
    updated[index] = updated[index - 1];
    updated[index - 1] = temp;
    items = updated;
  }

  function moveDown(index: number) {
    if (index >= items.length - 1) return;
    const updated = [...items];
    const temp = updated[index];
    updated[index] = updated[index + 1];
    updated[index + 1] = temp;
    items = updated;
  }
</script>

<div class="expect-tab-container">
  <div class="expect-list">
    {#if items.length === 0}
      <div class="empty-hint">
        No Expect rules defined. Click "Add Expect Rule" below to automate interactive prompts.
      </div>
    {:else}
      {#each items as rule, i}
        <div class="expect-card">
          <div class="card-header">
            <div class="header-left">
              <CheckButton
                bind:checked={rule.Enabled}
                label={`Expect #${i + 1}`}
                {disabled}
              />
            </div>
            <div class="header-actions">
              <Button
                icon="gtk-go-up"
                tooltip="Move up"
                disabled={disabled || i === 0}
                onclick={() => moveUp(i)}
              />
              <Button
                icon="gtk-go-down"
                tooltip="Move down"
                disabled={disabled || i === items.length - 1}
                onclick={() => moveDown(i)}
              />
              <Button
                icon="gtk-delete"
                tooltip="Delete rule"
                {disabled}
                onclick={() => removeRule(i)}
              />
            </div>
          </div>

          <div class="card-body">
            <div class="row-flex">
              <span class="field-label">Expect:</span>
              <Entry
                bind:value={rule.Pattern}
                placeholder="Regular expression pattern"
                tooltip="RegExp pattern to wait for"
                {disabled}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Send:</span>
              <Entry
                type={rule.Hidden ? 'password' : 'text'}
                bind:value={rule.Send}
                placeholder="String to send"
                tooltip="String to send when matched"
                {disabled}
              />
              <CheckButton
                bind:checked={rule.Hidden}
                label="Hide"
                tooltip="Hide string in secrets vault"
                {disabled}
              />
              <CheckButton
                bind:checked={rule.AppendReturn}
                label="Return"
                tooltip="Sends a <RETURN> (\n) after the string is entered."
                {disabled}
              />
            </div>

            <div class="row-flex wrap-row">
              <span class="field-label">If match:</span>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`match_${i}`}
                  value="next"
                  checked={rule.OnMatch === 'next'}
                  onchange={() => (rule.OnMatch = 'next')}
                  {disabled}
                />
                Next
              </label>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`match_${i}`}
                  value="stop"
                  checked={rule.OnMatch === 'stop'}
                  onchange={() => (rule.OnMatch = 'stop')}
                  {disabled}
                />
                Stop
              </label>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`match_${i}`}
                  value="goto"
                  checked={rule.OnMatch === 'goto'}
                  onchange={() => (rule.OnMatch = 'goto')}
                  {disabled}
                />
                Goto
              </label>
              {#if rule.OnMatch === 'goto'}
                <SpinButton
                  bind:value={rule.OnMatchGoto}
                  min={1}
                  max={items.length}
                  {disabled}
                />
              {/if}
            </div>

            <div class="row-flex wrap-row">
              <span class="field-label">If fail:</span>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`fail_${i}`}
                  value="close"
                  checked={rule.OnFail === 'close'}
                  onchange={() => (rule.OnFail = 'close')}
                  {disabled}
                />
                Close session
              </label>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`fail_${i}`}
                  value="stop"
                  checked={rule.OnFail === 'stop'}
                  onchange={() => (rule.OnFail = 'stop')}
                  {disabled}
                />
                Stop
              </label>
              <label class="radio-label">
                <input
                  type="radio"
                  name={`fail_${i}`}
                  value="goto"
                  checked={rule.OnFail === 'goto'}
                  onchange={() => (rule.OnFail = 'goto')}
                  {disabled}
                />
                Goto
              </label>
              {#if rule.OnFail === 'goto'}
                <SpinButton
                  bind:value={rule.OnFailGoto}
                  min={1}
                  max={items.length}
                  {disabled}
                />
              {/if}

              <span style="margin-left: 16px;">Timeout (sec):</span>
              <SpinButton
                bind:value={rule.Timeout}
                min={0}
                max={3600}
                {disabled}
              />
            </div>
          </div>
        </div>
      {/each}
    {/if}
  </div>

  <div class="footer-actions">
    <Button
      label="Add Expect Rule"
      icon="gtk-add"
      {disabled}
      onclick={addRule}
    />
  </div>
</div>

<style>
  .expect-tab-container {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
  }

  .expect-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .empty-hint {
    padding: 24px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
  }

  .expect-card {
    border: 1px solid var(--gtk-border-color);
    border-radius: 4px;
    background: var(--gtk-base-color, #ffffff);
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--gtk-border-color);
    padding-bottom: 4px;
  }

  .header-actions {
    display: flex;
    gap: 4px;
  }

  .card-body {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 4px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .wrap-row {
    flex-wrap: wrap;
    gap: 10px;
  }

  .field-label {
    min-width: 60px;
    font-size: 0.9em;
    font-weight: 500;
  }

  .radio-label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 0.9em;
    cursor: pointer;
  }

  .footer-actions {
    padding-top: 4px;
  }
</style>
