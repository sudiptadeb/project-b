<script>
  import { tabLabel } from '@/sdk/svelte.svelte.js';

  // A tab's or a saved address's icon: its favicon, else the first letter of its label on
  // the accent's tint.
  let { tab, size = 16 } = $props();
  const letter = $derived((tab.title || tabLabel(tab)).slice(0, 1).toUpperCase());
</script>

{#if tab.favicon}
  <img src={tab.favicon} alt="" width={size} height={size} />
{:else}
  <span class="letter" style:width="{size}px" style:height="{size}px" style:font-size="{Math.round(size * 0.62)}px">{letter}</span>
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
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
    line-height: 1;
  }
</style>
