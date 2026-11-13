// The minimal-vue shell's own tests: the tab bar with one tab and with many, which scrolls
// with the active tab kept in view and New Tab and the drag space in place; the list down
// the side; the close button; the address with its suggestions; the extensions' buttons,
// menu and prompt, from the mock store; the crash and update notices; light and dark. The
// check covers what every shell does.
// Run: node shell/e2e/shells/minimal-vue.cjs   (from the repo root, after `npm run build`
// in shell/; the mock store, .tools/extensions, has the test extension's package)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, out, regions, settle, until } = lib;

const id = 'minimal-vue';
// The test extension, as tools/pack-test-extension.py packs it into the mock store.
const extension = 'oepjjojhhaclfhmopocgmdfbeiffpheh';
const extensionName = 'Project B test extension';
const packaged = path.join(lib.root, `.tools/extensions/crx/${extension}_1.0.crx`);
const work = { id: '0a1b2c3d', name: 'Work', color: '#d97706' };

// The mock's extension runtime: what happens outside the app, and what an extension calls.
const fireExtension = (kind, value) => fetch(`${lib.base}/mock/extensions/fire`, { method: 'POST', body: new URLSearchParams({ kind, extension: '', value }) });
const callExtension = (name, args) => fetch(`${lib.base}/mock/extensions/call?extension=${extension}&name=${name}&tab=0`, { method: 'POST', body: JSON.stringify(args) });
const popup = async () => (await (await fetch(`${lib.base}/mock/extensions`)).json()).popup;
// A message to Go as the shell sends it.
const tell = (msg) => fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify(msg) });

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [work] }));
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ extensions: 'on' }));
  await lib.start([`-use=${id}`]);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const tabs = ui.locator('[role=tab]');
  const field = ui.getByRole('combobox', { name: 'Address' });
  const options = ui.getByRole('option');
  const newTab = ui.getByRole('button', { name: 'New Tab', exact: true });
  const drawn = () => tabs.evaluateAll((els) => els.map((el) => ({ id: Number(el.dataset.tab), selected: el.getAttribute('aria-selected') === 'true' })));
  const agrees = async () => {
    const r = await regions();
    const d = await drawn();
    return JSON.stringify(d.map((t) => t.id)) === JSON.stringify(r.tabs.map((t) => t.id)) && d.every((t) => t.selected === (t.id === r.active));
  };
  const tabCount = (n) => until(async () => (await regions()).tabs.length === n && (await tabs.count()) === n && (await agrees()));
  const active = async () => {
    const r = await regions();
    return r.tabs.find((t) => t.id === r.active);
  };
  const box = (selector) => w.shell().evaluate((s) => document.querySelector(s).getBoundingClientRect().toJSON(), selector);
  // Whether the active tab shows whole in the list that holds it.
  const activeInView = () => w.shell().evaluate(() => {
    const tab = document.querySelector('[role=tab][aria-selected=true]').getBoundingClientRect();
    const list = document.querySelector('[role=tablist]').getBoundingClientRect();
    return tab.left >= list.left - 0.5 && tab.right <= list.right + 0.5 && tab.top >= list.top - 0.5 && tab.bottom <= list.bottom + 0.5;
  });
  const scroller = () => w.shell().evaluate(() => {
    const s = document.querySelector('[role=tablist]');
    return { scrollLeft: s.scrollLeft, scrollTop: s.scrollTop, scrollWidth: s.scrollWidth, clientWidth: s.clientWidth, scrollHeight: s.scrollHeight, clientHeight: s.clientHeight };
  });
  // A person's pointer arrives in many moves; the window gives the shell the pointer on
  // the first move into its region, so the hover begins with the second.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  const shown = (locator) => locator.evaluate((el) => el.checkVisibility());
  const visit = async (text, url) => {
    await field.click();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type(text);
    await page.keyboard.press('Enter');
    return until(async () => (await active()).url === url && !(await active()).loading && (await field.inputValue()) === url);
  };
  // The controls that show and a press on them doesn't reach, as the check's rule 6 has
  // them, for the tabs on top, which the check doesn't see. A tab scrolled part way out
  // shows only its part inside the tabs.
  const notTheShells = async () => {
    const { controls } = await import('../../src/sdk/controls.js');
    const shownControls = await w.shell().evaluate((selector) => [...document.querySelectorAll(selector)]
      .filter((el) => el.checkVisibility({ visibilityProperty: true, opacityProperty: true }))
      .map((el) => {
        const r = el.getBoundingClientRect();
        const list = (el.closest('[role=tablist]') || document.body).getBoundingClientRect();
        const left = Math.max(r.left, list.left);
        const right = Math.min(r.right, list.right);
        return { what: el.outerHTML.slice(0, 60), rect: { x: left, y: r.y, width: right - left, height: r.height } };
      })
      .filter(({ rect }) => rect.width > 0 && rect.x < innerWidth), controls);
    const missed = [];
    for (const c of shownControls) {
      const x = Math.min(Math.max(c.rect.x + c.rect.width / 2, 0), 1099);
      if ((await regions({ x, y: c.rect.y + c.rect.height / 2 })).hit !== 'shell') missed.push(c.what);
    }
    return missed;
  };
  const shots = async (name) => {
    await page.mouse.move(600, 400);
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(out, `${id}-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  check('the window loads the minimal-vue shell with its one tab', await tabCount(1) && (await regions()).shell === id);
  check('the address and the rest are in their own type',
    await w.shell().evaluate(async () => {
      await document.fonts.ready;
      const family = (s) => getComputedStyle(document.querySelector(s)).fontFamily;
      return [...document.fonts].every((f) => f.status === 'loaded') && family('.address').startsWith('"Space Mono"') && family('[role=tab]').startsWith('"Space Grotesk"');
    }));

  // Tabs on top.
  await fire('menu', 'view.tabsTop');
  check('with the tabs on top there is a tab bar, then the toolbar', await until(async () => (await ui.locator('.tabbar').count()) === 1 && (await ui.locator('.side').count()) === 0));
  await settle(page);
  let r = await regions();
  check('it asks for no band, and the page reaches the window\'s sides and foot',
    r.frameWidth === 0 && r.visible[0].rect.x === 0 && r.visible[0].rect.w === r.window.W && r.visible[0].rect.y + r.visible[0].rect.h === r.window.H, JSON.stringify({ frame: r.frameWidth, visible: r.visible }));
  const tabbar = await box('.tabbar');
  const toolbar = await box('.toolbar');
  check('the tab bar is 34px and the toolbar 38px, each with a hairline under it, and the page under both',
    tabbar.height === 35 && toolbar.top === 35 && toolbar.height === 39 && r.visible[0].rect.y === 74, JSON.stringify({ tabbar, toolbar, page: r.visible[0].rect }));
  const lights = await box('.tabbar .lights');
  const c = r.window.Controls;
  check('the bar leaves the window buttons their space, which drags the window',
    lights.left === 0 && lights.right >= c.x + c.w && (await regions({ x: lights.width / 2, y: 10 })).hit === 'drag', JSON.stringify({ lights, controls: c }));

  // The address and the loading tab.
  await field.click();
  await page.keyboard.type('example.com/news/today');
  const loading = w.shell().waitForFunction(() => document.querySelector('[aria-label=Stop]') && document.querySelector('[role=tab] .loading'), null, { polling: 10, timeout: 3000 }).then(() => true, () => false);
  await page.keyboard.press('Enter');
  check('while the page loads Reload is Stop and the tab shows its loading mark', await loading);
  check('once it has loaded, Reload again and the mark gone',
    await until(async () => (await active()).url === 'https://example.com/news/today' && (await ui.getByRole('button', { name: 'Reload' }).isEnabled()) && (await ui.locator('.loading').count()) === 0));
  check('the field shows the active tab\'s address', await until(async () => (await field.inputValue()) === 'https://example.com/news/today'), await field.inputValue());
  check('a tab is 200px wide', (await tabs.first().boundingBox()).width === 200, JSON.stringify(await tabs.first().boundingBox()));

  // A tab in a workspace, then New Tab from the first, outside it, twice.
  await fire('menu', `workspaces:${work.id}`);
  await tabCount(2);
  const inWork = (await regions()).active;
  await visit('mail.test', 'https://mail.test');
  const dot = ui.locator(`[role=tab][data-tab="${inWork}"] .dot`);
  check('a tab in a workspace shows a square in its colour', (await dot.count()) === 1 && (await dot.evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(217, 119, 6)');
  check('a tab outside one shows none', (await tabs.first().locator('.dot').count()) === 0);
  await tabs.first().click();
  await until(async () => (await active()).url === 'https://example.com/news/today');
  await newTab.click();
  check('New Tab opens a tab after the last, active', await tabCount(3) && (await drawn())[2].selected);
  check('and puts the keyboard in the address', await until(() => field.evaluate((el) => el === document.activeElement && document.hasFocus())));
  check('a visit there shows in the field', await visit('github.test/project-b', 'https://github.test/project-b'));
  await newTab.click();
  await tabCount(4);
  await visit('weather.test', 'https://weather.test');
  check('four tabs, each 200px', JSON.stringify(await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width))) === '[200,200,200,200]');

  // The close button: always on the active tab, on hover on the others.
  const closeOf = (i) => tabs.nth(i).locator('.x');
  await page.mouse.move(600, 400);
  await settle(page);
  check('the active tab shows its close button, the others don\'t', (await shown(closeOf(3))) && !(await shown(closeOf(0))) && !(await shown(closeOf(1))));
  await hover(tabs.first());
  check('a tab pointed at shows its close button', await until(() => shown(closeOf(0))));
  await tabs.first().click();
  check('a click on a tab makes it active, and the field shows its address',
    await until(async () => (await active()).url === 'https://example.com/news/today' && (await field.inputValue()) === 'https://example.com/news/today'));
  await newTab.click();
  await tabCount(5);
  const extra = (await regions()).active;
  await closeOf(4).click();
  check('the active tab\'s close button closes it', await tabCount(4) && !(await regions()).tabs.some((t) => t.id === extra));

  // The address: focusAddressBar, suggestions, Escape.
  await tabs.first().click();
  await until(async () => (await active()).url === 'https://example.com/news/today');
  await fire('key', 'focusAddressBar', { tab: (await regions()).active });
  check('focusAddressBar puts the keyboard in the field with the address selected', await until(() => field.evaluate((el) => el === document.activeElement && el.selectionStart === 0 && el.selectionEnd === el.value.length && el.value !== '')));
  await page.keyboard.type('exam');
  check('typing lists suggestions', await until(async () => (await options.count()) >= 1), String(await options.count()));
  const fieldBox = await field.boundingBox();
  const listBox = await ui.getByRole('listbox').boundingBox();
  check('in a panel under the field, as wide as it', Math.abs(listBox.y - (fieldBox.y + fieldBox.height)) <= 1 && Math.abs(listBox.width - fieldBox.width) <= 1, JSON.stringify({ fieldBox, listBox }));
  const last = await options.last().boundingBox();
  check('a press on the panel reaches the shell, over the page', await until(async () => (await regions({ x: last.x + last.width / 2, y: last.y + last.height / 2 })).hit === 'shell'));
  await shots('suggestions');
  await page.keyboard.press('ArrowDown');
  check('Down chooses one, and the field shows it', (await ui.locator('[role=option][aria-selected=true]').count()) === 1 && (await field.inputValue()).startsWith('https://example.com'), await field.inputValue());
  await page.keyboard.press('ArrowUp');
  check('Up from the first comes back to what was typed', (await field.inputValue()) === 'exam', await field.inputValue());
  await page.keyboard.press('Escape');
  check('Escape puts the address back, closes the panel and gives the page the keyboard',
    await until(async () => (await field.inputValue()) === 'https://example.com/news/today' && (await options.count()) === 0 && (await regions()).focused === (await regions()).active),
    JSON.stringify({ value: await field.inputValue(), focused: (await regions()).focused }));
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('weath');
  await until(async () => (await options.count()) >= 1);
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  check('Enter on a chosen one goes there', await until(async () => (await active()).url === 'https://weather.test' && !(await active()).loading));
  await visit('example.com/news/today', 'https://example.com/news/today');

  // The extensions: the prompt, from the store's package, which the settings page installs.
  // The test drives a settings page of its own, which talks to Go as the one over the
  // window does, opened while that one is, and stays usable once Go has closed it, so the
  // shell is not covered (extensions.cjs).
  const closeSettings = async () => {
    await page.frameLocator('iframe.mock-settings').getByRole('button', { name: 'Close settings' }).click();
    await until(async () => (await regions()).settingsOpen === false);
  };
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  const settings = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  lib.watch(settings, w.errors);
  await settings.goto(`${lib.base}/settings/`);
  await settings.getByRole('heading', { name: 'Settings' }).waitFor();
  await closeSettings();
  await settings.getByRole('button', { name: 'Extensions' }).click();
  const install = async () => {
    await fireExtension('choose', packaged);
    await settings.getByRole('button', { name: 'Install package' }).click();
  };
  const prompt = ui.locator('.pbx-prompt');
  await install();
  check('an install asks in the shell, over the window', await until(async () => (await prompt.count()) === 1 && /Add "Project B test extension"\?/.test(await prompt.innerText())));
  const cover = await box('.pbx-prompt');
  check('the prompt is the shell\'s, not the page\'s', await until(async () => (await regions({ x: 600, y: 500 })).hit === 'shell' && (await regions({ x: cover.left + 10, y: cover.top + 10 })).hit === 'shell'));
  await shots('prompt');
  await prompt.locator('button:not(.primary)').click();
  check('its no button installs nothing and takes it away', await until(async () => (await prompt.count()) === 0) && (await ui.locator('.pbx-action').count()) === 0);
  await install();
  await prompt.waitFor();
  await prompt.locator('.primary').click();
  const puzzle = ui.getByRole('button', { name: 'Extensions', exact: true });
  const menu = ui.locator('.pbx-menu');
  await puzzle.click();
  check('its yes button installs it, and the Extensions menu lists it', await until(async () => (await prompt.count()) === 0 && (await menu.count()) === 1 && (await menu.innerText()).includes(extensionName)));
  const m = await box('.pbx-menu');
  check('the menu hangs under the toolbar, over the page, and is the shell\'s',
    m.top === toolbar.top + 38 && await until(async () => (await regions({ x: m.left + m.width / 2, y: m.bottom - 5 })).hit === 'shell'), JSON.stringify(m));
  await menu.getByRole('button', { name: `Pin ${extensionName}` }).click();
  const action = ui.locator('.pbx-action');
  check('Pin puts its button on the toolbar, named for it', await until(async () => (await action.count()) === 1 && (await action.getAttribute('aria-label')) === extensionName && (await action.getAttribute('data-extension')) === extension));
  check('and the menu offers Unpin', await until(async () => (await menu.getByRole('button', { name: `Unpin ${extensionName}` }).count()) === 1));
  await shots('extensions-menu');
  await page.keyboard.press('Escape');
  check('Escape closes the menu', await until(async () => (await menu.count()) === 0));
  await callExtension('action.setBadgeText', [{ text: '7' }]);
  check('a badge the extension sets shows on its button', await until(async () => (await action.locator('.pbx-badge').count()) === 1 && (await action.locator('.pbx-badge').innerText()) === '7'));
  await action.click();
  const anchor = await box('.pbx-action');
  check('a press on its button opens its popup under it', await until(async () => {
    const p = await popup();
    return p && p.extension === extension && Math.abs(p.anchor.x - anchor.x) < 0.5 && Math.abs(p.anchor.y - anchor.y) < 0.5;
  }), JSON.stringify(await popup()));
  check('and the button looks pressed while it is open', await until(async () => (await action.getAttribute('class')).includes('pbx-open')));
  await action.click();
  check('a second press closes it', await until(async () => (await popup()) === null && !(await action.getAttribute('class')).includes('pbx-open')));
  await puzzle.click();
  await menu.getByRole('button', { name: extensionName, exact: true }).click();
  const puzzleBox = await box('[aria-label=Extensions]');
  check('its name in the menu opens the popup under the Extensions button, and closes the menu', await until(async () => {
    const p = await popup();
    return p && p.extension === extension && Math.abs(p.anchor.x - puzzleBox.x) < 0.5 && (await menu.count()) === 0;
  }), JSON.stringify(await popup()));
  await fireExtension('closePopup', '');
  await until(async () => (await popup()) === null);
  await puzzle.click();
  await menu.getByRole('button', { name: `Unpin ${extensionName}` }).click();
  check('Unpin takes the button off the toolbar', await until(async () => (await action.count()) === 0));
  await menu.getByRole('button', { name: `Pin ${extensionName}` }).click();
  await until(async () => (await action.count()) === 1);
  await menu.locator('.pbx-manage').click();
  check('Manage extensions opens the settings page, and closes the menu', await until(async () => (await regions()).settingsOpen === true && (await menu.count()) === 0));
  await closeSettings();
  await settings.close();

  // Four tabs, the first active, as the reference has them.
  await tabs.first().click();
  await until(async () => (await active()).url === 'https://example.com/news/today');
  let missed = [];
  check('with four tabs on top every control is the shell\'s', await until(async () => (missed = await notTheShells()).length === 0), missed.join(' | '));
  await shots('top-4');

  // Many tabs: each shrinks to 96px, then the tabs scroll, with the active one in view and
  // New Tab and the drag space in place.
  const hosts = ['calendar', 'maps', 'music', 'search', 'shopping', 'bank', 'recipes', 'flights', 'hotels', 'wiki', 'video', 'photos', 'chat',
    'notes', 'tasks', 'releases', 'issues', 'pulls', 'design', 'docs', 'news', 'sports', 'books', 'games', 'travel', 'jobs'];
  for (let i = 0; i < hosts.length; i++) await fire('menu', 'tab.new');
  check('thirty tabs', await tabCount(30));
  for (const [i, t] of (await regions()).tabs.slice(4).entries()) await fire('navigate', `https://${hosts[i]}.test/`, { tab: t.id });
  await until(async () => (await regions()).tabs.every((t) => t.title !== '' && !t.loading), 10000);
  const widths = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('with thirty tabs each is 96px', widths.every((x) => x === 96), JSON.stringify(widths));
  let s = await scroller();
  check('the tabs scroll sideways', s.scrollWidth > s.clientWidth, JSON.stringify(s));
  check('the new active tab, the last, is scrolled into view', await until(activeInView), JSON.stringify(s));
  const plus = await newTab.boundingBox();
  const strip = await box('.tabs');
  r = await regions();
  const dragEnd = r.drag.find((d) => d.x + d.w === r.window.W);
  check('New Tab stays right after the tabs, in the window', plus.x >= strip.right - 0.5 && plus.x + plus.width <= r.window.W, JSON.stringify({ plus, strip }));
  check('and the drag space after it, at least 48px, drags the window',
    !!dragEnd && dragEnd.w >= 48 && dragEnd.x >= plus.x + plus.width - 0.5 && (await regions({ x: dragEnd.x + dragEnd.w / 2, y: 17 })).hit === 'drag', JSON.stringify(r.drag));
  await fire('menu', 'tab.select1');
  check('making the first tab active scrolls it into view', await until(async () => (await activeInView()) && (await scroller()).scrollLeft === 0), JSON.stringify(await scroller()));
  const first = await tabs.nth(2).boundingBox();
  await page.mouse.move(first.x + 10, first.y + 10);
  await page.mouse.wheel(0, 400);
  check('a mouse wheel scrolls the tabs sideways', await until(async () => (await scroller()).scrollLeft > 0), JSON.stringify(await scroller()));
  await fire('menu', 'tab.selectLast');
  await until(activeInView);
  await tabs.nth(26).click();
  check('a click on a tab in view makes it active, and it stays in view', await until(async () => (await regions()).active === (await regions()).tabs[26].id && (await activeInView())));
  check('and with thirty, scrolled', await until(async () => (missed = await notTheShells()).length === 0), missed.join(' | '));
  await shots('top-30');

  // The tabs down the side.
  await fire('menu', 'view.tabsSide');
  check('with the tabs on the side there is no tab bar, and a list of the thirty', await until(async () => (await ui.locator('.tabbar').count()) === 0 && (await ui.locator('.side').count()) === 1) && await tabCount(30));
  await settle(page);
  r = await regions();
  const side = await box('.side');
  const top = await box('.toolbar');
  check('the toolbar is the top row, the list 220px wide under it, the page beside the list',
    top.top === 0 && top.height === 39 && side.top === 39 && side.width === 221 && r.visible[0].rect.x === 221 && r.visible[0].rect.y === 39, JSON.stringify({ top, side, page: r.visible[0].rect }));
  const room = await box('.toolbar .lights');
  check('the toolbar starts after the window buttons\' space, which drags the window',
    room.right >= c.x + c.w && (await regions({ x: room.width / 2, y: 10 })).hit === 'drag', JSON.stringify(room));
  s = await scroller();
  check('the list scrolls, and the active tab is in view', s.scrollHeight > s.clientHeight && await until(activeInView), JSON.stringify(s));
  const sideNew = await newTab.boundingBox();
  check('New Tab is pinned at the top of the list', sideNew.y === 39 && sideNew.x === 0 && sideNew.width === 220, JSON.stringify(sideNew));
  await fire('menu', 'tab.select1');
  check('making the first tab active scrolls the list to it', await until(async () => (await activeInView()) && (await scroller()).scrollTop === 0));
  await fire('menu', 'tab.selectLast');
  check('and the last', await until(async () => (await activeInView()) && (await scroller()).scrollTop > 0));
  await newTab.click();
  check('New Tab in the list opens a tab, active and in view', await tabCount(31) && await until(activeInView));
  await tell({ t: 'closeTab', id: (await regions()).active });
  await tabCount(30);
  await tabs.nth(26).click();
  await until(activeInView);
  await shots('side-30');
  const keep = (await regions()).tabs.slice(0, 4).map((t) => t.id);
  for (const t of (await regions()).tabs) if (!keep.includes(t.id)) await tell({ t: 'closeTab', id: t.id });
  await tabCount(4);
  await fire('menu', 'tab.select1');
  await until(async () => (await regions()).active === keep[0]);
  await shots('side-4');
  await fire('menu', 'view.tabsTop');
  await ui.locator('.tabbar').waitFor();

  // Light and dark, from the system's setting, which the shell follows by default.
  const bars = async (scheme) => {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    return w.shell().evaluate(() => ['.tabbar', '.toolbar', '.address'].map((s) => getComputedStyle(document.querySelector(s)).backgroundColor));
  };
  const light = await bars('light');
  const dark = await bars('dark');
  await page.emulateMedia({ colorScheme: 'light' });
  check('light and dark paint the bars and the field each their own', light.every((v, i) => v !== dark[i]) && new Set(light).size === 3, JSON.stringify({ light, dark }));

  check('no errors logged', w.errors.length === 0, w.errors.join(' | '));

  // The notices: last run's crash, and an update ready. The app is started again on the
  // same profile with a crash log left behind, as a crash leaves it, and a release staged,
  // as a download that passed its checks leaves it, newer than the version it runs as.
  await lib.stop();
  const temp = path.dirname(lib.profile);
  const bundle = (dir, version) => {
    const app = path.join(dir, 'Project B.app');
    fs.mkdirSync(path.join(app, 'Contents/MacOS'), { recursive: true });
    fs.writeFileSync(path.join(app, 'Contents/Info.plist'), `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleShortVersionString</key><string>${version}</string>
</dict></plist>
`);
    fs.writeFileSync(path.join(app, 'Contents/MacOS/project_b'), '#!/bin/sh\n', { mode: 0o755 });
    return app;
  };
  const installed = bundle(path.join(temp, 'installed'), '0.1.0');
  bundle(path.join(lib.profile, 'updates/stage'), '0.2.0');
  fs.writeFileSync(path.join(lib.profile, 'updates/settings.json'), JSON.stringify({ autoUpdate: false }));
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), `pid 1, started 2026-11-13 10:00:00\n`);
  await lib.start([`-use=${id}`, '-version=0.1.0', `-releases=${path.join(temp, 'releases.json')}`, `-app=${installed}`]);
  await page.goto(`${lib.base}/?window=1`);
  w.errors.length = 0;
  await tabCount(1);

  const crash = ui.locator('.crash');
  check('last run\'s crash shows a bar under the toolbar, saying where its log is', await until(async () => (await crash.count()) === 1 && /crashed last time\. The log is at .*crash-.*\.log/.test(await crash.innerText())));
  await settle(page);
  const bar = await box('.crash');
  r = await regions();
  check('one line, and the page under it', bar.top === 74 && bar.height === 33 && r.visible[0].rect.y === 107, JSON.stringify({ bar, page: r.visible[0].rect }));
  const relaunch = ui.getByRole('button', { name: 'Relaunch to update' });
  check('an update ready shows Relaunch to update in the toolbar', await until(async () => (await relaunch.count()) === 1 && (await ui.locator('.toolbar').getByRole('button', { name: 'Relaunch to update' }).count()) === 1, 10000));
  await shots('notices');
  await crash.getByRole('button', { name: 'Dismiss' }).click();
  check('Dismiss closes the crash bar, and the page moves up', await until(async () => (await crash.count()) === 0 && (await regions()).visible[0].rect.y === 74));
  await relaunch.click();
  const version = () => /CFBundleShortVersionString<\/key><string>([^<]+)/.exec(fs.readFileSync(path.join(installed, 'Contents/Info.plist'), 'utf8'))[1];
  check('Relaunch to update puts the release in place and ends the app, to start it', await until(async () => (await regions()).quit === true && version() === '0.2.0'));

  check('no errors logged after the restart', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
