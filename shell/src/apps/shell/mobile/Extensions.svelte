<script>
  import { answerExtensionPrompt, manageExtensions, pinExtension, pressExtension } from '@/sdk/extensions.js';
  import { shell } from '@/sdk/svelte.svelte.js';
  import Sheet from './Sheet.svelte';

  // The extensions, from the menu: a row each, which opens it, with its pin, then Manage
  // extensions; and the install or removal prompt over the window. The classes and labels
  // are what tools/drive_extensions.py finds them by (docs/shells.md).
  let { extensions, open, onclose } = $props();

  function modal(dialog) {
    dialog.showModal();
  }
</script>

{#snippet icon(b)}
  {#if b.icon}<img src={b.icon} alt="" width="20" height="20" />{:else}<span class="letter">{b.name.slice(0, 1).toUpperCase()}</span>{/if}
{/snippet}

{#if open}
  <Sheet label="Extensions" {onclose}>
    <div class="pbx-menu" role="group" aria-label="Extensions menu">
      {#each extensions.buttons as b (b.id)}
        <div class="row">
          <button
            class="item pbx-action"
            class:pbx-open={extensions.popup === b.id}
            data-extension={b.id}
            disabled={!b.enabled}
            aria-label={b.title || b.name}
            onclick={(e) => pressExtension(b.id, e.currentTarget)}
          >
            {@render icon(b)}
            <span class="name">{b.name}</span>
            {#if b.badge}<span class="pbx-badge">{b.badge.slice(0, 4)}</span>{/if}
          </button>
          <button class="pin" aria-label="{b.pinned ? 'Unpin' : 'Pin'} {b.name}" aria-pressed={b.pinned} onclick={() => pinExtension(b.id, !b.pinned)}>{b.pinned ? 'Pinned' : 'Pin'}</button>
        </div>
      {/each}
      <button class="item pbx-manage" onclick={() => { onclose(); manageExtensions(); }}>Manage extensions</button>
    </div>
  </Sheet>
{/if}

{#if extensions.prompt}
  {@const p = extensions.prompt}
  <div class="cover" use:shell>
    <dialog class="pbx-prompt" use:modal oncancel={(e) => { e.preventDefault(); answerExtensionPrompt(false); }} aria-label="{p.remove ? 'Remove' : 'Add'} {p.name}">
      <h2>{p.remove ? 'Remove' : 'Add'} "{p.name}"?</h2>
      {#if !p.remove}
        <p class="secondary">Version {p.version}</p>
        {#if p.permissions?.length}
          <ul>{#each p.permissions as permission (permission)}<li>{permission}</li>{/each}</ul>
        {/if}
      {/if}
      <div class="answers">
        <button class="text-button" onclick={() => answerExtensionPrompt(false)}>Cancel</button>
        <button class="text-button primary" onclick={() => answerExtensionPrompt(true)}>{p.remove ? 'Remove' : 'Add extension'}</button>
      </div>
    </dialog>
  </div>
{/if}

<style>
  .pbx-menu {
    display: flex;
    flex-direction: column;
    padding: 4px 0;
  }

  .row {
    display: flex;
    align-items: center;
  }

  .item {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 16px;
    height: 48px;
    padding: 0 20px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: 14px;
    text-align: left;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .pbx-open {
    background: var(--accent-soft);
  }

  .pbx-badge {
    padding: 0 4px;
    border-radius: 4px;
    background: var(--accent);
    color: var(--on-accent);
    font-size: 9px;
    font-weight: 600;
    line-height: 14px;
  }

  .pin {
    flex: none;
    height: 48px;
    margin-right: 12px;
    padding: 0 12px;
    border: 1px solid var(--line);
    border-radius: 20px;
    background: none;
    color: var(--text);
    font-size: 13px;
  }

  .pin[aria-pressed='true'] {
    border-color: transparent;
    background: var(--accent-soft);
  }

  .letter {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    border-radius: 4px;
    background: var(--accent);
    color: var(--on-accent);
    font-size: 12px;
    font-weight: 600;
  }

  .cover {
    position: absolute;
    inset: 0;
    z-index: 20;
  }

  dialog {
    width: min(360px, calc(100% - 32px));
    padding: 20px;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-float);
  }

  dialog::backdrop {
    background: color-mix(in srgb, var(--bar) 70%, transparent);
  }

  h2 {
    margin: 0 0 8px;
    font-size: 15px;
    font-weight: 600;
  }

  ul {
    margin: 8px 0;
    padding-left: 20px;
    font-size: 13px;
  }

  .answers .text-button {
    height: 48px;
  }

  .answers {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
