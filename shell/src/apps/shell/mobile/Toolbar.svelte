<script>
  import { drag, shell } from '@/sdk/svelte.svelte.js';
  import Address from './Address.svelte';
  import Icon from './Icon.svelte';

  // The one bar: the address pill filling it, the tab count, which opens the switcher, and
  // ⋯, which opens the menu; they stay while the address is edited, so the tabs are a tap
  // away from an empty new tab too. A swipe
  // sideways along it goes to the tab before or after (onswipe, -1 or 1).
  let { tab, count, top, sheet, onsheet, onswipe, address = $bindable() } = $props();
  let editing = $state(false);
  let start = null;

  function down(e) {
    if (editing) return;
    start = { x: e.clientX, y: e.clientY };
  }

  function up(e) {
    if (!start) return;
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    start = null;
    if (Math.abs(dx) > 48 && Math.abs(dx) > 2 * Math.abs(dy)) {
      // The press was a swipe, not a tap: the field mustn't take the keyboard for it.
      address.blur();
      onswipe(dx < 0 ? 1 : -1);
    }
  }
</script>

<div class="toolbar" role="toolbar" aria-label="Toolbar" class:top use:shell use:drag onpointerdown={down} onpointerup={up} onpointercancel={() => (start = null)}>
  <Address bind:this={address} {tab} {top} onedit={(on) => (editing = on)} />
  <button class="count" aria-label="Tabs" aria-haspopup="dialog" aria-expanded={sheet === 'switcher'} onclick={() => onsheet(sheet === 'switcher' ? '' : 'switcher')}>
    <span>{count > 99 ? ':D' : count}</span>
  </button>
  <button class="more" aria-label="Menu" aria-haspopup="menu" aria-expanded={sheet === 'menu'} onclick={() => onsheet(sheet === 'menu' ? '' : 'menu')}>
    <Icon name="more" />
  </button>
</div>

<style>
  .toolbar {
    position: relative;
    z-index: 3;
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 48px;
    padding: 0 4px 0 8px;
    box-sizing: border-box;
    background: var(--bar);
    border-top: 1px solid var(--line);
    touch-action: pan-y;
  }

  .toolbar.top {
    border-top: 0;
    border-bottom: 1px solid var(--line);
  }

  button {
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

  button:active {
    background: var(--press);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .count span {
    display: grid;
    place-items: center;
    min-width: 20px;
    height: 20px;
    padding: 0 2px;
    box-sizing: border-box;
    border: 1.5px solid currentColor;
    border-radius: 5px;
    font-size: 11px;
    font-weight: 600;
    line-height: 1;
  }
</style>
