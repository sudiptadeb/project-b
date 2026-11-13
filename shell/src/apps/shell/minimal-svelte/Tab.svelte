<script>
  import { send, state, tabLabel, workspaceOf } from '@/sdk/svelte.svelte.js';

  // One tab: its workspace's colour when it has one, its label, a mark while it loads, and
  // its close button, always on the active tab and on the others while pointed at.
  let { tab, selected } = $props();
  const label = $derived(tabLabel(tab));
  const workspace = $derived(workspaceOf(state, tab));

  const activate = () => send({ t: 'activateTab', id: tab.id });

  function close(e) {
    e.stopPropagation();
    send({ t: 'closeTab', id: tab.id });
  }
</script>

<div
  class="tab"
  class:active={selected}
  role="tab"
  tabindex="0"
  data-tab={tab.id}
  aria-selected={selected}
  title={label}
  onclick={activate}
  onkeydown={(e) => e.key === 'Enter' && activate()}
>
  {#if workspace}
    <span class="dot" style:background={workspace.color} role="img" aria-label="In {workspace.name}"></span>
  {/if}
  <span class="t">{label}</span>
  {#if tab.loading}
    <!-- SVG's own animation turns it, so the layout isn't measured every frame while a page
         loads, as it would be during a CSS animation. -->
    <svg class="load" viewBox="0 0 10 10" role="img" aria-label="Loading">
      <rect x="2" y="2" width="6" height="6">
        <animateTransform attributeName="transform" type="rotate" from="0 5 5" to="90 5 5" dur="0.6s" repeatCount="indefinite" />
      </rect>
    </svg>
  {/if}
  <button class="x" aria-label="Close {label}" onclick={close}>✕</button>
</div>

<style>
  /* A cell in the tab bar, a hairline after it. The active one has the toolbar's tone and
     reaches 1px lower, over the bar's hairline, to join the toolbar. */
  .tab {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 1 auto;
    width: 200px;
    min-width: 96px;
    height: 34px;
    padding: 0 10px;
    box-sizing: border-box;
    border-right: 1px solid var(--line);
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
  }

  .tab.active {
    height: 35px;
    background: var(--bar);
    color: var(--text);
    font-weight: 500;
  }

  .tab:not(.active):hover {
    background: var(--hover);
  }

  .tab:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  /* In the side list: a row, a hairline under it. */
  :global(.side) .tab,
  :global(.side) .tab.active {
    width: auto;
    min-width: 0;
    height: 32px;
    border-right: 0;
    border-bottom: 1px solid var(--line);
  }

  .t {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dot {
    flex: none;
    width: 6px;
    height: 6px;
  }

  .load {
    flex: none;
    width: 10px;
    height: 10px;
    fill: var(--accent);
  }

  .x {
    flex: none;
    display: grid;
    place-items: center;
    width: 16px;
    height: 16px;
    margin-right: -4px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
    font: 11px/1 var(--sans);
  }

  .x:hover {
    background: var(--hover);
  }

  .x:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .tab:not(.active, :hover, :focus-within) .x {
    display: none;
  }
</style>
