<script>
  import { drag, page, shell, state as go } from '@/sdk/svelte.svelte.js';
  import * as g from './glide.svelte.js';

  // The pages: the active tab as a card inset in the frame, or the tabs of its split side
  // by side, 8px apart, the active one ringed in the accent; a press in a pane's page makes
  // it the active one, as Go does by itself. The frame round them is
  // painted in strips, so nothing is painted where a page shows; four corner pieces round
  // each card. The band over the cards drags the window. hidden: the sidebar is hidden,
  // so the cards keep the frame's 8px on every side. The cards' corners are concentric
  // with the window's: its radius less the 8px inset (18 on macOS 26), and the design
  // system's 8 where the window has square corners, as in full screen.
  let { hidden } = $props();

  const radius = $derived(Math.max(go.cornerRadius - 8, 8));

  const ids = $derived(g.shown());
  const split = $derived(g.splitOf(go.active));
  const sizes = $derived(split ? split.sizes : [1]);

  // No pane narrower than this while a divider is dragged.
  const narrowest = 240;
  let row = $state();
  let resizing = null;

  function grab(e, i) {
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    const room = row.getBoundingClientRect().width - (ids.length - 1) * 8;
    resizing = { i, room, from: e.clientX, a: split.sizes[i], b: split.sizes[i + 1] };
  }

  function move(e) {
    if (!resizing) return;
    const { i, room, from, a, b } = resizing;
    const least = narrowest / room;
    const d = Math.min(Math.max((e.clientX - from) / room, least - a), b - least);
    split.sizes[i] = a + d;
    split.sizes[i + 1] = b - d;
  }

  function release() {
    if (!resizing) return;
    resizing = null;
    g.save();
  }
</script>

<section class="panes" class:full={hidden} style:--card-radius="{radius}px">
  <div class="band top" use:drag></div>
  <div class="row" bind:this={row}>
    {#if hidden}<div class="band side"></div>{/if}
    {#each ids as id, i (id)}
      {#if i > 0}
        <div class="band gap" use:shell role="separator" aria-orientation="vertical" aria-label="Resize" onpointerdown={(e) => grab(e, i - 1)} onpointermove={move} onpointerup={release} onpointercancel={release}></div>
      {/if}
      <div class="card" style:flex="{sizes[i] ?? 1} 1 0" class:focused={ids.length > 1 && id === go.active}>
        <main use:page={id}></main>
        <span class="corner tl"></span><span class="corner tr"></span><span class="corner bl"></span><span class="corner br"></span>
        <span class="ring"></span>
      </div>
    {/each}
    <div class="band side"></div>
  </div>
  <div class="band bottom"></div>
</section>

<style>
  .panes {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .band {
    flex: none;
    background: var(--g-frame);
  }

  .band.top,
  .band.bottom {
    height: 8px;
  }

  .band.side,
  .band.gap {
    width: 8px;
  }

  .band.gap {
    cursor: col-resize;
  }

  .row {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .card {
    position: relative;
    min-width: 0;
    display: flex;
  }

  main {
    flex: 1;
  }

  /* The corners, in the frame's colour, cut round on the page's side. */
  .corner {
    position: absolute;
    width: var(--card-radius);
    height: var(--card-radius);
    pointer-events: none;
  }

  .tl {
    top: 0;
    left: 0;
    background: radial-gradient(circle at 100% 100%, transparent calc(var(--card-radius) - 0.5px), var(--g-frame) var(--card-radius));
  }

  .tr {
    top: 0;
    right: 0;
    background: radial-gradient(circle at 0 100%, transparent calc(var(--card-radius) - 0.5px), var(--g-frame) var(--card-radius));
  }

  .bl {
    bottom: 0;
    left: 0;
    background: radial-gradient(circle at 100% 0, transparent calc(var(--card-radius) - 0.5px), var(--g-frame) var(--card-radius));
  }

  .br {
    bottom: 0;
    right: 0;
    background: radial-gradient(circle at 0 0, transparent calc(var(--card-radius) - 0.5px), var(--g-frame) var(--card-radius));
  }

  /* A hairline round the card, drawn outside the page, on the frame. */
  .ring {
    position: absolute;
    inset: 0;
    border-radius: var(--card-radius);
    box-shadow: 0 0 0 1px var(--g-line);
    pointer-events: none;
  }

  .focused .ring {
    box-shadow: 0 0 0 2px var(--accent);
  }

</style>
