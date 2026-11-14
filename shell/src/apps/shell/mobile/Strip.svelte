<script>
  import { shell, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import { groupLabel } from './groups.js';

  // The group strip, while the active tab is in a group: the group's dot, which opens its
  // sheet; its tabs as favicons, the active one on a pill with an × on it; and + for a new
  // tab in the group. A tap on a favicon goes to its tab, a long press closes it.
  let { group, tabs, active, onsheet, onpick, onclosetab, onnewtab } = $props();
  let strip;

  // The active tab scrolled into view.
  $effect(() => {
    active;
    strip?.querySelector('.on')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  });
</script>

<div class="strip" use:shell role="group" aria-label="Group {groupLabel(group)}">
  <button class="dot" aria-label="Group {groupLabel(group)}" aria-haspopup="dialog" onclick={onsheet}>
    <i style:background={group.color}></i>
  </button>
  <div class="tabs" bind:this={strip}>
    {#each tabs as tab (tab.id)}
      <span class="slot" class:on={tab.id === active}>
        <button
          class="tab"
          data-tab={tab.id}
          aria-label={tabLabel(tab)}
          aria-current={tab.id === active ? 'page' : undefined}
          onclick={() => onpick(tab.id)}
          oncontextmenu={(e) => {
            e.preventDefault();
            onclosetab(tab.id);
          }}
        ><Favicon {tab} /></button>
        {#if tab.id === active}
          <button class="x" aria-label="Close {tabLabel(tab)}" onclick={() => onclosetab(tab.id)}><Icon name="close" size={10} /></button>
        {/if}
      </span>
    {/each}
    <button class="add" aria-label="New tab in group" onclick={onnewtab}><Icon name="plus" /></button>
  </div>
</div>

<style>
  .strip {
    flex: none;
    display: flex;
    align-items: center;
    height: 44px;
    padding: 0 4px;
    box-sizing: border-box;
    background: var(--bar);
    border-top: 1px solid var(--line);
  }

  .tabs {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }

  button {
    flex: none;
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    padding: 0;
    border: 0;
    border-radius: 12px;
    background: none;
    color: var(--icon);
  }

  .dot i {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .slot {
    position: relative;
    flex: none;
  }

  .slot.on .tab {
    background: var(--surface);
    box-shadow: 0 0 0 1px var(--line);
  }

  /* The × on the active favicon: a small round badge, its target 24px. */
  .x {
    position: absolute;
    top: -2px;
    right: -6px;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: none;
  }

  .x :global(svg) {
    box-sizing: content-box;
    padding: 2px;
    border-radius: 50%;
    background: var(--text);
    color: var(--bar);
    stroke-width: 3;
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
</style>
