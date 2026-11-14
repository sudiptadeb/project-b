<script>
  import { send, state as go } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // The ⋯ sheet: a row of the page's actions, Back while there is somewhere to go back to
  // (the phone's back key does it too), Forward, Reload or Stop, New tab, and New tab in
  // group while the tab is in one; then Workspaces, where new tabs open, while there are
  // two or more, Extensions while their flag is on, the update while one is ready, and Settings.
  // Each acts and closes the sheet, but Workspaces and Extensions, which open their own.
  let { tab, group, workspace, onclose, onnewtab, onsheet, onworkspace } = $props();
  let list = $state('');

  function act(msg) {
    send(msg);
    onclose();
  }

  const workspaces = $derived(go.workspaces || []);
  const ready = $derived(go.update?.phase === 'ready');
</script>

<Sheet label="Menu" {onclose}>
  {#if list === 'workspaces'}
    <div class="rows" role="menu" aria-label="Workspaces">
      {#each workspaces as w (w.id)}
        <button class="row" role="menuitemradio" aria-checked={workspace === w.id} onclick={() => { onworkspace(w.id); onclose(); }}>
          <span class="dot" style:background={w.color || 'var(--muted)'}></span>
          <span class="name">{w.name}</span>
          {#if workspace === w.id}<span class="tick">✓</span>{/if}
        </button>
      {/each}
    </div>
  {:else}
    <div class="actions">
      {#if tab.canGoBack}
        <button aria-label="Back" onclick={() => act({ t: 'back' })}><Icon name="back" /><span aria-hidden="true">Back</span></button>
      {/if}
      <button aria-label="Forward" disabled={!tab.canGoForward} onclick={() => act({ t: 'forward' })}><Icon name="forward" /><span aria-hidden="true">Forward</span></button>
      {#if tab.loading}
        <button aria-label="Stop" onclick={() => act({ t: 'run', text: 'nav.stop' })}><Icon name="stop" /><span aria-hidden="true">Stop</span></button>
      {:else}
        <button aria-label="Reload" disabled={!tab.url} onclick={() => act({ t: 'reload' })}><Icon name="reload" /><span aria-hidden="true">Reload</span></button>
      {/if}
      <button aria-label="New Tab" onclick={() => { onnewtab(false); onclose(); }}><Icon name="plus" /><span aria-hidden="true">New tab</span></button>
      {#if group}
        <button aria-label="New tab in group" onclick={() => { onnewtab(true); onclose(); }}><Icon name="group" /><span aria-hidden="true">In group</span></button>
      {/if}
    </div>
    <div class="rows" role="menu" aria-label="More">
      {#if workspaces.length > 1}
        <button class="row" role="menuitem" onclick={() => (list = 'workspaces')}>
          <Icon name="workspace" />
          <span class="name">Workspaces</span>
          <Icon name="chevron" />
        </button>
      {/if}
      {#if extensionsBuilt && go.extensions}
        <button class="row" role="menuitem" aria-label="Extensions" onclick={() => onsheet('extensions')}>
          <Icon name="extensions" />
          <span class="name">Extensions</span>
          <Icon name="chevron" />
        </button>
      {/if}
      {#if ready}
        <button class="row" role="menuitem" onclick={() => act({ t: 'relaunch' })}>
          <Icon name="update" />
          <span class="name">Install update to {go.update.version}</span>
        </button>
      {/if}
      <button class="row" role="menuitem" aria-label="Settings" onclick={() => act({ t: 'run', text: 'app.settings' })}>
        <Icon name="settings" />
        <span class="name">Settings</span>
      </button>
    </div>
  {/if}
</Sheet>

<style>
  .actions {
    display: flex;
    justify-content: space-around;
    padding: 4px 8px 8px;
    border-bottom: 1px solid var(--line);
  }

  .actions button {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 0;
    height: 56px;
    padding: 6px 0 0;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--icon);
  }

  .actions span {
    color: var(--muted);
    font-size: 12px;
  }

  .actions button:disabled {
    opacity: 0.4;
  }

  .rows {
    display: flex;
    flex-direction: column;
    padding: 4px 0;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 16px;
    height: 48px;
    padding: 0 20px;
    border: 0;
    background: none;
    color: var(--icon);
    text-align: left;
    font-size: 14px;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
  }

  .dot {
    width: 10px;
    height: 10px;
    margin: 0 5px;
    border-radius: 50%;
  }

  .tick {
    color: var(--accent);
  }

  button:active:not(:disabled) {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
