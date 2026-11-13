<script>
  import { controlSpace, drag, send, shell, state } from '@/sdk/svelte.svelte.js';
  import { hint } from './hint.js';
  import TabList from './TabList.svelte';

  // The tabs on top: the native window buttons' space, the tabs, New Tab, then empty space
  // that moves the window, never less than 48px. The tabs scroll sideways, and the SDK
  // doesn't measure on a scroll, so the bar is the shell's and only its two spaces drag.
</script>

<div class="tabbar" use:shell>
  <div class="space" use:drag style:width={controlSpace(state).left}></div>
  <TabList tabs={state.tabs} active={state.active} />
  <button class="new" aria-label="New Tab" title={hint('New Tab', 'tab.new')} onclick={() => send({ t: 'newTab' })}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12" /></svg>
  </button>
  <div class="drag" use:drag></div>
</div>

<style>
  /* 34px of tabs and a hairline under them, which the active tab covers to join the
     toolbar. */
  .tabbar {
    display: flex;
    align-items: flex-start;
    flex: none;
    height: 35px;
    background: var(--tabbar);
    box-shadow: inset 0 -1px var(--line);
  }

  .space {
    flex: none;
    align-self: stretch;
  }

  .new {
    flex: none;
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  .new:hover {
    background: var(--hover);
  }

  .new:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .drag {
    flex: 1 0 48px;
    align-self: stretch;
  }
</style>
