<script>
  import { shell } from '@/sdk/svelte.svelte.js';

  // A bottom sheet over the window: the backdrop, a tap on which closes it, and the panel,
  // which a swipe down on its handle closes too. label names it for a screen reader.
  let { label, onclose, children } = $props();
  let start = null;

  function down(e) {
    start = e.clientY;
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function up(e) {
    if (start !== null && e.clientY - start > 40) onclose();
    start = null;
  }
</script>

<div class="backdrop" use:shell onclick={onclose} role="presentation"></div>
<div class="sheet" use:shell role="dialog" aria-label={label}>
  <div class="handle" onpointerdown={down} onpointerup={up} onpointercancel={() => (start = null)} role="presentation"><i></i></div>
  {@render children()}
</div>

<style>
  .backdrop {
    position: absolute;
    inset: 0;
    z-index: 10;
    background: color-mix(in srgb, var(--bar) 70%, transparent);
  }

  .sheet {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 11;
    max-height: 85%;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding-bottom: 8px;
    background: var(--surface);
    border-radius: 16px 16px 0 0;
    box-shadow: var(--shadow-float);
    animation: rise 200ms ease-out;
  }

  .handle {
    display: grid;
    place-items: center;
    height: 20px;
    touch-action: none;
  }

  .handle i {
    width: 32px;
    height: 4px;
    border-radius: 2px;
    background: var(--line);
  }

  @keyframes rise {
    from { transform: translateY(100%); }
  }

  @media (prefers-reduced-motion: reduce) {
    .sheet { animation: none; }
  }
</style>
