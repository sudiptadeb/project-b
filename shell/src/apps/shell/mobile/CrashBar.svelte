<script>
  import { send, shell, state } from '@/sdk/svelte.svelte.js';

  // The last run's crash, in a line over the toolbar until it is dismissed.
</script>

{#if state.crashLog}
  <div class="crash" use:shell title={state.crashReport}>
    <span role="status">Project B crashed last time. The log is at {state.crashLog}</span>
    <button class="text-button quiet" onclick={() => send({ t: 'dismissCrash' })}>Dismiss</button>
  </div>
{/if}

<style>
  .crash {
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 49px;
    padding: 0 4px 0 16px;
    background: var(--bar);
    border-top: 1px solid var(--line);
    font-size: 12px;
  }

  button {
    height: 48px;
  }

  span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
