<script lang="ts">
  // Shows dialogs requested by the backend ("dialog:request"): passwords, key
  // passphrases, keyboard-interactive/OTP questions, host key decisions and
  // <ASK:...> variables. Requests are queued and answered one at a time.
  import { onMount } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import { getIcon } from '../../lib/theme/icons';
  import { sessions, setActiveTab } from '../../lib/stores/sessions';
  import * as DialogService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/dialogservice.js';

  interface PromptField {
    name: string;
    label: string;
    secret: boolean;
    value?: string;
  }

  interface PromptRequest {
    id: string;
    tmpID?: string;
    kind: string;
    title: string;
    message: string;
    fields?: PromptField[];
    options?: string[];
    remember?: boolean;
  }

  let queue = $state<PromptRequest[]>([]);
  let current = $derived(queue[0]);
  let values = $state<Record<string, string>>({});
  let choice = $state('');
  let remember = $state(false);
  let showSecret = $state(false);

  // Host key decisions and confirmations show their options as buttons.
  let isHostKey = $derived(
    current?.kind === 'hostkey-unknown' || current?.kind === 'hostkey-changed' || current?.kind === 'confirm'
  );
  let isChoiceList = $derived(!isHostKey && (current?.options?.length ?? 0) > 0);
  let sessionTitle = $derived(
    current?.tmpID ? $sessions.find((s) => s.tmpID === current?.tmpID)?.title ?? '' : ''
  );

  // Reset the form whenever a new request becomes current.
  let shownId = '';
  $effect(() => {
    if (!current || current.id === shownId) return;
    shownId = current.id;
    const v: Record<string, string> = {};
    for (const f of current.fields ?? []) v[f.name] = f.value ?? '';
    values = v;
    choice = current.options?.[0] ?? '';
    remember = false;
    showSecret = false;
    // Bring the affected terminal to the front so the user sees what is asked for.
    if (current.tmpID) setActiveTab(current.tmpID);
  });

  function payload(ev: any): any {
    return ev && typeof ev === 'object' && 'data' in ev ? ev.data : ev;
  }

  onMount(() => {
    const offReq = Events.On('dialog:request', (ev: any) => {
      const req = payload(ev) as PromptRequest;
      if (req?.id) queue = [...queue, req];
    });
    const offCancel = Events.On('dialog:cancel', (ev: any) => {
      const id = payload(ev)?.id;
      if (id) queue = queue.filter((r) => r.id !== id);
    });
    return () => {
      offReq();
      offCancel();
    };
  });

  function finish(resp: { cancelled: boolean; choice?: string }) {
    const req = current;
    if (!req) return;
    queue = queue.slice(1);
    DialogService.Respond(req.id, {
      cancelled: resp.cancelled,
      choice: resp.choice ?? '',
      values: { ...values },
      remember,
    } as any).catch((err: unknown) => console.warn('Respond failed:', err));
  }

  function submit() {
    if (isChoiceList) {
      finish({ cancelled: false, choice });
    } else {
      finish({ cancelled: false, choice: 'ok' });
    }
  }

  function onFieldKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      submit();
    }
  }

  function iconFor(kind: string): string {
    if (kind === 'hostkey-changed') return 'gtk-dialog-warning';
    if (kind === 'hostkey-unknown' || kind === 'confirm') return 'gtk-dialog-question';
    return 'asbru-protected';
  }
</script>

{#if current}
  <Dialog
    open={true}
    title={current.title || 'LeanTerm'}
    width={isHostKey ? 560 : 420}
    class={current.kind === 'hostkey-changed' ? 'w-error' : ''}
    onclose={() => finish({ cancelled: true })}
  >
    <div class="prompt-body">
      <img src={getIcon(iconFor(current.kind))} alt="" width="32" height="32" class="prompt-icon" />
      <div class="prompt-main">
        {#if sessionTitle}
          <div class="prompt-session">{sessionTitle}</div>
        {/if}
        <p class="prompt-message">{current.message}</p>

        {#if isChoiceList}
          <select class="prompt-choice" bind:value={choice} onkeydown={onFieldKey}>
            {#each current.options ?? [] as opt}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        {:else}
          {#each current.fields ?? [] as field (field.name)}
            <label class="prompt-field">
              <span>{field.label}</span>
              <input
                type={field.secret && !showSecret ? 'password' : 'text'}
                bind:value={values[field.name]}
                autocomplete="off"
                spellcheck="false"
                onkeydown={onFieldKey}
              />
            </label>
          {/each}
          {#if (current.fields ?? []).some((f) => f.secret)}
            <div class="prompt-options">
              <CheckButton label="Show" bind:checked={showSecret} />
              {#if current.remember}
                <CheckButton label="Remember (store encrypted)" bind:checked={remember} />
              {/if}
            </div>
          {/if}
        {/if}
      </div>
    </div>

    {#snippet actions()}
      {#if isHostKey}
        {#each current?.options ?? [] as opt, i}
          <Button
            label={opt}
            isDefault={i === 0 && current?.kind !== 'hostkey-changed'}
            onclick={() => finish({ cancelled: false, choice: opt })}
          />
        {/each}
      {:else}
        <Button label="Cancel" onclick={() => finish({ cancelled: true })} />
        <Button label="OK" isDefault={true} onclick={submit} />
      {/if}
    {/snippet}
  </Dialog>
{/if}

<style>
  .prompt-body {
    display: flex;
    gap: 14px;
    align-items: flex-start;
  }
  .prompt-icon {
    flex-shrink: 0;
    margin-top: 2px;
  }
  .prompt-main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .prompt-session {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
  }
  .prompt-message {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.4;
    user-select: text;
  }
  .prompt-field {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .prompt-field input,
  .prompt-choice {
    width: 100%;
  }
  .prompt-options {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
  }
</style>
