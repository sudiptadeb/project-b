<script>
  import { focusPage, send, shell } from '@/sdk/svelte.svelte.js';

  // The address field: url, the active tab's address, until something is typed in it.
  // Typing asks Go for suggestions, listed in a panel under it: Up and Down choose one,
  // Enter or a click goes. Escape puts the address back and gives the page the keyboard.
  let { url } = $props();
  let input;
  // Whether the field holds what was typed since it took the keyboard.
  let edited = $state(false);
  let text = $state('');
  let items = $state([]);
  let chosen = $state(-1);
  // What was typed, which Up and Down come back to past either end of the list.
  let typed = '';
  // The id of the last suggest sent: only its answer is listed.
  let asked = 0;

  const open = $derived(items.length > 0);

  $effect(() => {
    if (!edited) text = url;
  });

  function close() {
    items = [];
    chosen = -1;
    asked++;
  }

  function oninput() {
    edited = true;
    typed = text;
    chosen = -1;
    if (text.trim() === '') close();
    else send({ t: 'suggest', id: ++asked, text });
  }

  function move(by) {
    const next = chosen + by;
    chosen = next >= 0 && next < items.length ? next : -1;
    text = chosen < 0 ? typed : items[chosen].url;
  }

  function onkeydown(e) {
    if (open && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      move(e.key === 'ArrowDown' ? 1 : -1);
    } else if (e.key === 'Enter') {
      go(text);
    } else if (e.key === 'Escape') {
      leave();
    }
  }

  function go(value) {
    if (value.trim() === '') return;
    send({ t: 'navigate', text: value.trim() });
    leave();
  }

  // The field lets the keyboard go to the page, and shows the address again.
  function leave() {
    input.blur();
    focusPage();
  }

  // What the shell asks of the field: to take the keyboard, with what it holds selected,
  // and to let it go.
  export function focus() {
    input.focus();
    input.select();
  }

  export function blur() {
    input.blur();
  }

  export function suggestions(m) {
    if (m.id !== asked) return;
    items = m.items;
    chosen = -1;
  }
</script>

<div class="address">
  <input
    bind:this={input}
    bind:value={text}
    role="combobox"
    aria-label="Address"
    aria-expanded={open}
    aria-controls="suggestions"
    aria-autocomplete="list"
    aria-activedescendant={chosen >= 0 ? `suggestion-${chosen}` : undefined}
    autocomplete="off"
    autocapitalize="off"
    spellcheck="false"
    {oninput}
    {onkeydown}
    onblur={() => {
      edited = false;
      close();
    }}
  />
  {#if open}
    <!-- The keyboard stays in the field, which chooses with Up and Down: a press on the
         list doesn't take it. -->
    <ul id="suggestions" role="listbox" aria-label="Suggestions" use:shell onmousedown={(e) => e.preventDefault()}>
      {#each items as item, i (item.url)}
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_interactive_supports_focus -->
        <li id="suggestion-{i}" role="option" aria-selected={i === chosen} class:chosen={i === chosen} onmouseenter={() => (chosen = i)} onclick={() => go(item.url)}>
          <span class="title">{item.title || item.url}</span>
          {#if item.title}<span class="url">{item.url}</span>{/if}
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
    margin: 0 6px;
  }

  input {
    display: block;
    width: 100%;
    height: 28px;
    padding: 0 10px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 0;
    outline: none;
    background: var(--surface);
    color: var(--text);
    font: 12px var(--mono);
  }

  input:focus {
    border-color: var(--accent);
  }

  /* A plain panel under the field, as wide as it, over what is below. */
  ul {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    margin: 0;
    padding: 0;
    list-style: none;
    background: var(--surface);
    border: 1px solid var(--line);
    border-top: 0;
  }

  li {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 32px;
    padding: 0 10px;
    white-space: nowrap;
  }

  li.chosen {
    background: var(--accent-tint);
  }

  .title {
    flex: none;
    max-width: 60%;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .url {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--muted);
    font: 12px var(--mono);
  }
</style>
