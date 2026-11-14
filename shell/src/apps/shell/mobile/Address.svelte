<script>
  import { focusPage, send, shell } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // The address pill. At rest it shows the host and the page's lock; a tap turns it into
  // the field, with the whole address selected. Typing
  // asks Go for suggestions, listed away from the field towards the page, the closest next
  // to it: a tap on one goes there, its ↖ puts its address in the field. Enter goes; Escape
  // or a press in the page puts the address back.
  let { tab, top, onedit } = $props();
  let input;
  let editing = $state(false);
  let text = $state('');
  let items = $state([]);
  // The id of the last suggest sent: only its answer is listed.
  let asked = 0;

  const host = $derived.by(() => {
    if (!tab.url) return '';
    try {
      const u = new URL(tab.url);
      return u.protocol === 'http:' || u.protocol === 'https:' ? u.host.replace(/^www\./, '') : tab.url;
    } catch {
      return tab.url;
    }
  });
  // What went wrong with the page, said at the pill's end.
  const status = $derived(tab.blocked ? 'Blocked' : tab.crashed ? 'Crashed' : tab.error ? 'Failed' : '');
  const secure = $derived(tab.url.startsWith('https:') ? 'lock' : tab.url.startsWith('http:') ? 'warn' : '');
  // The list from the field outwards: under a field on top, over one at the bottom with
  // the first suggestion nearest it.
  const shown = $derived(top ? items : [...items].reverse());

  $effect(() => onedit(editing));

  function begin() {
    if (editing) return;
    editing = true;
    text = tab.url;
    // After the value is in, so the whole of it is selected.
    queueMicrotask(() => input.select());
  }

  function end() {
    editing = false;
    items = [];
    asked++;
  }

  function oninput() {
    if (text.trim() === '') {
      items = [];
      asked++;
    } else {
      send({ t: 'suggest', id: ++asked, text });
    }
  }

  function go(value) {
    if (value.trim() === '') return;
    send({ t: 'navigate', text: value.trim() });
    leave();
  }

  function leave() {
    input.blur();
    focusPage();
  }

  function fill(url) {
    text = url;
    input.focus();
    oninput();
  }

  // What the shell asks of the field.
  export function focus() {
    input.focus();
    begin();
  }

  export function blur() {
    input.blur();
  }

  export function suggestions(m) {
    if (m.id === asked && editing) items = m.items;
  }
</script>

<div class="address" class:editing>
  {#if !editing && secure}
    <span class="mark" aria-hidden="true"><Icon name={secure} size={16} /></span>
  {/if}
  <input
    bind:this={input}
    value={editing ? text : host}
    oninput={(e) => {
      text = e.currentTarget.value;
      oninput();
    }}
    aria-label="Address"
    placeholder="Search or type an address"
    autocomplete="off"
    autocapitalize="off"
    spellcheck="false"
    enterkeyhint="go"
    inputmode="url"
    onfocus={begin}
    onblur={() => {
      end();
      // A press in the page takes the window's keyboard, not the field's: let it go, so
      // the field doesn't take it back when the shell next has the keyboard.
      setTimeout(() => !document.hasFocus() && document.activeElement === input && input.blur());
    }}
    onkeydown={(e) => {
      if (e.key === 'Enter') go(text);
      else if (e.key === 'Escape') leave();
    }}
  />
  {#if !editing && status}
    <span class="status">{status}</span>
  {/if}
  {#if editing && text}
    <!-- A press here mustn't take the keyboard from the field. -->
    <button class="clear" aria-label="Clear" onmousedown={(e) => e.preventDefault()} onclick={() => fill('')}><Icon name="close" size={18} /></button>
  {/if}
  {#if !editing && tab.loading}
    <span class="progress" style:width="{Math.max(tab.progress, 0.05) * 100}%"></span>
  {/if}
  {#if editing && items.length}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <ul class="suggestions" class:below={top} use:shell aria-label="Suggestions" onmousedown={(e) => e.preventDefault()}>
      {#each shown as item (item.url)}
        <li>
          <button class="go" onclick={() => go(item.url)}>
            <Icon name={item.title ? 'globe' : 'search'} />
            <span class="lines">
              <span class="title">{item.title || item.url}</span>
              {#if item.title}<span class="url">{item.url.replace(/^https?:\/\/(www\.)?/, '')}</span>{/if}
            </span>
          </button>
          <button class="fill" aria-label="Edit {item.url}" onclick={() => fill(item.url)}><Icon name="fill" /></button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .address {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    height: 40px;
    padding: 0 6px 0 14px;
    gap: 6px;
    box-sizing: border-box;
    border-radius: 20px;
    background: var(--surface);
    color: var(--text);
  }

  .address.editing {
    box-shadow: inset 0 0 0 1.5px var(--accent);
  }

  .mark {
    display: grid;
    color: var(--icon);
  }

  /* The field and its Clear reach 48px, the toolbar's height, past the 40px pill. */
  input {
    flex: 1;
    min-width: 0;
    height: 48px;
    margin: -4px 0;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: inherit;
    font-family: inherit;
    font-size: 14px;
    font-weight: 500;
    text-overflow: ellipsis;
  }

  .editing input {
    font-weight: 400;
  }

  input::placeholder {
    color: var(--muted);
    font-weight: 400;
  }

  .status {
    flex: none;
    padding: 2px 8px;
    border: 1px solid var(--danger);
    border-radius: 10px;
    color: var(--danger);
    font-size: 12px;
  }

  .clear {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin: -4px -6px -4px 0;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: none;
    color: var(--icon);
  }

  .progress {
    position: absolute;
    left: 14px;
    bottom: 0;
    max-width: calc(100% - 28px);
    height: 2px;
    border-radius: 1px;
    background: var(--accent);
    transition: width 150ms;
  }

  .suggestions {
    position: absolute;
    left: -8px;
    right: -92px;
    bottom: calc(100% + 4px);
    z-index: 5;
    margin: 0;
    padding: 0;
    list-style: none;
    background: var(--surface);
    border-top: 1px solid var(--line);
  }

  .suggestions.below {
    top: calc(100% + 4px);
    bottom: auto;
    border-top: 0;
    border-bottom: 1px solid var(--line);
  }

  li {
    display: flex;
    align-items: center;
    height: 48px;
    border-bottom: 1px solid var(--line);
  }

  .below li {
    border-bottom: 0;
    border-top: 1px solid var(--line);
  }

  li:last-child {
    border-bottom: 0;
  }

  .go {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 14px;
    height: 100%;
    padding: 0 0 0 16px;
    border: 0;
    background: none;
    color: var(--icon);
    text-align: left;
  }

  .lines {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title,
  .url {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .title {
    color: var(--text);
    font-size: 14px;
  }

  .url {
    color: var(--muted);
    font-size: 12px;
  }

  .fill {
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

  button:focus-visible {
    box-shadow: var(--focus-ring-inset);
  }
</style>
