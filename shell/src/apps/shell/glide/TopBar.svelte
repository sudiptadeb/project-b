<script>
  import { activeTab, controlSpace, drag, send, shell, state } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Nav from './Nav.svelte';
  import * as g from './glide.svelte.js';
  import { hint } from './hint.js';

  // With the sidebar hidden, a slim bar over the page keeps the window's buttons off it:
  // Show Sidebar, the navigation, and the address in the middle. Its gaps drag the window.
  let { onaddress, oncopy } = $props();
  const tab = $derived(activeTab(state));
  const host = $derived(g.hostOf(tab.url) || tab.url);
</script>

<div class="topbar" use:shell use:drag style:padding-left={controlSpace(state).left} style:padding-right={controlSpace(state).right}>
  <button class="g-icon" aria-label="Show Sidebar" title={hint('Show Sidebar', 'glide.sidebar')} onclick={() => ((g.win.hidden = false), g.save())}><Icon name="sidebar" /></button>
  <Nav />
  <span class="gap"></span>
  <button class="field" aria-haspopup="dialog" aria-label="Address bar, {host || 'empty'}" onclick={onaddress}>{host || 'Search or enter an address'}</button>
  {#if tab.url}<button class="g-icon" aria-label="Copy Address" onclick={oncopy}><Icon name="link" /></button>{/if}
  <span class="gap"></span>
  <button class="g-icon" aria-label="New Tab" title={hint('New Tab', 'tab.new')} onclick={() => send({ t: 'newTab', workspace: g.spaceId() })}><Icon name="plus" /></button>
  <button class="g-icon" aria-label="Settings" onclick={() => send({ t: 'run', text: 'app.settings' })}><Icon name="gear" /></button>
</div>

<style>
  .topbar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 2px;
    height: 36px;
    padding-top: 4px;
    background: var(--g-frame);
  }

  .gap {
    flex: 1;
  }

  .field {
    width: min(420px, 50%);
    height: 28px;
    padding: 0 12px;
    border: 0;
    border-radius: 8px;
    background: var(--g-field);
    color: var(--g-ink);
    font: inherit;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .field:hover {
    background: var(--g-row-hover);
  }
</style>
