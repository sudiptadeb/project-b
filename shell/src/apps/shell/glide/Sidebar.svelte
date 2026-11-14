<script>
  import { extensionsBuilt } from '@extensions';
  import { activeTab, controlSpace, drag, send, shell, state, tabLabel } from '@/sdk/svelte.svelte.js';
  import Extensions from './Extensions.svelte';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import Nav from './Nav.svelte';
  import TabRow from './TabRow.svelte';
  import { favoriteMenu, folderMenu, newSpace, pinMenu, spaceMenu, tabMenu } from './menus.js';
  import * as g from './glide.svelte.js';
  import { hint } from './hint.js';

  // The sidebar, top to bottom: the window's buttons and the navigation, the address, the
  // Space's Favorites, its name and menu, its pinned tabs, New Tab, its tabs with the
  // folders and splits among them, and at the foot the extensions, the Spaces, New Space
  // and Settings. Its right edge drags to resize it.
  let { onaddress, oncopy, onnew, onclosebar } = $props();

  const tab = $derived(activeTab(state));
  const host = $derived(g.hostOf(tab.url) || tab.url);
  const space = $derived(g.space());
  const favorites = $derived(g.favorites());
  const pins = $derived(g.pins());
  const entries = $derived(g.listing());
  let menuButton;

  function openMenu(e, label, items) {
    e.preventDefault();
    g.ui.menu = { x: e.clientX, y: e.clientY, label, items };
  }

  function toggle(group) {
    send({ t: 'editGroup', group: group.id, collapsed: !group.collapsed });
  }

  // The edge: a drag sets the width, 184 to 400; a double press puts it back to 240.
  let resizing = null;
  function grab(e) {
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    resizing = { from: e.clientX, width: g.win.width };
  }
  function move(e) {
    if (resizing) g.win.width = Math.round(Math.min(400, Math.max(184, resizing.width + e.clientX - resizing.from)));
  }
  function release() {
    if (!resizing) return;
    resizing = null;
    g.save();
  }
</script>

{#snippet entry(e, depth)}
  {#if e.kind === 'tab'}
    <TabRow tab={e.tab} {depth} />
  {:else if e.kind === 'split'}
    <div class="split" style:margin-left="{depth * 20}px" class:active={e.tabs.some((t) => t.id === state.active)}>
      <span class="split-icon" title="Split View"><Icon name="split" size={14} /></span>
      {#each e.tabs as t (t.id)}
        <div
          class="part"
          class:on={t.id === state.active}
          role="tab"
          tabindex="0"
          data-tab={t.id}
          aria-selected={t.id === state.active}
          title={tabLabel(t)}
          onclick={() => send({ t: 'activateTab', id: t.id })}
          onkeydown={(ev) => ev.key === 'Enter' && send({ t: 'activateTab', id: t.id })}
          oncontextmenu={(ev) => openMenu(ev, tabLabel(t), tabMenu(t))}
        >
          <Favicon tab={t} size={14} />
          <span class="label">{tabLabel(t)}</span>
          <button class="g-icon x" aria-label="Close {tabLabel(t)}" onclick={(ev) => (ev.stopPropagation(), g.closeTab(t))}><Icon name="close" size={12} /></button>
        </div>
      {/each}
    </div>
  {:else}
    {@const group = e.group}
    <button
      class="g-row folder"
      style:padding-left="{4 + depth * 20}px"
      aria-expanded={!group.collapsed}
      title={group.name || 'Folder'}
      onclick={() => toggle(group)}
      oncontextmenu={(ev) => openMenu(ev, group.name || 'Folder', folderMenu(group))}
    >
      <span class="chevron"><Icon name={group.collapsed ? 'chevronRight' : 'chevronDown'} size={14} /></span>
      <span class="folder-icon" style:color={group.color}><Icon name="folder" /></span>
      <span class="label">{group.name || 'Folder'}</span>
    </button>
    {#each e.children as child (child.key)}
      <!-- A closed folder still shows its active tab under it. -->
      {#if !group.collapsed || (child.kind === 'tab' ? child.tab.id === state.active : child.tabs.some((t) => t.id === state.active))}
        {@render entry(child, depth + 1)}
      {/if}
    {/each}
  {/if}
{/snippet}

<aside class="sidebar" use:shell style:width="{g.win.width}px" onpointerdowncapture={() => g.ui.bar && onclosebar()}>
  <div class="top" use:drag style:padding-left={controlSpace(state).left}>
    <span class="gap"></span>
    <button class="g-icon" aria-label="Hide Sidebar" title={hint('Hide Sidebar', 'glide.sidebar')} onclick={() => ((g.win.hidden = true), g.save())}><Icon name="sidebar" /></button>
    <Nav />
  </div>

  <div class="address">
    <button class="field" aria-haspopup="dialog" aria-label="Address bar, {host || 'empty'}" title={hint('Search or enter an address', 'address.focus')} onclick={onaddress}>
      <span class="host">{host || 'Search or enter an address'}</span>
    </button>
    {#if tab.url}
      <button class="g-icon copy" aria-label="Copy Address" title={hint('Copy Address', 'glide.copyAddress')} onclick={oncopy}><Icon name="link" /></button>
    {/if}
  </div>

  {#if favorites.length}
    <div class="favorites" role="group" aria-label="Favorites">
      {#each favorites as fav (fav.url)}
        {@const open = g.favoriteTab(fav)}
        <button
          class="fav"
          class:active={open && open.id === state.active}
          aria-label="Favorite {fav.title || fav.url}"
          title={fav.title || fav.url}
          onclick={() => g.openFavorite(fav)}
          oncontextmenu={(e) => openMenu(e, fav.title || fav.url, favoriteMenu(fav))}
        >
          <Favicon tab={fav} size={20} />
          {#if open}<span class="dot"></span>{/if}
        </button>
      {/each}
    </div>
  {/if}

  <div class="space-head">
    <span class="space-dot" style:background={space.color}></span>
    <span class="label">{space.name}</span>
    <button
      bind:this={menuButton}
      class="g-icon"
      aria-label="Space menu"
      aria-haspopup="menu"
      aria-expanded={g.ui.menu?.label === space.name + ' menu'}
      onclick={() => (g.ui.menu = { anchor: menuButton, label: space.name + ' menu', items: spaceMenu(space) })}
    ><Icon name="more" /></button>
  </div>

  <div class="list" role="tablist" aria-label="Tabs in {space.name}" aria-orientation="vertical">
    {#each pins as p (p.id)}
      {#if p.tab}
        <div class="pinned"><TabRow tab={p.tab} /></div>
      {:else}
        <div class="g-row unloaded" role="button" tabindex="0" title={p.title || p.url} onclick={() => g.openPin(p)} onkeydown={(e) => e.key === 'Enter' && g.openPin(p)} oncontextmenu={(e) => openMenu(e, p.title || p.url, pinMenu(p))}>
          <Favicon tab={p} />
          <span class="label">{p.title || p.url}</span>
          <button class="g-icon x" aria-label="Unpin {p.title || p.url}" title="Unpin" onclick={(e) => (e.stopPropagation(), g.unpin(p.id))}><Icon name="close" size={14} /></button>
        </div>
      {/if}
    {/each}
    {#if pins.length}<div class="divider" role="separator"></div>{/if}
    <button class="g-row new" title={hint('New Tab', 'tab.new')} onclick={onnew}><Icon name="plus" /><span class="label">New Tab</span></button>
    {#each entries as e (e.key)}
      {@render entry(e, 0)}
    {/each}
  </div>

  {#if state.update?.phase === 'ready'}
    <button class="g-row notice" onclick={() => send({ t: 'relaunch' })}><Icon name="update" /><span class="label">Relaunch to update to {state.update.version}</span></button>
  {/if}
  {#if state.crashLog}
    <div class="g-row notice">
      <Icon name="warn" /><span class="label" title={state.crashLog}>Closed unexpectedly last time</span>
      <button class="g-icon" aria-label="Dismiss" onclick={() => send({ t: 'dismissCrash' })}><Icon name="close" size={14} /></button>
    </div>
  {/if}

  <div class="foot">
    {#if extensionsBuilt}<Extensions extensions={state.extensions} />{/if}
    <div class="spaces" role="group" aria-label="Spaces">
      {#each g.spaces() as s, i (s.id)}
        <button
          class="g-icon space"
          class:on={s.id === space.id}
          aria-label="Space {s.name}"
          aria-pressed={s.id === space.id}
          title={hint(s.name, i < 9 ? 'glide.space' + (i + 1) : '')}
          onclick={() => g.showSpace(s.id)}
          oncontextmenu={(e) => openMenu(e, s.name, spaceMenu(s))}
        ><span class="space-dot" style:background={s.color}></span></button>
      {/each}
    </div>
    <button class="g-icon" aria-label="New Space" title={hint('New Space', 'glide.newSpace')} onclick={newSpace}><Icon name="plus" /></button>
    <button class="g-icon" aria-label="Settings" title={hint('Settings', 'app.settings')} onclick={() => send({ t: 'run', text: 'app.settings' })}><Icon name="gear" /></button>
  </div>
</aside>
<div class="edge" use:shell role="separator" aria-orientation="vertical" aria-label="Resize sidebar" onpointerdown={grab} onpointermove={move} onpointerup={release} onpointercancel={release} ondblclick={() => ((g.win.width = 240), g.save())}></div>

<style>
  .sidebar {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
    padding: 8px 0 8px 8px;
    box-sizing: border-box;
    background: var(--g-frame);
  }

  .edge {
    flex: none;
    width: 4px;
    background: var(--g-frame);
    cursor: col-resize;
  }

  .top {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 28px;
    margin-top: -2px;
  }

  .gap {
    flex: 1;
  }

  .address {
    position: relative;
    display: flex;
    align-items: center;
  }

  .field {
    flex: 1;
    min-width: 0;
    height: 36px;
    padding: 0 36px 0 12px;
    border: 0;
    border-radius: 8px;
    background: var(--g-field);
    color: var(--g-ink);
    font: inherit;
    text-align: left;
  }

  .field:hover {
    background: var(--g-row-hover);
  }

  .field:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .host {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .copy {
    position: absolute;
    right: 4px;
    width: 28px;
  }

  .favorites {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }

  .fav {
    position: relative;
    display: grid;
    place-items: center;
    height: 44px;
    padding: 0;
    border: 0;
    border-radius: 8px;
    background: var(--g-tile);
  }

  .fav:hover {
    background: var(--g-row-hover);
  }

  .fav.active {
    background: var(--g-row-active);
  }

  .fav .dot {
    position: absolute;
    bottom: 5px;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--g-icon);
  }

  .space-head {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 32px;
    padding-left: 8px;
    font-weight: 600;
  }

  .space-head .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .space-dot {
    flex: none;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    box-shadow: 0 0 0 1px var(--g-line);
  }

  .list {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: -4px;
    overflow: hidden auto;
    scrollbar-width: none;
  }

  .list > :global(*) {
    flex: none;
  }

  .unloaded {
    color: var(--g-ink-muted);
  }

  .unloaded:not(:hover, :focus-within) .x {
    display: none;
  }

  .x {
    width: 24px;
    height: 24px;
    color: inherit;
  }

  .divider {
    height: 1px;
    margin: 6px 8px;
    background: var(--g-line);
  }

  .new {
    color: var(--g-ink-muted);
  }

  .folder .chevron,
  .folder .folder-icon {
    display: grid;
    place-items: center;
  }

  .folder .chevron {
    color: var(--g-icon);
    width: 16px;
  }

  /* A split: one row, a part for each of its tabs. */
  .split {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 32px;
    padding: 0 4px 0 8px;
    border-radius: 8px;
  }

  .split.active {
    background: var(--g-row-active);
    color: var(--g-active-ink);
  }

  .split-icon {
    display: grid;
    place-items: center;
    color: var(--g-icon);
  }

  .part {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    padding: 0 2px 0 6px;
    border-radius: 6px;
    white-space: nowrap;
  }

  .part + .part {
    box-shadow: -1px 0 0 var(--g-line);
  }

  .part:hover {
    background: var(--g-row-hover);
  }

  .part.on {
    font-weight: 600;
  }

  .part .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .part:not(:hover) .x {
    display: none;
  }

  .part .x {
    width: 20px;
    height: 20px;
  }

  .notice {
    color: var(--g-ink);
    background: var(--g-tile);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 28px;
  }

  .spaces {
    flex: 1;
    min-width: 0;
    display: flex;
    justify-content: center;
    gap: 4px;
    overflow: hidden;
  }

  .space.on {
    background: var(--g-row-active);
  }
</style>
