<script>
  import { answerDialog, dialogFor, focusPage, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';

  // A page's alert, confirm, prompt, leave-page question or sign-in (docs/daily-driver.md),
  // over the card of the tab that asks, so the sidebar and other cards still work. Go words
  // it; the page waits until it is answered. tab: the card's tab.
  let { tab } = $props();

  const dialog = $derived(dialogFor(go, tab));
  let text = $state('');
  let password = $state('');
  let card = $state();
  let field = $state();
  let ok = $state();
  // Where the host asking sits in a sign-in's words, set in bold; -1 for none.
  const hostAt = $derived(dialog?.host ? dialog.message.indexOf(dialog.host) : -1);

  // Each dialog starts afresh, takes the keyboard while its tab is the active one, and
  // answers no to Escape.
  $effect(() => {
    const d = dialog;
    if (!d) return;
    text = d.defaultText || '';
    password = '';
    if (tab !== go.active) return;
    const pop = pushEscape(() => answer(false));
    send({ t: 'focusShell' });
    queueMicrotask(() => {
      if (field) {
        field.focus();
        field.select();
      } else ok?.focus();
    });
    return pop;
  });

  function answer(yes) {
    const d = dialog;
    if (!d) return;
    answerDialog(d.id, yes, yes ? text : '', yes ? password : '');
    focusPage();
  }

  // Tab goes round the card's own controls.
  function trap(e) {
    if (e.key !== 'Tab') return;
    const all = [...card.querySelectorAll('input, button')];
    const i = all.indexOf(document.activeElement);
    e.preventDefault();
    all[(i + (e.shiftKey ? -1 : 1) + all.length) % all.length]?.focus();
  }

  function enter(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      answer(true);
    }
  }
</script>

{#if dialog}
  <div class="scrim" use:shell>
    {#key dialog.id}
      <div bind:this={card} class="card" role="alertdialog" aria-modal="true" aria-label={dialog.title} tabindex="-1" onkeydown={trap}>
        <h2>{dialog.title}</h2>
        {#if dialog.message}
          <p class="words">
            {#if hostAt >= 0}{dialog.message.slice(0, hostAt)}<b>{dialog.host}</b>{dialog.message.slice(hostAt + dialog.host.length)}{:else}{dialog.message}{/if}
          </p>
        {/if}
        {#if dialog.kind === 'prompt'}
          <input bind:this={field} bind:value={text} class="text-field field" type="text" aria-label={dialog.message || 'Answer'} onkeydown={enter} />
        {/if}
        {#if dialog.kind === 'signin'}
          {#if dialog.realm}<p class="realm">The site says: “{dialog.realm}”</p>{/if}
          <div class="group">
            <label for="g-sign-in-user-{tab}">Username</label>
            <input id="g-sign-in-user-{tab}" bind:this={field} bind:value={text} class="text-field" type="text" autocomplete="username" spellcheck="false" onkeydown={enter} />
          </div>
          <div class="group">
            <label for="g-sign-in-password-{tab}">Password</label>
            <input id="g-sign-in-password-{tab}" bind:value={password} class="text-field" type="password" autocomplete="current-password" onkeydown={enter} />
          </div>
          {#if dialog.insecure}<p class="insecure">This connection isn't secure: your password will be sent as plain text.</p>{/if}
        {/if}
        <div class="buttons">
          {#if dialog.cancel}<button type="button" class="text-button" onclick={() => answer(false)}>{dialog.cancel}</button>{/if}
          <button bind:this={ok} type="button" class="text-button primary" onclick={() => answer(true)}>{dialog.ok}</button>
        </div>
      </div>
    {/key}
  </div>
{/if}

<style>
  /* Over the card only: the page waits, the rest of the window doesn't. */
  .scrim {
    position: absolute;
    inset: 0;
    z-index: 5;
    display: grid;
    place-items: start center;
    padding-top: 64px;
    /* The card's own corners, concentric with the window's. */
    border-radius: var(--card-radius);
    background: var(--g-scrim);
  }

  .card {
    width: 360px;
    max-width: calc(100% - 32px);
    padding: 20px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-float);
    outline: none;
  }

  h2 {
    margin: 0 0 16px;
    font-size: 15px;
    font-weight: 600;
  }

  .words {
    margin: 0;
    color: var(--muted);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 50vh;
    overflow: auto;
  }

  .field {
    width: 100%;
    margin: 0;
    box-sizing: border-box;
  }

  .words + .field {
    margin-top: 16px;
  }

  .realm {
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .group {
    margin-top: 12px;
  }

  .realm + .group,
  .words + .group {
    margin-top: 16px;
  }

  .group label {
    display: block;
    margin: 0 0 4px;
    color: var(--muted);
    font-size: 12px;
  }

  .group .text-field {
    width: 100%;
    box-sizing: border-box;
  }

  .insecure {
    margin: 16px 0 0;
    color: var(--danger);
    font-size: 12px;
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
