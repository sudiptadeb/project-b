<script>
  import { activeTab, connect, focusPage, page, send, setFrame, state } from '@/sdk/svelte.svelte.js';
  import Address from './Address.svelte';
  import CrashBar from './CrashBar.svelte';
  import SideList from './SideList.svelte';
  import TabBar from './TabBar.svelte';
  import Toolbar from './Toolbar.svelte';

  // The minimal shell: a tab bar, a toolbar under it, and the page, square and flat. With
  // the tabs down the side the tab bar goes, the toolbar is the top row, and the tabs are a
  // list on the left. The bars and the page reach the window's edges: it asks for no band.
  setFrame(0);

  const horizontal = $derived(state.tabLayout === 'horizontal');
  let address;
  // The tab last made active, so each change of it is seen once.
  let active = 0;

  // A tab that becomes active with nothing in it wants the address; one with a page wants
  // the page. What was typed in the field was for the tab before. The shell has to hold
  // the keyboard before its field can.
  function activated() {
    if (state.active === active) return;
    active = state.active;
    address.blur();
    if (activeTab(state).url === '') {
      send({ t: 'focusShell' });
      address.focus();
    } else {
      focusPage();
    }
  }

  connect({
    snapshot: activated,
    tabs: activated,
    focusAddressBar: () => address.focus(),
    suggestions: (m) => address.suggestions(m),
  });
</script>

<div class="window">
  {#if horizontal}
    <TabBar />
  {/if}
  <Toolbar vertical={!horizontal}>
    <Address bind:this={address} url={activeTab(state).url} />
  </Toolbar>
  <CrashBar />
  <div class="body">
    {#if !horizontal}
      <SideList />
    {/if}
    <main use:page={state.active}></main>
  </div>
</div>

<style>
  .window {
    display: flex;
    flex-direction: column;
    height: 100%;
    font: 13px/1 var(--sans);
    color: var(--text);
  }

  .body {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  main {
    flex: 1;
    min-width: 0;
  }
</style>
