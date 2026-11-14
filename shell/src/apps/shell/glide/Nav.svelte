<script>
  import { activeTab, send, state } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import { hint } from './hint.js';

  // Back, Forward, and Reload, which is Stop while the page loads.
  const tab = $derived(activeTab(state));
</script>

<button class="g-icon" aria-label="Back" title={hint('Back', 'nav.back')} disabled={!tab.canGoBack} onclick={() => send({ t: 'back' })}><Icon name="back" /></button>
<button class="g-icon" aria-label="Forward" title={hint('Forward', 'nav.forward')} disabled={!tab.canGoForward} onclick={() => send({ t: 'forward' })}><Icon name="forward" /></button>
{#if tab.loading}
  <button class="g-icon" aria-label="Stop" title={hint('Stop', 'nav.stop')} onclick={() => send({ t: 'run', text: 'nav.stop' })}><Icon name="close" /></button>
{:else}
  <button class="g-icon" aria-label="Reload" title={hint('Reload', 'nav.reload')} onclick={() => send({ t: 'reload' })}><Icon name="reload" /></button>
{/if}
