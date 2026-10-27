<script lang="ts">
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  interface Props {
    locked: boolean;
    onunlock?: () => void;
  }

  let { locked = false, onunlock }: Props = $props();

  let password = $state<string>('');
  let errorMsg = $state<string>('');
  let isChecking = $state<boolean>(false);

  async function handleUnlock() {
    if (!password) {
      errorMsg = 'Please enter password';
      return;
    }
    isChecking = true;
    errorMsg = '';
    try {
      await SystemService.UnlockGUI(password);
      password = '';
      if (onunlock) onunlock();
    } catch (err: any) {
      errorMsg = err?.message || 'Incorrect master password';
    } finally {
      isChecking = false;
    }
  }
</script>

{#if locked}
  <div class="lock-overlay">
    <div class="lock-card">
      <div class="lock-header">
        <span class="icon">🔒</span>
        <h2>LeanTerm Locked</h2>
      </div>
      <p class="subtitle">Please enter your Master Password to unlock the interface:</p>

      <div class="input-row">
        <Entry
          bind:value={password}
          type="password"
          placeholder="Master Password..."
          onkeydown={(e) => {
            if (e.key === 'Enter') handleUnlock();
          }}
        />
        <Button
          label="Unlock"
          icon="gtk-apply"
          disabled={isChecking || !password}
          onclick={handleUnlock}
        />
      </div>

      {#if errorMsg}
        <div class="error-msg">{errorMsg}</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .lock-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(8px);
    z-index: 99999;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .lock-card {
    background: var(--bg-primary, #2d2d2d);
    border: 1px solid var(--border-color, #555);
    border-radius: 8px;
    padding: 24px 32px;
    width: 420px;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.7);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .lock-header {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .lock-header .icon {
    font-size: 28px;
  }

  .lock-header h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-color, #eee);
  }

  .subtitle {
    margin: 0;
    font-size: 13px;
    color: var(--text-muted, #aaa);
  }

  .input-row {
    display: flex;
    gap: 8px;
  }

  .error-msg {
    font-size: 12px;
    color: var(--gtk-error-fg);
    background: rgba(255, 100, 100, 0.1);
    padding: 6px 10px;
    border-radius: 4px;
    border-left: 3px solid #ff6b6b;
  }
</style>
