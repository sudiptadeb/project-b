<script>
  import { tabLabel } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';
  import { groupLabel } from './groups.js';

  // A card's menu, from a long press on it in the switcher: Add to group, which lists the
  // groups of its workspace and New group, Remove from group while it is in one, and Close.
  let { tab, group, groups, onclose, onadd, onnewgroup, onremove, onclosetab } = $props();
  let adding = $state(false);
</script>

<Sheet label="Tab" {onclose}>
  <div class="label">{tabLabel(tab)}</div>
  <div class="rows" role="menu" aria-label={tabLabel(tab)}>
    {#if adding}
      {#each groups as g (g.id)}
        <button class="row" role="menuitem" onclick={() => onadd(g.id)}>
          <span class="dot" style:background={g.color}></span>
          <span class="name">{groupLabel(g)}</span>
        </button>
      {/each}
      <button class="row" role="menuitem" onclick={onnewgroup}>
        <Icon name="plus" />
        <span class="name">New group</span>
      </button>
    {:else}
      <button class="row" role="menuitem" onclick={() => (adding = true)}>
        <Icon name="group" />
        <span class="name">Add to group</span>
        <Icon name="chevron" />
      </button>
      {#if group}
        <button class="row" role="menuitem" onclick={onremove}>
          <Icon name="tab" />
          <span class="name">Remove from group</span>
        </button>
      {/if}
      <button class="row" role="menuitem" onclick={onclosetab}>
        <Icon name="close" />
        <span class="name">Close</span>
      </button>
    {/if}
  </div>
</Sheet>

<style>
  .label {
    padding: 0 20px 8px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: 12px;
    border-bottom: 1px solid var(--line);
  }

  .rows {
    display: flex;
    flex-direction: column;
    padding: 4px 0;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 16px;
    height: 48px;
    padding: 0 20px;
    border: 0;
    background: none;
    color: var(--icon);
    font-size: 14px;
    text-align: left;
  }

  .name {
    flex: 1;
    color: var(--text);
  }

  .dot {
    width: 10px;
    height: 10px;
    margin: 0 5px;
    border-radius: 50%;
  }

  button:active {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
