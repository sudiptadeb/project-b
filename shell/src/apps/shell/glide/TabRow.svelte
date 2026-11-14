<script>
  import { send, state, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import { tabMenu } from './menus.js';
  import * as g from './glide.svelte.js';

  // One tab in the sidebar: its icon, its title, and its close button on the active row
  // and while pointed at. A press makes it active, the middle button closes it, and the
  // right button opens its menu. depth indents it in a folder.
  let { tab, depth = 0 } = $props();
  const label = $derived(tabLabel(tab));
  const selected = $derived(tab.id === state.active);

  function onauxclick(e) {
    if (e.button === 1) g.closeTab(tab);
  }

  function close(e) {
    e.stopPropagation();
    g.closeTab(tab);
  }
</script>

<div
  class="g-row tab"
  class:active={selected}
  role="tab"
  tabindex="0"
  data-tab={tab.id}
  aria-selected={selected}
  title={label}
  style:padding-left="{8 + depth * 20}px"
  onclick={() => send({ t: 'activateTab', id: tab.id })}
  onkeydown={(e) => e.key === 'Enter' && send({ t: 'activateTab', id: tab.id })}
  {onauxclick}
  oncontextmenu={(e) => {
    e.preventDefault();
    g.ui.menu = { x: e.clientX, y: e.clientY, label: label, items: tabMenu(tab) };
  }}
>
  <Favicon {tab} />
  <span class="label">{label}</span>
  {#if tab.loading}<span class="loading" aria-label="Loading" role="img"></span>{/if}
  <button class="g-icon x" aria-label="Close {label}" title="Close" onclick={close}><Icon name="close" size={14} /></button>
</div>

<style>
  .x {
    width: 24px;
    height: 24px;
    color: inherit;
  }

  .tab:not(.active, :hover, :focus-within) .x {
    display: none;
  }

  .loading {
    flex: none;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
  }
</style>
