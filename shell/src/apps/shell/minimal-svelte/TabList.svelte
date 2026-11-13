<script>
  import Tab from './Tab.svelte';

  // The tabs in Go's order, in a strip that scrolls: sideways in the tab bar, where a wheel
  // turns it too, and down in the side list. The active tab is kept in view as it changes,
  // as tabs come and go before it, and as the strip is resized.
  let { tabs, active, vertical = false } = $props();
  let strip;
  let width = $state(0);
  let height = $state(0);
  const index = $derived(tabs.findIndex((t) => t.id === active));

  // The scroll that shows the stretch from start, length long, in a view size long now
  // scrolled to scroll, moving it as little as it takes.
  const into = (scroll, size, start, length) => Math.min(Math.max(scroll, start + length - size), start);

  // There is no active tab before the snapshot.
  $effect(() => {
    const cell = strip.children[index];
    if (!cell) return;
    if (vertical) strip.scrollTop = into(strip.scrollTop, height, cell.offsetTop, cell.offsetHeight);
    else strip.scrollLeft = into(strip.scrollLeft, width, cell.offsetLeft, cell.offsetWidth);
  });

  function wheel(e) {
    if (!vertical && Math.abs(e.deltaY) > Math.abs(e.deltaX)) strip.scrollLeft += e.deltaY;
  }
</script>

<div
  bind:this={strip}
  bind:clientWidth={width}
  bind:clientHeight={height}
  class={vertical ? 'list' : 'tabs'}
  role="tablist"
  aria-label="Tabs"
  aria-orientation={vertical ? 'vertical' : 'horizontal'}
  onwheel={wheel}
>
  {#each tabs as tab (tab.id)}
    <Tab {tab} selected={tab.id === active} />
  {/each}
</div>

<style>
  .tabs,
  .list {
    position: relative;
    scrollbar-width: none;
  }

  /* Each tab 200px wide until they no longer fit, then narrower, down to 96px, and then
     the strip scrolls. A hairline before the first. */
  .tabs {
    display: flex;
    align-items: flex-start;
    flex: 0 1 auto;
    min-width: 0;
    height: 35px;
    overflow: auto hidden;
    border-left: 1px solid var(--line);
  }

  .list {
    flex: 1;
    min-height: 0;
    overflow: hidden auto;
  }
</style>
