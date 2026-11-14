<script>
  import { activeTab, connect, drag, focusPage, page, send, setFrame, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import CardMenu from './CardMenu.svelte';
  import CrashBar from './CrashBar.svelte';
  import Extensions from './Extensions.svelte';
  import GroupSheet from './GroupSheet.svelte';
  import LinkSheet from './LinkSheet.svelte';
  import MenuSheet from './MenuSheet.svelte';
  import PageNotice from './PageNotice.svelte';
  import Strip from './Strip.svelte';
  import Switcher from './Switcher.svelte';
  import Toolbar from './Toolbar.svelte';
  import { groupOf } from './groups.js';

  // The phone's shell: the page, and under it one bar, the address, the tab count and ⋯,
  // with the group strip over the bar while the active tab is in a group. Everything else
  // is a sheet one of them opens: the switcher over the page, the rest from the bottom. The
  // page reaches the window's edges: no band.
  setFrame(0);

  const tab = $derived(activeTab(go));
  // The address bar on top, as a setting has it, rather than at the foot. The group strip
  // stays at the foot either way, as Chrome's does.
  const top = $derived(go.addressBarPosition === 'top');
  // What is open over the page: '', 'switcher', 'menu', 'extensions', 'group' (the active
  // tab's, or the one in sheetGroup), 'card' (cardTab's menu) or 'link' (a long press in
  // the page).
  let sheet = $state('');
  let sheetGroup = $state('');
  let cardTab = $state(0);
  let askClose = $state(false);
  // What a long press in the page was on, for the 'link' sheet.
  let pressed = $state(null);
  // The workspace new tabs open in, and the switcher shows: the active tab's, until the
  // user picks another.
  let workspace = $state('');
  let address = $state();
  // The tab last made active, so each change of it is seen once.
  let active = 0;
  // The bars while the page scrolls down: away, sliding off the screen, then gone, their
  // room given to the page in one step, so the page is resized once and not every frame.
  // Back with the page scrolling up, another tab, the address or a sheet.
  // Whether the address is being edited, which the phone's back key ends, as it closes a
  // sheet: while either is so, the engine gives the key to the shell (backCloses).
  let editing = $state(false);
  $effect(() => send({ t: 'backCloses', on: !!sheet || editing }));

  // The phone's back key: the sheet open closes, or the editing ends.
  function backKey() {
    if (sheet) sheet = '';
    else if (editing) {
      address.blur();
      focusPage();
    }
  }

  let away = $state(false);
  let gone = $state(false);
  let sliding = 0;
  const slide = matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 150;

  function bars(show) {
    if (show !== away) return;
    clearTimeout(sliding);
    if (show) {
      gone = false;
      requestAnimationFrame(() => requestAnimationFrame(() => (away = false)));
    } else {
      away = true;
      sliding = setTimeout(() => (gone = true), slide);
    }
  }

  // A page that starts loading in the active tab brings the bars back, as Chrome's do.
  let loading = '';
  $effect(() => {
    const now = tab.loading ? tab.url : '';
    if (now && now !== loading) bars(true);
    loading = now;
  });

  function scrolled(m) {
    const editing = document.activeElement?.getAttribute('aria-label') === 'Address';
    if (m.tab === go.active && !sheet && !editing) bars(m.direction !== 'down');
  }

  $effect(() => {
    if (sheet) bars(true);
  });

  // Space for the native window buttons, which a phone hasn't: a desktop window, as on
  // the mock, keeps the page from under them.
  const clear = $derived(go.controls?.h > 0 ? go.controls.y + go.controls.h : 0);

  // Go's tab groups (docs/groups.md): the active tab's, and the one whose sheet is open.
  const group = $derived(groupOf(go.groups, go.active));
  const tabsOf = (g) => g.tabs.map((id) => go.tabs.find((t) => t.id === id)).filter(Boolean);
  const shownGroup = $derived(go.groups.find((g) => g.id === sheetGroup) || null);

  // A tab that becomes active with nothing in it wants the address; one with a page wants
  // the page. The shell has to hold the keyboard before its field can.
  function activated() {
    if (go.active === active) return;
    active = go.active;
    bars(true);
    workspace = tab.workspace || '';
    address.blur();
    if (tab.url === '') {
      send({ t: 'focusShell' });
      address.focus();
    } else {
      focusPage();
    }
  }

  // A new tab, or with a tab's id, one in that tab's group, starting one with it when it
  // has none.
  function newTab(groupWith = 0) {
    sheet = '';
    send(groupWith ? { t: 'newTab', groupWith } : { t: 'newTab', workspace });
  }

  function pick(id) {
    sheet = '';
    if (id !== go.active) send({ t: 'activateTab', id });
    else focusPage();
  }

  // The window never has no tabs: closing the last ones leaves a new one. Closing the
  // active tab of a group goes to its neighbour in the group, as the strip shows it.
  function closeTabs(ids) {
    if (ids.length >= go.tabs.length) {
      sheet = '';
      send({ t: 'newTab', workspace });
    } else if (ids.includes(go.active) && group) {
      const left = group.tabs.filter((id) => !ids.includes(id));
      const at = group.tabs.indexOf(go.active);
      const next = left[Math.min(at, left.length - 1)];
      if (next) send({ t: 'activateTab', id: next });
    }
    for (const id of ids) send({ t: 'closeTab', id });
  }

  // The tab before or after the active one: in its group while it is in one.
  function swipe(by) {
    const ids = group ? group.tabs : go.tabs.map((t) => t.id);
    const next = ids[ids.indexOf(go.active) + by];
    if (next) send({ t: 'activateTab', id: next });
  }

  function onsheet(name) {
    if (name === 'switcher') workspace = tab.workspace || '';
    if (name === 'group') sheetGroup = group?.id || '';
    askClose = false;
    sheet = name;
  }

  function openGroup(id, ask = false) {
    sheetGroup = id;
    askClose = ask;
    sheet = 'group';
  }

  connect({
    snapshot: activated,
    tabs: activated,
    focusAddressBar: () => {
      sheet = '';
      bars(true);
      address.focus();
    },
    scroll: scrolled,
    // Go's right-click menu: the sheet opens on one for the shown tab. Go drops it when
    // the tab changes, and says so with null.
    contextMenu: ({ contextMenu: m }) => {
      if (!m || m.tab !== go.active) return;
      pressed = m;
      sheet = 'link';
    },
    backKey,
    suggestions: (m) => address.suggestions(m),
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && sheet && ((sheet = ''), e.preventDefault())} />

<div class="window">
  {#if clear}
    <div class="clear" use:shell use:drag style:height="{clear}px"></div>
  {/if}
  {#if top}
    <div class="bars head" class:away class:gone>
      <Toolbar {tab} top count={go.tabs.length} {sheet} {onsheet} onswipe={swipe} onedit={(on) => (editing = on)} bind:address />
    </div>
  {/if}
  <div class="stage">
    <main use:page={go.active}>
      {#if tab.url && !tab.loading && (tab.crashed || tab.error || tab.blocked)}
        <PageNotice {tab} />
      {/if}
    </main>
    {#if group}
      <div class="bars" class:away class:gone>
        <Strip {group} tabs={tabsOf(group)} active={go.active} onsheet={() => onsheet('group')} onpick={pick} onclosetab={(id) => closeTabs([id])} onnewtab={() => newTab(go.active)} />
      </div>
    {/if}
    <Switcher
      open={sheet === 'switcher'}
      groups={go.groups}
      bind:workspace
      onpick={pick}
      onclosetab={(id) => closeTabs([id])}
      onnewtab={() => newTab()}
      oncard={(id) => {
        cardTab = id;
        sheet = 'card';
      }}
      ongroup={(id) => openGroup(id)}
      ondrop={(id, to) => send(to.group ? { t: 'addToGroup', id, group: to.group } : { t: 'createGroup', ids: [to.tab, id] })}
      onclosegroup={(id) => openGroup(id, true)}
    />
  </div>
  <div class="bars" class:away class:gone>
    <CrashBar />
    {#if !top}
      <Toolbar {tab} count={go.tabs.length} {sheet} {onsheet} onswipe={swipe} onedit={(on) => (editing = on)} bind:address />
    {/if}
  </div>
  {#if sheet === 'menu'}
    <MenuSheet {tab} {group} {workspace} onclose={() => (sheet = '')} onnewtab={(inGroup) => newTab(inGroup ? go.active : 0)} {onsheet} onworkspace={(id) => (workspace = id)} />
  {:else if sheet === 'group' && shownGroup}
    {#key shownGroup.id}
      <GroupSheet
        group={shownGroup}
        tabs={tabsOf(shownGroup)}
        ask={askClose}
        onclose={() => (sheet = '')}
        onchange={(fields) => send({ t: 'editGroup', group: shownGroup.id, ...fields })}
        onpick={pick}
        onclosetab={(id) => closeTabs([id])}
        onnewtab={() => newTab(shownGroup.tabs.at(-1))}
        onungroup={() => {
          send({ t: 'ungroup', group: shownGroup.id });
          sheet = '';
        }}
        onclosegroup={() => {
          sheet = '';
          if (shownGroup.tabs.length >= go.tabs.length) send({ t: 'newTab', workspace });
          send({ t: 'closeGroup', group: shownGroup.id });
        }}
      />
    {/key}
  {:else if sheet === 'link' && pressed}
    <LinkSheet
      menu={pressed}
      onclose={() => (sheet = '')}
      onopen={(m) => {
        sheet = '';
        send({ t: 'newTab', ...m });
      }}
    />
  {:else if sheet === 'card' && go.tabs.some((t) => t.id === cardTab)}
    {@const t = go.tabs.find((x) => x.id === cardTab)}
    {@const own = groupOf(go.groups, cardTab)}
    <CardMenu
      tab={t}
      group={own}
      groups={go.groups.filter((g) => g !== own && (tabsOf(g)[0]?.workspace || '') === (t.workspace || ''))}
      onclose={() => (sheet = '')}
      onadd={(id) => {
        send({ t: 'addToGroup', id: cardTab, group: id });
        sheet = 'switcher';
      }}
      onnewgroup={() => newTab(cardTab)}
      onremove={() => {
        send({ t: 'removeFromGroup', id: cardTab });
        sheet = 'switcher';
      }}
      onclosetab={() => {
        closeTabs([cardTab]);
        if (sheet === 'card') sheet = 'switcher';
      }}
    />
  {/if}
  {#if extensionsBuilt && go.extensions}
    <Extensions extensions={go.extensions} open={sheet === 'extensions'} onclose={() => (sheet = '')} />
  {/if}
</div>

<style>
  .window {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    color: var(--text);
    font-size: 14px;
    -webkit-user-select: none;
    user-select: none;
  }

  .clear {
    flex: none;
    background: var(--bar);
  }

  .stage {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  main {
    position: relative;
    flex: 1;
    min-height: 0;
  }

  .bars {
    flex: none;
    display: flex;
    flex-direction: column;
    transition: transform 150ms ease-out;
  }

  .bars.away {
    transform: translateY(100%);
  }

  .head.away {
    transform: translateY(-100%);
  }

  .bars.gone {
    height: 0;
    overflow: hidden;
  }

  @media (prefers-reduced-motion: reduce) {
    .bars {
      transition: none;
    }
  }

  :global(button) {
    font-family: inherit;
    -webkit-tap-highlight-color: transparent;
  }
</style>
