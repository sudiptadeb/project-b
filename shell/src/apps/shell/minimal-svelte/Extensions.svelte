<script>
  import { answerExtensionPrompt, manageExtensions, pinExtension, pressExtension } from '@/sdk/extensions.js';
  import { shell } from '@/sdk/svelte.svelte.js';

  // The extensions' buttons, plainly, from extensions, the SDK's state.extensions: a button
  // for each pinned one, then Extensions, whose menu lists them all to open, pin or unpin,
  // and Manage extensions; and the install prompt over the window. The classes and labels
  // are what tools/drive_extensions.py finds them by (docs/shells.md).
  let { extensions } = $props();
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

{#if extensions}
  {#each extensions.buttons.filter((b) => b.pinned) as b (b.id)}
    <button
      class="pbx-action"
      class:pbx-open={extensions.popup === b.id}
      data-extension={b.id}
      disabled={!b.enabled}
      aria-label={b.title || b.name}
      title={b.title || b.name}
      onclick={(e) => pressExtension(b.id, e.currentTarget)}
    >
      {#if b.icon}<img src={b.icon} alt="" width="16" height="16" />{:else}{b.name.slice(0, 1)}{/if}
      {#if b.badge}<span class="pbx-badge">{b.badge.slice(0, 4)}</span>{/if}
    </button>
  {/each}
  <span class="anchor">
    <button bind:this={opener} aria-label="Extensions" aria-expanded={menu} onclick={() => (menu = !menu)}>Extensions</button>
    {#if menu}
      <div class="pbx-menu" use:shell role="group" aria-label="Extensions menu">
        {#each extensions.buttons as b (b.id)}
          <div>
            <button disabled={!b.enabled} onclick={() => open(b.id)}>{b.name}</button>
            <button aria-label="{b.pinned ? 'Unpin' : 'Pin'} {b.name}" onclick={() => pinExtension(b.id, !b.pinned)}>{b.pinned ? 'Unpin' : 'Pin'}</button>
          </div>
        {/each}
        <button class="pbx-manage" onclick={manage}>Manage extensions</button>
      </div>
    {/if}
  </span>
  {#if extensions.prompt}
    {@const p = extensions.prompt}
    <!-- The cover makes the whole window the shell's while it asks, so no press reaches the page. -->
    <div class="cover" use:shell>
      <dialog class="pbx-prompt" use:modal oncancel={cancel} aria-label="{p.remove ? 'Remove' : 'Add'} {p.name}">
        <p>{p.remove ? 'Remove' : 'Add'} "{p.name}"?</p>
        {#if !p.remove}
          <p>Version {p.version}</p>
          {#if p.permissions?.length}
            <ul>{#each p.permissions as permission}<li>{permission}</li>{/each}</ul>
          {/if}
        {/if}
        <button onclick={() => answerExtensionPrompt(false)}>Cancel</button>
        <button class="primary" onclick={() => answerExtensionPrompt(true)}>{p.remove ? 'Remove' : 'Add extension'}</button>
      </dialog>
    </div>
  {/if}
{/if}

<style>
  .anchor {
    position: relative;
  }

  /* The menu, under its button, over the page. */
  .anchor > div {
    position: absolute;
    top: 100%;
    right: 0;
    background: var(--surface);
    border: 1px solid var(--line);
  }

  .cover {
    position: fixed;
    inset: 0;
  }
</style>
