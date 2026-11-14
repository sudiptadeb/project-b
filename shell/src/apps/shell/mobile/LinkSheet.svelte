<script>
  import { state as go } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // What a long press on a link or an image in the page offers, drawn by the shell since a
  // phone has no menu of its own: for a link, Open in new tab in group first, as Chrome
  // has it, then Open in new tab, Open in workspace, and Copy link; for an image, Open
  // image in new tab and Copy image address. A tab opened here opens behind, so the group
  // strip gathers them.
  let { menu, onclose, onopen } = $props();
  let list = $state('');
  const link = $derived(menu.link);
  const url = $derived(link || menu.src);
  const shown = $derived(url.replace(/^https?:\/\/(www\.)?/, ''));
  const workspaces = $derived(go.workspaces || []);

  async function copy(text) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // A page without the clipboard's permission still has the old way.
      const area = document.createElement('textarea');
      area.value = text;
      document.body.append(area);
      area.select();
      document.execCommand('copy');
      area.remove();
    }
    onclose();
  }
</script>

<Sheet label="Link" {onclose}>
  <div class="head">
    {#if link && menu.linkText}<span class="text">{menu.linkText}</span>{/if}
    <span class="url">{shown}</span>
  </div>
  <div class="rows" role="menu" aria-label={link ? 'Link' : 'Image'}>
    {#if list === 'workspaces'}
      {#each [{ id: '', name: 'Default', color: '' }, ...workspaces] as w (w.id)}
        <button class="row" role="menuitem" onclick={() => onopen({ text: url, workspace: w.id, background: true })}>
          <span class="dot" style:background={w.color || 'var(--muted)'}></span>
          <span class="name">{w.name}</span>
        </button>
      {/each}
    {:else if link}
      <button class="row" role="menuitem" onclick={() => onopen({ text: url, groupWith: menu.tab, background: true })}>
        <Icon name="group" /><span class="name">Open in new tab in group</span>
      </button>
      <button class="row" role="menuitem" onclick={() => onopen({ text: url, background: true })}>
        <Icon name="tab" /><span class="name">Open in new tab</span>
      </button>
      {#if workspaces.length}
        <button class="row" role="menuitem" onclick={() => (list = 'workspaces')}>
          <Icon name="workspace" /><span class="name">Open in workspace</span><Icon name="chevron" />
        </button>
      {/if}
      <button class="row" role="menuitem" onclick={() => copy(url)}>
        <Icon name="copy" /><span class="name">Copy link</span>
      </button>
    {:else}
      <button class="row" role="menuitem" onclick={() => onopen({ text: url, background: true })}>
        <Icon name="image" /><span class="name">Open image in new tab</span>
      </button>
      <button class="row" role="menuitem" onclick={() => copy(url)}>
        <Icon name="copy" /><span class="name">Copy image address</span>
      </button>
    {/if}
  </div>
</Sheet>

<style>
  .head {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0 20px 10px;
    border-bottom: 1px solid var(--line);
    font-size: 12px;
  }

  .text {
    color: var(--text);
    font-weight: 500;
  }

  .text,
  .url {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .url {
    color: var(--muted);
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
    font-size: 14px;
    text-align: left;
  }

  .name {
    flex: 1;
    color: var(--text);
  }

  .row:first-child .name {
    font-weight: 500;
  }

  .dot {
    width: 10px;
    height: 10px;
    margin: 0 5px;
    border-radius: 50%;
  }

  button:active {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
