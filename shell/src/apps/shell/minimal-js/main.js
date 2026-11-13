// minimal-js: the minimal shell in plain JavaScript, on the SDK with no framework, drawn
// with the DOM itself and the few utilities in dom.js. A tab bar on top or a list down the
// side, a toolbar, any notice, and the page, to the window's edges: it asks for no band.
// Each change Go sends redraws only what it touches, as onChange names it.

import '@/design.css';
import './minimal.css';
import { extensionsBuilt } from '@extensions';
import { activeTab, connect, controlSpace, focusPage, onChange, send, state } from '@/sdk/shell.js';
import { drag, page, shell } from '@/sdk/regions.js';
import { address } from './address.js';
import { h } from './dom.js';
import { drawTabs, reveal, sideList, tabBar } from './tabs.js';
import { drawNav, drawUpdate, settings, toolbar } from './toolbar.js';

const app = document.getElementById('app');
const main = h('main');
const body = h('div', { class: 'body' }, main);
app.append(toolbar, body);
shell.mark(toolbar);
page.mark(main, 0);

// The extensions' part of the toolbar, only in a build that has them.
// Imported only in a build with extensions, so a build without has none of it, its styles
// too; the import is inlined, so the shell is still one script.
const extensions = extensionsBuilt ? (await import('./extensions.js')).extensionsPart() : null;
if (extensions) settings.before(extensions.el);

// The tab bar or the side list, whichever the tabLayout pref says, and what it marked.
// Down the side, the toolbar is the top row, and its empty space drags the window.
let strip = null;

function drawLayout() {
  if (strip) {
    for (const [marker, el] of strip.marks) marker.unmark(el);
    strip.el.remove();
  }
  const vertical = state.tabLayout === 'vertical';
  strip = vertical ? sideList() : tabBar();
  if (vertical) {
    body.prepend(strip.el);
    strip.marks.push([drag, toolbar]);
  } else {
    app.prepend(strip.el);
  }
  for (const [marker, el] of strip.marks) marker.mark(el);
  app.classList.toggle('vertical', vertical);
}

// The room the native window buttons take at each end of the top row.
function drawControls() {
  const space = controlSpace(state);
  app.style.setProperty('--start', space.left);
  app.style.setProperty('--end', space.right);
}

// The last run's crash, in a bar under the toolbar until the user dismisses it.
let crash = null;

function drawCrash() {
  if (crash) {
    shell.unmark(crash);
    crash.remove();
    crash = null;
  }
  if (!state.crashLog) return;
  const text = `Project B crashed last time. The log is at ${state.crashLog}, macOS's report at ${state.crashReport}`;
  crash = h(
    'div',
    { class: 'crash', role: 'status' },
    h('span', { title: text }, text),
    h('button', { class: 'plain', type: 'button', onclick: () => send({ t: 'dismissCrash' }) }, 'Dismiss'),
  );
  toolbar.after(crash);
  shell.mark(crash);
}

onChange((fields) => {
  const has = (name) => name in fields;
  if (has('controls')) drawControls();
  if (has('tabLayout')) drawLayout();
  if (has('tabs') || has('active') || has('workspaces') || has('tabLayout')) drawTabs(has('workspaces'));
  if (has('active') || has('tabLayout')) reveal();
  if (has('tabs') || has('active')) drawNav();
  if (has('update')) drawUpdate();
  if (has('crashLog')) drawCrash();
  if (extensions && has('extensions')) extensions.draw();
  if (has('active')) page.update(main, state.active);
});

// A tab that becomes active with nothing in it wants the address; one with a page wants
// the page. The shell has to hold the keyboard before its field can.
let active = 0;

function activated() {
  if (state.active === active) return;
  active = state.active;
  address.reset();
  if (activeTab(state).url === '') {
    send({ t: 'focusShell' });
    address.focus();
  } else {
    address.blur();
    focusPage();
  }
}

connect({
  snapshot: activated,
  tabs: activated,
  focusAddressBar: () => address.focus(),
  suggestions: (m) => address.suggestions(m),
});
