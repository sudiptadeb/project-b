<script>
  import { tabLabel } from '@/sdk/svelte.svelte.js';

  // A tab's favicon, or the first letter of its label on the accent's tint while it has none.
  let { tab, size = 20 } = $props();
</script>

{#if tab.favicon}
  <img src={tab.favicon} alt="" width={size} height={size} />
{:else}
  <span class="letter" style:width="{size}px" style:height="{size}px" style:font-size="{Math.round(size * 0.6)}px">{tabLabel(tab).slice(0, 1).toUpperCase()}</span>
{/if}

<style>
  img {
    flex: none;
    object-fit: contain;
  }

  .letter {
    flex: none;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: var(--accent-soft);
    color: var(--text);
    font-weight: 600;
    line-height: 1;
  }
</style>
