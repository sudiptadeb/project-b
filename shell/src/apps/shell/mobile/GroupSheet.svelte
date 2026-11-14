<script>
  import { tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';
  import { colors } from './groups.js';

  // A group's sheet: its name, saved on Enter or on leaving the field; its colour; its tabs,
  // a tap on one going there, its × closing it; then New tab in it, Ungroup, and Close
  // group, which asks first.
  let { group, tabs, ask = false, onclose, onchange, onpick, onclosetab, onnewtab, onungroup, onclosegroup } = $props();
  // What the sheet opens with: it is made again for another group.
  // svelte-ignore state_referenced_locally
  let name = $state(group.name);
  // svelte-ignore state_referenced_locally
  let asking = $state(ask);

  function save() {
    if (name.trim() !== group.name) onchange({ name: name.trim() });
  }

  function modal(dialog) {
    dialog.showModal();
  }
</script>

<Sheet label="Group" {onclose}>
  <div class="head">
    <span class="dot" style:background={group.color}></span>
    <input
      class="text-field"
      aria-label="Group name"
      placeholder="Name"
      bind:value={name}
      onblur={save}
      onkeydown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur();
        else if (e.key === 'Escape') {
          e.stopPropagation();
          name = group.name;
          e.currentTarget.blur();
        }
      }}
    />
  </div>
  <div class="swatches" role="radiogroup" aria-label="Colour">
    {#each colors as c (c)}
      <button role="radio" aria-label={c} aria-checked={group.color === c} style:--swatch={c} onclick={() => onchange({ color: c })}></button>
    {/each}
  </div>
  <div class="tabs">
    {#each tabs as tab (tab.id)}
      <div class="row">
        <button class="go" onclick={() => onpick(tab.id)}>
          <Favicon {tab} />
          <span class="title">{tabLabel(tab)}</span>
        </button>
        <button class="x" aria-label="Close {tabLabel(tab)}" onclick={() => onclosetab(tab.id)}><Icon name="close" size={16} /></button>
      </div>
    {/each}
  </div>
  <div class="answers">
    <button class="text-button" onclick={onnewtab}>New tab</button>
    <button class="text-button" onclick={onungroup}>Ungroup</button>
    <button class="text-button danger-text" onclick={() => (asking = true)}>Close group…</button>
  </div>
</Sheet>

{#if asking}
  <dialog use:modal aria-label="Close group" oncancel={(e) => { e.preventDefault(); asking = false; }}>
    <h2>Close {tabs.length} tabs?</h2>
    <div class="ask">
      <button class="text-button" onclick={() => (asking = false)}>Cancel</button>
      <button class="text-button danger" onclick={onclosegroup}>Close</button>
    </div>
  </dialog>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 4px 16px 8px 20px;
  }

  .head input {
    flex: 1;
    min-width: 0;
    height: 40px;
    font-size: 14px;
  }

  .dot {
    flex: none;
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .swatches {
    display: flex;
    gap: 4px;
    padding: 0 12px 8px;
    border-bottom: 1px solid var(--line);
  }

  .swatches button {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: none;
  }

  .swatches button::after {
    content: '';
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--swatch);
  }

  .swatches button[aria-checked='true']::after {
    box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--swatch);
  }

  .tabs {
    display: flex;
    flex-direction: column;
    border-bottom: 1px solid var(--line);
  }

  .row {
    display: flex;
    align-items: center;
    height: 48px;
  }

  .go {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 14px;
    height: 100%;
    padding: 0 0 0 20px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: 14px;
    text-align: left;
  }

  .title {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .x {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  .answers {
    display: flex;
    gap: 8px;
    padding: 12px 16px 4px;
  }

  .answers .text-button,
  .ask .text-button {
    height: 40px;
    padding: 0 16px;
    border-radius: 8px;
  }

  .danger-text {
    color: var(--danger);
  }

  dialog {
    width: min(320px, calc(100% - 32px));
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
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .ask {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
</style>
