<script>
  import {
    activeTab, connect, controlSpace, drag, focusPage, keyLabel, page, send, shell, state, tabLabel,
  } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import Extensions from './Extensions.svelte';

  // The barest shell on the SDK, in Svelte: every control plain, with no design. The tabs
  // and New Tab, then a row of back, forward, reload, the address, the extensions and
  // Settings, both in one bar whose gaps move the window; any notice; then the page, to
  // the window's edge: it asks for no band.
  let address;
  // The tab last made active, so each change of it is seen once.
  let active = 0;

  // A tab that becomes active with nothing in it wants the address; one with a page wants
  // the page. The shell has to hold the keyboard before its field can.
  function activated() {
    if (state.active === active) return;
    active = state.active;
    if (activeTab(state).url === '') {
      send({ t: 'focusShell' });
      address.focus();
    } else {
      address.blur();
      focusPage();
    }
  }

  connect({ snapshot: activated, tabs: activated, focusAddressBar: () => address.focus() });

  function act(msg) {
    send(msg);
    focusPage();
  }

  function go(e) {
    if (e.key !== 'Enter' || address.value.trim() === '') return;
    send({ t: 'navigate', text: address.value.trim() });
    address.blur();
    focusPage();
  }

  const newTab = $derived(keyLabel(state, 'tab.new') ? `New Tab (${keyLabel(state, 'tab.new')})` : 'New Tab');
</script>

<header use:shell use:drag style:padding-left={controlSpace(state).left} style:padding-right={controlSpace(state).right}>
  <div class="row">
    {#each state.tabs as tab (tab.id)}
      <span>
        <button role="tab" data-tab={tab.id} aria-selected={tab.id === state.active} onclick={() => send({ t: 'activateTab', id: tab.id })}>{tabLabel(tab)}</button>
        <button aria-label="Close {tabLabel(tab)}" onclick={() => send({ t: 'closeTab', id: tab.id })}>×</button>
      </span>
    {/each}
    <button title={newTab} onclick={() => send({ t: 'newTab' })}>New Tab</button>
  </div>
  <div class="row">
    <button disabled={!activeTab(state).canGoBack} onclick={() => act({ t: 'back' })}>Back</button>
    <button disabled={!activeTab(state).canGoForward} onclick={() => act({ t: 'forward' })}>Forward</button>
    {#if activeTab(state).loading}
      <button onclick={() => act({ t: 'run', text: 'nav.stop' })}>Stop</button>
    {:else}
      <button disabled={!activeTab(state).url} onclick={() => act({ t: 'reload' })}>Reload</button>
    {/if}
    <input
      bind:this={address}
      aria-label="Address"
      value={activeTab(state).url}
      onkeydown={go}
      onfocus={() => address.select()}
      onblur={() => (address.value = activeTab(state).url)}
    />
    {#if extensionsBuilt}
      <Extensions extensions={state.extensions} />
    {/if}
    <button onclick={() => send({ t: 'run', text: 'app.settings' })}>Settings</button>
  </div>
</header>
{#if state.update.phase === 'ready'}
  <p use:shell>Project B {state.update.version} is ready. <button onclick={() => send({ t: 'relaunch' })}>Relaunch to update</button></p>
{/if}
{#if state.crashLog}
  <p use:shell>Project B crashed last time. The log is at {state.crashLog}. <button onclick={() => send({ t: 'dismissCrash' })}>Close</button></p>
{/if}
<main use:page={state.active}></main>

<style>
  :global(#app) {
    display: flex;
    flex-direction: column;
  }

  header,
  p {
    margin: 0;
    padding: 4px 8px;
    background: var(--bar);
    color: var(--text);
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
  }

  input {
    flex: 1;
    min-width: 120px;
  }

  main {
    flex: 1;
  }
</style>
