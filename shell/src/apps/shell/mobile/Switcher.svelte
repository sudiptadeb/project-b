<script>
  import { shell, state, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import { gesture } from './gesture.js';
  import { groupLabel, groupOf } from './groups.js';

  // The tab switcher, over the page: the workspaces as chips while there are two or more, which
  // choose the cards shown, and New tab; then two columns of cards, the active one ringed.
  // A tab's card: a tap shows it, its × or a swipe sideways closes it, a long press opens
  // its menu (oncard), and held and dragged onto another card it joins that one's group or
  // starts one with it (ondrop). A
  // group is one card where its first tab is, with its colour along the top and its tabs'
  // favicons, a tap on which goes to that tab; a tap elsewhere on it opens the group's
  // sheet. It stays in the document while closed, so every tab is always an element
  // (docs/shells.md, rule 4).
  let { open, groups, workspace = $bindable(), onpick, onclosetab, onnewtab, oncard, ongroup, onclosegroup, ondrop } = $props();

  // Where a card was let go: on a group's card, or one of its tabs, it joins that group;
  // on a tab's card, that tab's group, or a new one of the two.
  function dropped(id, target) {
    if (target.dataset.group) return ondrop(id, { group: target.dataset.group });
    const other = Number(target.dataset.tab);
    const g = groupOf(groups, other);
    ondrop(id, g ? { group: g.id } : { tab: other });
  }

  // What the grid shows, in Go's order: a tab, or a group at its first tab.
  const items = $derived.by(() => {
    const out = [];
    const done = new Set();
    for (const tab of state.tabs) {
      const g = groupOf(groups, tab.id);
      if (!g) out.push({ key: `t${tab.id}`, tab });
      else if (!done.has(g.id)) {
        done.add(g.id);
        out.push({ key: `g${g.id}`, group: g, tabs: g.tabs.map((id) => state.tabs.find((t) => t.id === id)).filter(Boolean) });
      }
    }
    return out;
  });

  function menu(e, tab) {
    e.preventDefault();
    oncard(tab.id);
  }

  const workspaces = $derived(state.workspaces || []);
  const host = (url) => {
    try {
      return new URL(url).host.replace(/^www\./, '') || url;
    } catch {
      return url;
    }
  };
</script>

<div class="switcher" hidden={!open} use:shell role="dialog" aria-label="Tabs">
  <div class="head">
    {#if workspaces.length > 1}
      <div class="chips" role="radiogroup" aria-label="Workspace">
        {#each workspaces as w (w.id)}
          <button class="chip" role="radio" aria-checked={workspace === w.id} onclick={() => (workspace = w.id)}>
            {#if w.color}<span class="dot" style:background={w.color}></span>{/if}<span>{w.name}</span>
          </button>
        {/each}
      </div>
    {:else}
      <span class="spacer"></span>
    {/if}
    <button class="new" aria-label="New Tab" onclick={onnewtab}><Icon name="plus" /></button>
  </div>
  <div class="grid">
    {#each items as item (item.key)}
      {#if item.tab}
        {@const tab = item.tab}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="card"
          class:active={tab.id === state.active}
          hidden={(tab.workspace || '') !== workspace}
          role="tab"
          tabindex="0"
          data-tab={tab.id}
          aria-selected={tab.id === state.active}
          onclick={() => onpick(tab.id)}
          oncontextmenu={(e) => menu(e, tab)}
          use:gesture={{ onclose: () => onclosetab(tab.id), onmenu: () => oncard(tab.id), ondrop: (target) => dropped(tab.id, target) }}
          onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onpick(tab.id)}
        >
          <div class="top">
            <span class="title">{tabLabel(tab)}</span>
            <button class="close" aria-label="Close {tabLabel(tab)}" onclick={(e) => { e.stopPropagation(); onclosetab(tab.id); }}><Icon name="close" size={16} /></button>
          </div>
          <div class="body"><Favicon {tab} size={32} /></div>
          <span class="host">{tab.url ? host(tab.url) : 'New tab'}</span>
        </div>
      {:else}
        {@const g = item.group}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="card group"
          class:active={g.tabs.includes(state.active)}
          hidden={(item.tabs[0]?.workspace || '') !== workspace}
          style:--group={g.color}
          role="button"
          tabindex="0"
          aria-label="Group {groupLabel(g)}"
          data-group={g.id}
          onclick={() => ongroup(g.id)}
          onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && ongroup(g.id)}
        >
          <div class="top">
            <span class="title">{groupLabel(g)}</span>
            <button class="close" aria-label="Close group {groupLabel(g)}" onclick={(e) => { e.stopPropagation(); onclosegroup(g.id); }}><Icon name="close" size={16} /></button>
          </div>
          <div class="body cluster">
            {#each item.tabs as tab, i (tab.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <span
                class="member"
                hidden={i > 3}
                role="tab"
                tabindex="-1"
                data-tab={tab.id}
                aria-label={tabLabel(tab)}
                aria-selected={tab.id === state.active}
                onclick={(e) => { e.stopPropagation(); onpick(tab.id); }}
                oncontextmenu={(e) => { e.stopPropagation(); menu(e, tab); }}
              ><Favicon {tab} size={28} /></span>
            {/each}
          </div>
          <span class="host">{g.name ? `${g.tabs.length} tabs` : ''}</span>
        </div>
      {/if}
    {/each}
  </div>
</div>

<style>
  .switcher {
    position: absolute;
    inset: 0;
    z-index: 4;
    display: flex;
    flex-direction: column;
    background: var(--bar);
    animation: rise 200ms ease-out;
  }

  .switcher[hidden] {
    display: none;
  }

  .head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 4px 0 12px;
  }

  .chips {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 8px;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .spacer {
    flex: 1;
  }

  /* A chip is a 32px pill in a 48px target. */
  .chip {
    position: relative;
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    height: 48px;
    padding: 0 12px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: 13px;
    font-weight: 500;
  }

  .chip::before {
    content: '';
    position: absolute;
    inset: 8px 0;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
  }

  .chip[aria-checked='true']::before {
    border-color: transparent;
    background: var(--accent-soft);
  }

  .chip > span {
    position: relative;
  }

  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .new {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    border-radius: 20px;
    background: none;
    color: var(--icon);
  }

  .grid {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    display: grid;
    align-content: start;
    gap: 12px;
    padding: 0 12px 12px;
    /* A tablet's is the phone's, wider. */
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  }

  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: var(--surface);
    overflow: hidden;
    outline: none;
  }

  .card {
    touch-action: pan-y;
  }

  .card:global(.lifted) {
    z-index: 2;
    box-shadow: var(--shadow-float);
    transition: box-shadow 150ms;
  }

  .card[hidden] {
    display: none;
  }

  .card.active {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .card.group.active {
    border-top-color: var(--group);
  }

  .card:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .top {
    flex: none;
    display: flex;
    align-items: center;
    height: 48px;
    padding-left: 12px;
  }

  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
    font-size: 13px;
    font-weight: 500;
  }

  .close {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  /* With the title and the host about it, the card is 1:1.1. */
  .card.group {
    border-top: 4px solid var(--group);
  }

  .cluster {
    grid-template-columns: 1fr 1fr;
    gap: 4px;
    padding: 8px;
  }

  .member {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
  }

  .member[hidden] {
    display: none;
  }

  .body {
    aspect-ratio: 5 / 4;
    display: grid;
    place-items: center;
    margin: 0 8px;
    border-radius: 8px;
    background: var(--accent-tint);
  }

  .host {
    flex: none;
    height: 28px;
    padding: 0 12px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: 12px;
    line-height: 28px;
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  @keyframes rise {
    from { transform: translateY(24px); opacity: 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    .switcher { animation: none; }
  }
</style>
