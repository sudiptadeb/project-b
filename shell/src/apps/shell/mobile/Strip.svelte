<script>
  import { shell, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import { groupLabel } from './groups.js';

  // The group strip, while the active tab is in a group: the group's dot, which opens its
  // sheet; its tabs as favicons, the active one on a pill with an × on it; and + for a new
  // tab in the group. A tap on a favicon goes to its tab, a tap on the active one closes
  // it, and a long press closes any.
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
      <!-- The active one closes at a tap: the whole 48px favicon is its close button, the
           × on it only says so. -->
      <button
        class="tab"
        class:on={tab.id === active}
        data-tab={tab.id}
        aria-label={tab.id === active ? `Close ${tabLabel(tab)}` : `Go to ${tabLabel(tab)}`}
        aria-current={tab.id === active ? 'page' : undefined}
        onclick={() => (tab.id === active ? onclosetab(tab.id) : onpick(tab.id))}
        oncontextmenu={(e) => {
          e.preventDefault();
          onclosetab(tab.id);
        }}
      >
        <Favicon {tab} />
        {#if tab.id === active}<span class="x" aria-hidden="true"><Icon name="close" size={10} /></span>{/if}
      </button>
    {/each}
    <button class="add" aria-label="New tab in group" onclick={onnewtab}><Icon name="plus" /></button>
  </div>
</div>

<style>
  /* Every target is 48px square, Android's least for a finger. */
  .strip {
    flex: none;
    display: flex;
    align-items: center;
    height: 49px;
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
    height: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }

  button {
    position: relative;
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    border-radius: 14px;
    background: none;
    color: var(--icon);
  }

  .dot i {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  /* The active favicon on a pill inside its target. */
  .tab.on::before {
    content: '';
    position: absolute;
    inset: 4px;
    border-radius: 12px;
    background: var(--surface);
    box-shadow: 0 0 0 1px var(--line);
  }

  .tab :global(img),
  .tab :global(.letter) {
    position: relative;
  }

  .x {
    position: absolute;
    top: 0;
    right: 0;
    display: grid;
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
