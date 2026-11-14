<script>
  import { activeTab, connect, drag, focusPage, page, send, setFrame, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import CrashBar from './CrashBar.svelte';
  import Extensions from './Extensions.svelte';
  import MenuSheet from './MenuSheet.svelte';
  import Switcher from './Switcher.svelte';
  import Toolbar from './Toolbar.svelte';

  // The phone's shell: the page, and under it one bar, the address, the tab count and ⋯.
  // Everything else is a sheet one of them opens: the switcher over the page, the menu
  // from the bottom. The page reaches the window's edges: no band.
  setFrame(0);

  const tab = $derived(activeTab(go));
  // What is open over the page: '', 'switcher', 'menu' or 'extensions'.
  let sheet = $state('');
  // The workspace new tabs open in, and the switcher shows: the active tab's, until the
  // user picks another.
  let workspace = $state('');
  let address = $state();
  // The tab last made active, so each change of it is seen once.
  let active = 0;
  // Space for the native window buttons, which a phone hasn't: a desktop window, as on
  // the mock, keeps the page from under them.
  const clear = $derived(go.controls?.h > 0 ? go.controls.y + go.controls.h : 0);

  // A tab that becomes active with nothing in it wants the address; one with a page wants
  // the page. The shell has to hold the keyboard before its field can.
  function activated() {
    if (go.active === active) return;
    active = go.active;
    workspace = tab.workspace || '';
    address.blur();
    if (tab.url === '') {
      send({ t: 'focusShell' });
      address.focus();
    } else {
      focusPage();
    }
  }

  function newTab() {
    sheet = '';
    send({ t: 'newTab', workspace });
  }

  function pick(id) {
    sheet = '';
    if (id !== go.active) send({ t: 'activateTab', id });
    else focusPage();
  }

  // The window never has no tabs: closing the last one leaves a new one.
  function closeTab(id) {
    if (go.tabs.length === 1) {
      sheet = '';
      send({ t: 'newTab', workspace });
    }
    send({ t: 'closeTab', id });
  }

  // The tab before or after the active one.
  function swipe(by) {
    const i = go.tabs.findIndex((t) => t.id === go.active);
    const next = go.tabs[i + by];
    if (next) send({ t: 'activateTab', id: next.id });
  }

  function onsheet(name) {
    if (name === 'switcher') workspace = tab.workspace || '';
    sheet = name;
  }

  connect({
    snapshot: activated,
    tabs: activated,
    focusAddressBar: () => {
      sheet = '';
      address.focus();
    },
    suggestions: (m) => address.suggestions(m),
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && sheet && ((sheet = ''), e.preventDefault())} />

<div class="window">
  {#if clear}
    <div class="clear" use:shell use:drag style:height="{clear}px"></div>
  {/if}
  <div class="stage">
    <main use:page={go.active}></main>
    <Switcher open={sheet === 'switcher'} bind:workspace onpick={pick} onclosetab={closeTab} onnewtab={newTab} />
  </div>
  <CrashBar />
  <Toolbar {tab} count={go.tabs.length} {sheet} {onsheet} onswipe={swipe} bind:address />
  {#if sheet === 'menu'}
    <MenuSheet {tab} group={null} {workspace} onclose={() => (sheet = '')} onnewtab={newTab} {onsheet} onworkspace={(id) => (workspace = id)} />
  {/if}
  {#if extensionsBuilt && go.extensions}
    <Extensions extensions={go.extensions} open={sheet === 'extensions'} onclose={() => (sheet = '')} />
  {/if}
</div>

<style>
  .window {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    color: var(--text);
    font-size: 14px;
    -webkit-user-select: none;
    user-select: none;
  }

  .clear {
    flex: none;
    background: var(--bar);
  }

  .stage {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  main {
    flex: 1;
    min-height: 0;
  }

  :global(button) {
    font-family: inherit;
    -webkit-tap-highlight-color: transparent;
  }
</style>
