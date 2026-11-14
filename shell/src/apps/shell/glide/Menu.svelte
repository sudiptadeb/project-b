<script>
  import { keyLabel, shell, state as go } from '@/sdk/svelte.svelte.js';
  import * as g from './glide.svelte.js';

  // A menu at a point, over a cover that takes every other press and closes it: {x, y,
  // label, items}, or {anchor, ...} to open under or over an element. Up and Down move,
  // Enter chooses, Escape closes.
  let { menu } = $props();
  let box = $state();
  let left = $state(0);
  let top = $state(0);
  const items = $derived(menu.items);

  $effect(() => {
    const w = box.offsetWidth;
    const h = box.offsetHeight;
    let x = menu.x;
    let y = menu.y;
    if (menu.anchor) {
      const r = menu.anchor.getBoundingClientRect();
      x = r.left;
      y = r.bottom + 4 + h > innerHeight ? r.top - 4 - h : r.bottom + 4;
    }
    left = Math.max(8, Math.min(x, innerWidth - w - 8));
    top = Math.max(8, Math.min(y, innerHeight - h - 8));
    box.querySelector('[role=menuitem]')?.focus();
  });

  function choose(item) {
    if (item.disabled) return;
    g.ui.menu = null;
    item.run();
  }

  // Gone without a choice: the cover pressed, or Escape. A page's menu tells Go.
  function dismiss() {
    const m = menu;
    g.ui.menu = null;
    m.onclose?.();
  }

  function onkeydown(e) {
    const all = [...box.querySelectorAll('[role=menuitem]')];
    const i = all.indexOf(document.activeElement);
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      all[(i + (e.key === 'ArrowDown' ? 1 : -1) + all.length) % all.length]?.focus();
    } else if (e.key === 'Escape') {
      dismiss();
    }
  }
</script>

<div class="cover" use:shell role="presentation" onpointerdown={dismiss} oncontextmenu={(e) => e.preventDefault()}></div>
<div bind:this={box} class="menu" use:shell role="menu" aria-label={menu.label} tabindex="-1" style:left="{left}px" style:top="{top}px" {onkeydown}>
  {#each items as item, i (i)}
    {#if item.separator}
      <div class="sep" role="separator"></div>
    {:else}
      {@const key = item.action ? keyLabel(go, item.action) : ''}
      <button role="menuitem" class="item" disabled={item.disabled} aria-disabled={item.disabled ? 'true' : undefined} onclick={() => choose(item)}>
        <span>{item.label}</span>
        {#if key}<span class="key">{key}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .cover {
    position: fixed;
    inset: 0;
  }

  .menu {
    position: fixed;
    min-width: 200px;
    max-width: 320px;
    padding: 4px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-float);
    outline: none;
  }

  .item {
    display: flex;
    align-items: center;
    gap: 16px;
    width: 100%;
    height: 28px;
    padding: 0 8px;
    border: 0;
    border-radius: 6px;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    white-space: nowrap;
  }

  .item span:first-child {
    flex: 1;
  }

  .item:hover,
  .item:focus-visible {
    outline: none;
    background: var(--accent-tint);
  }

  .key {
    color: var(--muted);
    font-size: 12px;
  }

  .sep {
    height: 1px;
    margin: 4px 8px;
    background: var(--line);
  }

  /* A separator first, last, or after another isn't drawn. */
  .sep:first-child,
  .sep:last-child,
  .sep + .sep {
    display: none;
  }

  .item:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
