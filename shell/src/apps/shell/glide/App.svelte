<script>
  import { activeTab, connect, focusPage, send, setFrame, state as go } from '@/sdk/svelte.svelte.js';
  import CommandBar from './CommandBar.svelte';
  import Menu from './Menu.svelte';
  import Panes from './Panes.svelte';
  import Sidebar from './Sidebar.svelte';
  import EditDialog from './EditDialog.svelte';
  import TopBar from './TopBar.svelte';
  import * as g from './glide.svelte.js';
  import { newSpace } from './menus.js';
  import { glideTones, styleOf } from './tokens.js';

  // Glide: the sidebar on the left, the page as a card inset in the frame, which is the
  // Space's colour. The window draws no band of its own: the frame is the shell's.
  setFrame(0);

  // Light or dark: the theme pref, or the system's.
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  let systemDark = $state(media.matches);
  media.addEventListener('change', () => (systemDark = media.matches));
  const dark = $derived(go.theme === 'dark' || (go.theme !== 'light' && systemDark));
  const tokens = $derived(styleOf(glideTones(g.space().color, dark)));

  let bar = $state();
  // The tab last made active, so each change of it is seen once.
  let active = 0;

  // A tab that becomes active with nothing in it opens the Command Bar to say where to go;
  // one with a page gets the keyboard.
  function activated() {
    if (go.active === active) return;
    active = go.active;
    if (!go.active) return;
    if (activeTab(go).url === '') openBar('address');
    else if (g.ui.bar?.mode === 'address' && !g.ui.bar.typed) closeBar();
    else if (!g.ui.bar) focusPage();
  }

  function openBar(mode) {
    const url = mode === 'address' ? activeTab(go).url : '';
    g.ui.bar = { mode, text: url, typed: false, at: Date.now() };
    send({ t: 'focusShell' });
    // The bar takes the keyboard once it has drawn.
    queueMicrotask(() => bar?.focus());
  }

  export function closeBar() {
    g.ui.bar = null;
    if (go.active) focusPage();
  }

  // Glide's own actions, as the registry lists them: Go sends each while Glide is on.
  const actions = {
    'glide.commandBar': () => (g.ui.bar?.mode === 'new' ? closeBar() : openBar('new')),
    'glide.sidebar': () => {
      g.win.hidden = !g.win.hidden;
      g.save();
    },
    'glide.pin': () => {
      const t = activeTab(go);
      if (t.id) g.isPinned(t) ? g.unpinTab(t) : g.pin(t);
    },
    'glide.favorite': () => g.addFavorite(activeTab(go)),
    'glide.copyAddress': copyAddress,
    'glide.splitAdd': g.splitWithRecent,
    'glide.splitRemove': () => g.separate(),
    'glide.spacePrevious': () => g.stepSpace(-1),
    'glide.spaceNext': () => g.stepSpace(1),
    'glide.newSpace': newSpace,
  };
  for (let i = 1; i <= 9; i++) {
    actions['glide.space' + i] = () => {
      const s = g.spaces()[i - 1];
      if (s) g.showSpace(s.id);
    };
  }

  export function copyAddress() {
    const url = activeTab(go).url;
    if (!url) return;
    navigator.clipboard.writeText(url).then(
      () => g.toast('Copied'),
      () => g.toast('Could not copy'),
    );
  }

  connect({
    snapshot: () => {
      g.load(go.data);
      activated();
    },
    tabs: () => {
      g.follow();
      activated();
    },
    // A Space made here shows once Go lists it.
    workspaces: () => {
      if (g.ui.created !== null && go.workspaces.length > g.ui.created) {
        g.ui.created = null;
        g.showSpace(go.workspaces[go.workspaces.length - 1].id);
      }
    },
    // The active tab went somewhere by other means, a link or a key: a bar opened on its
    // address, with nothing typed, has nothing left to say.
    tab: (m) => {
      const bar = g.ui.bar;
      if (m.tab.id === go.active && bar?.mode === 'address' && !bar.typed && m.tab.url && m.tab.url !== bar.text) g.ui.bar = null;
    },
    focusAddressBar: () => openBar('address'),
    suggestions: (m) => bar?.suggestions(m),
    action: (m) => actions[m.id]?.(),
  });
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && g.ui.menu) g.ui.menu = null;
  }}
/>

<div class="glide" style={tokens} class:dark>
  {#if g.win.hidden}
    <TopBar onaddress={() => openBar('address')} oncopy={copyAddress} />
  {/if}
  <div class="body">
    {#if !g.win.hidden}
      <Sidebar onaddress={() => openBar('address')} oncopy={copyAddress} onnew={() => send({ t: 'newTab', workspace: g.spaceId() })} onclosebar={() => (g.ui.bar = null)} />
    {/if}
    <Panes hidden={g.win.hidden} />
  </div>
  {#if g.ui.bar}
    <CommandBar bind:this={bar} close={closeBar} />
  {/if}
  {#if g.ui.menu}
    <Menu menu={g.ui.menu} />
  {/if}
  {#if g.ui.dialog}
    <EditDialog dialog={g.ui.dialog} />
  {/if}
  {#if g.ui.toast}
    <div class="toast" role="status">{g.ui.toast}</div>
  {/if}
</div>
