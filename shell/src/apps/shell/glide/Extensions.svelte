<script>
  import { answerExtensionPrompt, manageExtensions, pinExtension, pressExtension } from '@/sdk/extensions.js';
  import { shell } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // The extensions, from extensions, the SDK's state.extensions: a button for each pinned
  // one, then Extensions, whose menu lists them all to open, pin or unpin, and Manage
  // extensions; and the install or removal prompt over the window. The classes and labels
  // are what tools/drive_extensions.py finds them by (docs/shells.md).
  let { extensions } = $props();
  // Whether the menu is open: until Escape, a choice in it, or its button again.
  let menu = $state(false);
  // The Extensions button: a popup opened from the menu opens under it.
  let opener = $state();

  function open(id) {
    menu = false;
    pressExtension(id, opener);
  }

  function manage() {
    menu = false;
    manageExtensions();
  }

  function modal(dialog) {
    dialog.showModal();
  }

  // Escape answers the prompt No, and Go takes it away.
  function cancel(e) {
    e.preventDefault();
    answerExtensionPrompt(false);
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (menu = false)} />

{#snippet icon(b)}
  {#if b.icon}<img src={b.icon} alt="" width="16" height="16" />{:else}<span class="letter">{b.name.slice(0, 1).toUpperCase()}</span>{/if}
{/snippet}

{#if extensions}
  {#each extensions.buttons.filter((b) => b.pinned) as b (b.id)}
    <button
      class="g-icon pbx-action"
      class:pbx-open={extensions.popup === b.id}
      data-extension={b.id}
      disabled={!b.enabled}
      aria-label={b.title || b.name}
      title={b.title || b.name}
      onclick={(e) => pressExtension(b.id, e.currentTarget)}
    >
      {@render icon(b)}
      {#if b.badge}
        <span class="pbx-badge" style:background={b.badgeColor || null} style:color={b.badgeTextColor || null}>{b.badge.slice(0, 4)}</span>
      {/if}
    </button>
  {/each}
  <span class="anchor">
    <button bind:this={opener} class="g-icon" class:pbx-open={menu} aria-label="Extensions" title="Extensions" aria-expanded={menu} onclick={() => (menu = !menu)}>
      <Icon name="extensions" />
    </button>
    {#if menu}
      <div class="pbx-menu" use:shell role="group" aria-label="Extensions menu">
        {#each extensions.buttons as b (b.id)}
          <div class="row">
            <button class="item" disabled={!b.enabled} onclick={() => open(b.id)}>
              {@render icon(b)}
              <span class="name">{b.name}</span>
            </button>
            <button class="pin" class:on={b.pinned} aria-label="{b.pinned ? 'Unpin' : 'Pin'} {b.name}" title={b.pinned ? 'Unpin from the toolbar' : 'Pin to the toolbar'} onclick={() => pinExtension(b.id, !b.pinned)}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3.5h8" /><path d="M10 3.5h4v5.5l3.5 4.5h-11L10 9z" fill={b.pinned ? 'currentColor' : 'none'} /><path d="M12 13.5v7" /></svg>
            </button>
          </div>
        {/each}
        <button class="item pbx-manage" onclick={manage}>Manage extensions</button>
      </div>
    {/if}
  </span>
  {#if extensions.prompt}
    {@const p = extensions.prompt}
    <!-- The cover makes the whole window the shell's while it asks, so no press reaches the
         page. -->
    <div class="pbx-cover" use:shell>
      <dialog class="pbx-prompt" use:modal oncancel={cancel} aria-label="{p.remove ? 'Remove' : 'Add'} {p.name}">
        <h2>{p.remove ? 'Remove' : 'Add'} "{p.name}"?</h2>
        {#if !p.remove}
          <p>Version {p.version}</p>
          {#if p.permissions?.length}
            <ul>
              {#each p.permissions as permission (permission)}<li>{permission}</li>{/each}
            </ul>
          {/if}
        {/if}
        <div class="answers">
          <button class="text-button" onclick={() => answerExtensionPrompt(false)}>Cancel</button>
          <button class="text-button primary" onclick={() => answerExtensionPrompt(true)}>{p.remove ? 'Remove' : 'Add extension'}</button>
        </div>
      </dialog>
    </div>
  {/if}
{/if}

<style>
  img,
  .letter {
    display: block;
    flex: none;
    width: 16px;
    height: 16px;
  }

  .letter {
    display: grid;
    place-items: center;
    background: var(--accent);
    color: var(--on-accent);
    font-size: 10px;
    font-weight: 600;
    border-radius: 4px;
  }

  .pbx-action {
    position: relative;
  }

  /* A button whose popup is open, or Extensions with its menu open, stays pressed. */
  .pbx-open,
  .pbx-open:hover:not(:disabled) {
    background: var(--accent-soft);
  }

  .pbx-badge {
    position: absolute;
    right: 1px;
    bottom: 2px;
    min-width: 8px;
    height: 12px;
    padding: 0 2px;
    background: var(--accent);
    color: var(--on-accent);
    font-size: 9px;
    font-weight: 600;
    line-height: 12px;
    border-radius: 4px;
    text-align: center;
    pointer-events: none;
  }

  .anchor {
    position: relative;
  }

  /* A panel over the sidebar's foot, at its left. */
  .pbx-menu {
    position: absolute;
    bottom: calc(100% + 6px);
    left: 0;
    width: 280px;
    padding: 4px;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: var(--shadow-float);
    z-index: 2;
  }

  .row {
    display: flex;
  }

  .item {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 32px;
    padding: 0 12px;
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    border-radius: 6px;
    text-align: left;
  }

  .item:disabled {
    opacity: 0.4;
  }

  .name {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .pin {
    flex: none;
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border: 0;
    border-radius: 6px;
    background: none;
    color: var(--icon);
  }

  .pin svg {
    width: 14px;
    height: 14px;
  }

  .pin.on {
    color: var(--accent);
  }

  .item:hover:not(:disabled),
  .pin:hover {
    background: var(--accent-tint);
  }

  .item:focus-visible,
  .pin:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .pbx-manage {
    width: 100%;
    border-top: 1px solid var(--line);
  }

  .pbx-cover {
    position: fixed;
    inset: 0;
  }

  .pbx-prompt {
    width: 360px;
    padding: 20px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    background: var(--surface);
    color: var(--text);
    font-size: 13px;
    line-height: 1.4;
    border-radius: 8px;
    box-shadow: var(--g-float-shadow);
  }

  .pbx-prompt::backdrop {
    background: var(--g-scrim);
  }

  h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  p {
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 12px;
  }

  ul {
    max-height: 160px;
    margin: 12px 0 0;
    padding: 8px 12px 8px 28px;
    overflow: auto;
    border: 1px solid var(--line);
    border-radius: 6px;
  }

  .answers {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;
  }

</style>
