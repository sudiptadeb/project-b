<script>
  import { shell, state, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';

  // The tab switcher, over the page: the workspaces as chips while there are any, which
  // choose the cards shown, and New tab; then two columns of cards, a tab each, the active
  // one ringed. A tap on a card shows its tab, its × closes it. It stays in the document
  // while closed, so every tab is always an element (docs/shells.md, rule 4).
  let { open, workspace = $bindable(), onpick, onclosetab, onnewtab } = $props();

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
    {#if workspaces.length}
      <div class="chips" role="radiogroup" aria-label="Workspace">
        {#each [{ id: '', name: 'Default', color: '' }, ...workspaces] as w (w.id)}
          <button class="chip" role="radio" aria-checked={workspace === w.id} onclick={() => (workspace = w.id)}>
            {#if w.color}<span class="dot" style:background={w.color}></span>{/if}{w.name}
          </button>
        {/each}
      </div>
    {:else}
      <span class="spacer"></span>
    {/if}
    <button class="new" aria-label="New Tab" onclick={onnewtab}><Icon name="plus" /></button>
  </div>
  <div class="grid">
    {#each state.tabs as tab (tab.id)}
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
        onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onpick(tab.id)}
      >
        <div class="top">
          <span class="title">{tabLabel(tab)}</span>
          <button class="close" aria-label="Close {tabLabel(tab)}" onclick={(e) => { e.stopPropagation(); onclosetab(tab.id); }}><Icon name="close" size={16} /></button>
        </div>
        <div class="body"><Favicon {tab} size={32} /></div>
        <span class="host">{tab.url ? host(tab.url) : 'New tab'}</span>
      </div>
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

  .chip {
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    height: 32px;
    padding: 0 12px;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    font-size: 13px;
    font-weight: 500;
  }

  .chip[aria-checked='true'] {
    border-color: transparent;
    background: var(--accent-soft);
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
    width: 40px;
    height: 40px;
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

  .card[hidden] {
    display: none;
  }

  .card.active {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .card:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .top {
    flex: none;
    display: flex;
    align-items: center;
    height: 40px;
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
    width: 40px;
    height: 40px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  /* With the title and the host about it, the card is 1:1.1. */
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
