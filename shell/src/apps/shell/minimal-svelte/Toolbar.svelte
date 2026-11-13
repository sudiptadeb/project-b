<script>
  import { activeTab, controlSpace, drag, focusPage, send, shell, state } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import Extensions from './Extensions.svelte';
  import { hint } from './hint.js';

  // Back, Forward, Reload (Stop while the page loads), the address field it is given,
  // filling the rest, the extensions, Relaunch when an update is ready, and Settings. Its
  // gaps move the window. With the tabs down the side it is the top row, after the window
  // buttons' space.
  let { vertical, children } = $props();
  const tab = $derived(activeTab(state));

  // What acts on the page gives it the keyboard.
  function act(msg) {
    send(msg);
    focusPage();
  }
</script>

<div class="toolbar" use:shell use:drag style:padding-left={vertical ? controlSpace(state).left : null}>
  <button class="btn" aria-label="Back" title={hint('Back', 'nav.back')} disabled={!tab.canGoBack} onclick={() => act({ t: 'back' })}>
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M10 3 5 8l5 5" /></svg>
  </button>
  <button class="btn" aria-label="Forward" title={hint('Forward', 'nav.forward')} disabled={!tab.canGoForward} onclick={() => act({ t: 'forward' })}>
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m6 3 5 5-5 5" /></svg>
  </button>
  {#if tab.loading}
    <button class="btn" aria-label="Stop" title={hint('Stop', 'nav.stop')} onclick={() => act({ t: 'run', text: 'nav.stop' })}>
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m4 4 8 8M12 4l-8 8" /></svg>
    </button>
  {:else}
    <button class="btn" aria-label="Reload" title={hint('Reload', 'nav.reload')} disabled={!tab.url} onclick={() => act({ t: 'reload' })}>
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M13 8a5 5 0 1 1-1.5-3.5M13 2.5V5h-2.5" /></svg>
    </button>
  {/if}
  {@render children()}
  {#if extensionsBuilt}
    <Extensions extensions={state.extensions} />
  {/if}
  {#if state.update.phase === 'ready'}
    <button class="words" title="Project B {state.update.version} is ready" onclick={() => send({ t: 'relaunch' })}>Relaunch to update</button>
  {/if}
  <button class="btn" aria-label="Settings" title={hint('Settings', 'app.settings')} onclick={() => send({ t: 'run', text: 'app.settings' })}>
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
  </button>
</div>

<style>
  /* Above what follows it, so the suggestions and the extensions' menu lie over the
     notice, the side list and the page. */
  .toolbar {
    position: relative;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 2px;
    flex: none;
    height: 38px;
    padding: 0 6px;
    background: var(--bar);
    border-bottom: 1px solid var(--line);
  }
</style>
