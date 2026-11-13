<script>
  import { send, shell, state } from '@/sdk/svelte.svelte.js';
  import { hint } from './hint.js';
  import TabList from './TabList.svelte';

  // The tabs down the side: New Tab at the top, and the tabs under it in a list that
  // scrolls.
</script>

<div class="side" use:shell>
  <button class="new" title={hint('New Tab', 'tab.new')} onclick={() => send({ t: 'newTab' })}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12" /></svg>
    New Tab
  </button>
  <TabList tabs={state.tabs} active={state.active} vertical />
</div>

<style>
  .side {
    display: flex;
    flex-direction: column;
    flex: none;
    width: 220px;
    background: var(--tabbar);
    border-right: 1px solid var(--line);
  }

  .new {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
    height: 35px;
    padding: 0 12px;
    box-sizing: border-box;
    border: 0;
    border-bottom: 1px solid var(--line);
    background: none;
    color: var(--muted);
    font: inherit;
  }

  .new svg {
    color: var(--icon);
  }

  .new:hover {
    background: var(--hover);
  }

  .new:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }
</style>
