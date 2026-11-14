<script>
  import { activeTab, keyLabel, send, shell, state as go, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import * as g from './glide.svelte.js';

  // The Command Bar: one field for where to go and what to do. It finds open tabs in every
  // Space, actions by name (a leading / lists only them), the history, and the address or
  // search typed. Opened on the address (Cmd+L, the address bar, a new empty tab), Enter
  // takes the active tab there; opened empty (Open Command Bar), it opens a new tab in the
  // Space. Cmd+Enter opens it in a split beside the active tab instead. Up and Down choose,
  // Escape or a press outside closes.
  let { close } = $props();
  const bar = g.ui.bar;
  let input;
  let text = $state(bar.text);
  let chosen = $state(0);
  let history = $state([]);
  let asked = 0;

  const query = $derived(text.trim());
  const words = $derived(query.replace(/^\//, '').toLowerCase().split(/\s+/).filter(Boolean));
  const matches = (s) => words.every((w) => s.toLowerCase().includes(w));
  // Whether the text reads as an address rather than words to search for.
  const address = $derived(/^[a-z][a-z0-9+.-]*:\/?\/?\S*$/i.test(query) || (/^\S+\.\S+$/.test(query) && !/\s/.test(query)) || /^localhost(:\d+)?(\/|$)/.test(query));
  const spaceName = (t) => g.spaces().find((s) => s.id === (g.spaceOf(t)))?.name;

  const rows = $derived.by(() => {
    const actionsOnly = query.startsWith('/');
    const here = g.spaceId();
    const out = [];
    if (!actionsOnly) {
      const tabs = query === '' ? g.tabsIn(here).filter((t) => t.id !== go.active).slice(0, 6) : go.tabs.filter((t) => matches(tabLabel(t) + ' ' + t.url)).sort((a, b) => ((b.workspace || '') === here) - ((a.workspace || '') === here)).slice(0, 4);
      for (const t of tabs) out.push({ kind: 'tab', section: 'Tabs', tab: t, label: tabLabel(t), detail: (g.spaceOf(t)) !== here ? spaceName(t) : g.hostOf(t.url) });
    }
    if (words.length) {
      for (const a of (go.actions || []).filter((a) => matches(a.title)).slice(0, actionsOnly ? 12 : 3)) {
        out.push({ kind: 'action', section: 'Actions', action: a, label: a.title, detail: keyLabel(go, a.id) });
      }
    }
    if (!actionsOnly && query) {
      const open = new Set(go.tabs.map((t) => t.url));
      for (const h of history.filter((h) => !open.has(h.url)).slice(0, 4)) out.push({ kind: 'visit', section: 'History', url: h.url, label: h.title || h.url, detail: h.title ? h.url : '' });
      const goRow = { kind: 'go', section: address ? 'Open' : 'Search', url: query, label: address ? query : `Search for "${query}"` };
      // An address goes first; words look among the tabs and actions first.
      if (address || out.length === 0) out.unshift(goRow);
      else out.push(goRow);
    }
    return out;
  });

  $effect(() => {
    if (chosen >= rows.length) chosen = Math.max(0, rows.length - 1);
  });

  function oninput() {
    bar.typed = true;
    chosen = 0;
    history = [];
    if (query && !query.startsWith('/')) send({ t: 'suggest', id: ++asked, text: query });
  }

  function run(row, aside) {
    if (!row) return;
    if (row.kind === 'tab') send({ t: 'activateTab', id: row.tab.id });
    else if (row.kind === 'action') send({ t: 'run', text: row.action.id });
    else if (aside && activeTab(go).id) g.openInSplit(row.url);
    else if (bar.mode === 'address' && activeTab(go).id) send({ t: 'navigate', text: row.url });
    else send({ t: 'newTab', workspace: g.spaceId(), text: row.url });
    close();
  }

  function onkeydown(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (rows.length) chosen = (chosen + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(rows[chosen], e.metaKey || e.ctrlKey);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  }

  // What the window asks of the bar: to take the keyboard, with what it holds selected,
  // and the history Go found.
  export function focus() {
    input?.focus();
    input?.select();
  }

  export function suggestions(m) {
    if (m.id === asked) history = m.items || [];
  }

  const icons = { tab: 'tab', action: 'bolt', visit: 'clock', go: 'globe' };
</script>

<!-- Over the pages only: the sidebar stays in reach, and a press there closes the bar. -->
<div class="scrim" use:shell role="presentation" style:left={g.win.hidden ? 0 : g.win.width + 4 + 'px'} style:top={g.win.hidden ? '36px' : 0} onpointerdown={(e) => e.target === e.currentTarget && close()}>
  <div class="bar" role="dialog" aria-label="Command Bar">
    <div class="input">
      <Icon name="search" size={18} />
      <input
        bind:this={input}
        bind:value={text}
        role="combobox"
        aria-label="Address"
        aria-expanded={rows.length > 0}
        aria-controls="rows"
        aria-autocomplete="list"
        aria-activedescendant={rows.length ? `row-${chosen}` : undefined}
        placeholder={bar.mode === 'new' ? 'Search, enter an address, or find a tab' : 'Search or enter an address'}
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        {oninput}
        {onkeydown}
      />
    </div>
    {#if rows.length}
      <ul id="rows" role="listbox" aria-label="Results" onmousedown={(e) => e.preventDefault()}>
        {#each rows as row, i (i)}
          {#if i === 0 || rows[i - 1].section !== row.section}
            <li class="section" role="presentation">{row.section}</li>
          {/if}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <li id="row-{i}" role="option" aria-selected={i === chosen} class:chosen={i === chosen} onmouseenter={() => (chosen = i)} onclick={(e) => run(row, e.metaKey || e.ctrlKey)}>
            {#if row.kind === 'tab'}<Favicon tab={row.tab} />{:else}<span class="icon"><Icon name={icons[row.kind]} /></span>{/if}
            <span class="label">{row.label}</span>
            {#if row.detail}<span class="detail">{row.detail}</span>{/if}
          </li>
        {/each}
      </ul>
    {/if}
    <div class="hints"><span>↩ Go</span><span>⌘↩ Split View</span><span>/ Actions</span><span>⎋ Close</span></div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: var(--g-scrim);
  }

  .bar {
    position: absolute;
    top: 112px;
    left: 50%;
    width: min(640px, calc(100% - 32px));
    transform: translateX(-50%);
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--g-float-shadow);
    overflow: hidden;
  }

  .input {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 52px;
    padding: 0 16px;
    color: var(--muted);
  }

  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: 18px;
  }

  ul {
    margin: 0;
    padding: 4px 8px 8px;
    list-style: none;
    border-top: 1px solid var(--line);
  }

  .section {
    padding: 8px 8px 4px;
    color: var(--muted);
    font-size: 12px;
    font-weight: 500;
  }

  [role='option'] {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 36px;
    padding: 0 8px;
    border-radius: 6px;
    white-space: nowrap;
  }

  .chosen {
    background: var(--accent-tint);
  }

  .icon {
    display: grid;
    place-items: center;
    width: 16px;
    color: var(--icon);
  }

  .label {
    flex: none;
    max-width: 60%;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .detail {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: 12px;
    text-align: right;
  }

  .hints {
    display: flex;
    gap: 16px;
    height: 32px;
    align-items: center;
    padding: 0 16px;
    border-top: 1px solid var(--line);
    color: var(--muted);
    font-size: 12px;
  }
</style>
