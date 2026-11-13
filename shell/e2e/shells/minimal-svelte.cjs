// The minimal-svelte shell's own tests, on the mock engine: no band, the tab bar and the
// toolbar, one tab and many, the strip that scrolls with New Tab and the drag space kept,
// the side list, the close button, the address field and its suggestions, the extensions'
// buttons, menu and prompt from the mock store, the update and crash notices, light and
// dark. The check covers what every shell does.
// Run: node shell/e2e/shells/minimal-svelte.cjs   (from the repo root, after `npm run build`
// in shell/, and tools/pack-test-extension.py for the store)

const { execFileSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

const id = 'minimal-svelte';
// The test extension, as tools/pack-test-extension.py packs it into the mock store.
const extension = 'oepjjojhhaclfhmopocgmdfbeiffpheh';
const extensionName = 'Project B test extension';
const packaged = path.join(lib.root, `.tools/extensions/crx/${extension}_1.0.crx`);
if (!fs.existsSync(packaged)) {
  console.error('No mock store: run tools/pack-test-extension.py first.');
  process.exit(2);
}

// What the tabs are called in the screenshots.
const names = ['Example Domain', 'Mail · Work', 'Project B on GitHub', 'Weather', 'News today', 'Docs: shells', 'Calendar', 'Maps', 'Music', 'Search results', 'Shopping cart', 'Bank', 'Recipes', 'Flights', 'Hotel booking', 'Wiki: browsers', 'Video', 'Photos', 'Chat', 'Notes', 'Tasks', 'Release notes', 'Issue 42', 'Pull request 7', 'Design system', 'Maps again', 'Podcasts', 'Translate', 'Forum', 'Status'];

(async () => {
  // The Extensions flag, as the settings page saves it, and a workspace for a tab to be in.
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ extensions: 'on' }));
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#d97706' }] }));
  await lib.start([`-use=${id}`]);
  const browser = await chromium.launch();
  let w = await lib.openWindow(browser);
  let { page, ui } = w;
  page.setDefaultTimeout(5000);

  const tabs = ui.locator('[role=tab]');
  const field = ui.getByRole('combobox', { name: 'Address' });
  const options = ui.getByRole('option');
  const newTab = ui.getByRole('button', { name: 'New Tab', exact: true });
  const near = (a, b) => Math.abs(a - b) <= 1;
  const box = (selector) => w.shell().evaluate((s) => document.querySelector(s)?.getBoundingClientRect().toJSON(), selector);
  const activeURL = async () => {
    const r = await regions();
    return r.tabs.find((t) => t.id === r.active).url;
  };
  const tabCount = (n) => until(async () => (await regions()).tabs.length === n && (await tabs.count()) === n);
  // A person's pointer arrives in many moves; the window gives the shell the pointer on
  // the first move into its region, so the hover begins with the second.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  // What a press at the middle of locator, or at x, y, does: shell, drag, or page.
  const hitAt = async (x, y) => (await regions({ x, y })).hit;
  const hitOf = async (locator) => {
    const b = await locator.boundingBox();
    return hitAt(b.x + b.width / 2, b.y + b.height / 2);
  };
  const becomes = (what, fn) => until(async () => (await fn()) === what);
  // The scroller the tabs are in, top or side, and whether tab id shows whole in it.
  const strip = (selector) => w.shell().evaluate((s) => {
    const el = document.querySelector(s);
    return { scrollWidth: el.scrollWidth, clientWidth: el.clientWidth, scrollLeft: el.scrollLeft, scrollHeight: el.scrollHeight, clientHeight: el.clientHeight, scrollTop: el.scrollTop };
  }, selector);
  const inView = (selector, id) => w.shell().evaluate(([s, id]) => {
    const v = document.querySelector(s).getBoundingClientRect();
    const t = document.querySelector(`[role=tab][data-tab="${id}"]`).getBoundingClientRect();
    return t.left >= v.left - 1 && t.right <= v.right + 1 && t.top >= v.top - 1 && t.bottom <= v.bottom + 1;
  }, [selector, id]);
  const visit = async (text, url) => {
    await field.click();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type(text);
    await page.keyboard.press('Enter');
    return until(async () => (await activeURL()) === url && (await field.inputValue()) === url && !(await regions()).tabs.find((t) => t.url === url).loading);
  };
  // Each of the first n tabs gets the reference's name for its place, once its page has
  // loaded, as a page sets its title.
  const name = async () => {
    const r = await regions();
    for (const [i, t] of r.tabs.entries()) if (t.title !== names[i]) await fire('title', names[i], { tab: t.id });
    await until(async () => (await regions()).tabs.every((t, i) => t.title === names[i]));
  };
  // n tabs, each with a page loaded.
  const opened = async (n) => {
    for (let i = (await regions()).tabs.length; i < n; i++) await fire('menu', 'tab.new');
    await tabCount(n);
    for (const [i, t] of (await regions()).tabs.entries()) if (!t.url) await fire('navigate', `https://example.com/${i + 1}`, { tab: t.id });
    return until(async () => (await regions()).tabs.every((t) => t.url && !t.loading));
  };
  // The controls the shell draws that a press at their middle doesn't reach, as the
  // check's rule 6 has it.
  const { controls } = await import('../../src/sdk/controls.js');
  const strays = async () => {
    const found = await w.shell().evaluate((control) => [...document.querySelectorAll(control)]
      .filter((el) => el.checkVisibility({ visibilityProperty: true, opacityProperty: true }))
      .map((el) => ({ what: el.outerHTML.slice(0, 60), r: el.getBoundingClientRect().toJSON() }))
      .filter(({ r }) => r.width > 0 && r.height > 0 && r.right > 0 && r.bottom > 0 && r.left < innerWidth && r.top < innerHeight), controls);
    const out = [];
    for (const { what, r } of found) {
      if ((await hitAt(Math.min(r.x + r.width / 2, 1099), Math.min(r.y + r.height / 2, 699))) !== 'shell') out.push(what);
    }
    return out;
  };
  // Light and dark, from the system's setting, which the shell follows by default.
  const shots = async (what) => {
    await page.mouse.move(1099, 699);
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `${id}-${what}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  // One tab, and no band: the bars and the page reach the window's edges.
  check('the window loads the minimal-svelte shell with its one tab', await until(async () => (await regions()).shell === id && (await tabs.count()) === 1 && (await regions()).visible.length === 1, 10000));
  await w.shell().evaluate(() => document.fonts.ready);
  check('Space Grotesk and Space Mono load from the shell\'s own folder',
    await w.shell().evaluate(() => [...document.fonts].filter((f) => f.status === 'loaded').map((f) => f.family.replace(/"/g, '')).sort().join() === 'Space Grotesk,Space Mono'));
  let r = await regions();
  check('it asks for no band', r.frameWidth === 0, String(r.frameWidth));

  // Tabs down the side, the pref's default: no tab bar, the toolbar on top after the
  // window buttons, the list on the left under it, the page beside it.
  check('with the tabs down the side there is no tab bar, and a list of the tabs', (await ui.locator('.tabbar').count()) === 0 && (await ui.locator('.side [role=tab]').count()) === 1);
  let toolbar = await box('.toolbar');
  let side = await box('.side');
  const back = await ui.getByRole('button', { name: 'Back' }).boundingBox();
  const c = r.window.Controls;
  check('the toolbar is the top row, 38px and a hairline, its first button after the window buttons',
    toolbar.y === 0 && toolbar.height === 39 && back.x >= c.x + c.w, JSON.stringify({ toolbar, back, controls: c }));
  check('the list is 220px wide and a hairline, under the toolbar, down to the foot', side.x === 0 && side.width === 221 && side.y === 39 && side.bottom === 700, JSON.stringify(side));
  let p = r.visible[0].rect;
  check('the page fills the rest, to the window\'s edges', p.x === 221 && p.y === 39 && p.x + p.w === r.window.W && p.y + p.h === r.window.H, JSON.stringify(p));
  check('the window buttons\' space in the toolbar drags', (await hitAt(c.x + c.w + 4, 19)) === 'drag');

  // Tabs on top.
  await fire('menu', 'view.tabsTop');
  check('with the tabs on top there is a tab bar, and no list', await until(async () => (await ui.locator('.tabbar [role=tab]').count()) === 1 && (await ui.locator('.side').count()) === 0));
  await settle(page);
  const bar = await box('.tabbar');
  toolbar = await box('.toolbar');
  r = await regions();
  p = r.visible[0].rect;
  check('the tab bar is 34px and a hairline, the toolbar 38px and a hairline, edge to edge',
    bar.x === 0 && bar.y === 0 && bar.width === 1100 && bar.height === 35 && toolbar.y === 35 && toolbar.height === 39 && toolbar.width === 1100, JSON.stringify({ bar, toolbar }));
  check('the page lies under them, to the window\'s edges', await until(async () => {
    const v = (await regions()).visible[0].rect;
    return v.x === 0 && v.y === 74 && v.w === 1100 && v.y + v.h === 700;
  }), JSON.stringify((await regions()).visible));
  const first = await tabs.first().boundingBox();
  check('the tabs begin after the window buttons\' space', first.x >= c.x + c.w, JSON.stringify({ first, controls: c }));
  check('the window buttons\' space drags', (await hitAt(c.x + c.w + 4, 10)) === 'drag');
  check('one tab is 200px wide', near(first.width, 200), String(first.width));

  // The toolbar: Reload is Stop while the page loads, and a mark shows in the tab.
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('example.com/news/today');
  const loading = w.shell().waitForFunction(() => document.querySelector('[aria-label=Stop]') && document.querySelector('.tab.active [aria-label=Loading]'), null, { polling: 10, timeout: 3000 }).then(() => true, () => false);
  await page.keyboard.press('Enter');
  check('while the page loads Reload is Stop, and the tab shows a loading mark', await loading);
  check('once it has loaded the mark goes, and Stop is Reload again',
    await until(async () => (await activeURL()) === 'https://example.com/news/today' && (await ui.locator('[aria-label=Loading]').count()) === 0 && (await ui.getByRole('button', { name: 'Reload' }).isEnabled())));
  check('the address field shows the active tab\'s address', (await field.inputValue()) === 'https://example.com/news/today', await field.inputValue());
  check('the address is in Space Mono, the rest in Space Grotesk',
    (await field.evaluate((el) => getComputedStyle(el).fontFamily)).startsWith('"Space Mono"') && (await tabs.first().evaluate((el) => getComputedStyle(el).fontFamily)).startsWith('"Space Grotesk"'));
  check('the keyboard went back to the page', await until(async () => (await regions()).focused === (await regions()).active));

  // focusAddressBar puts the keyboard in the field with the address selected.
  await fire('key', 'focusAddressBar', { tab: (await regions()).active });
  check('focusAddressBar puts the keyboard in the field, its text selected', await until(() => field.evaluate((el) =>
    el === document.activeElement && el.selectionStart === 0 && el.selectionEnd === el.value.length && el.value.length > 0)));
  await page.keyboard.press('Escape');
  await until(async () => (await regions()).focused === (await regions()).active);

  // Four tabs, as the reference draws them: the first active, the second in a workspace.
  await fire('menu', 'workspaces:0a1b2c3d');
  await tabCount(2);
  await visit('example.net/mail', 'https://example.net/mail');
  await fire('menu', 'tab.select1');
  await until(async () => (await regions()).active === (await regions()).tabs[0].id);
  await newTab.click();
  check('New Tab opens a tab after the last, active, with the keyboard in the field',
    await tabCount(3) && (await tabs.nth(2).getAttribute('aria-selected')) === 'true' && await until(async () => (await field.evaluate((el) => el === document.activeElement)) && (await field.inputValue()) === ''));
  await visit('example.org/project', 'https://example.org/project');
  await fire('menu', 'tab.new');
  await tabCount(4);
  await visit('example.com/weather', 'https://example.com/weather');
  await fire('menu', 'tab.select1');
  await until(async () => (await regions()).active === (await regions()).tabs[0].id);
  check('a tab in a workspace shows its colour as a small square, the others none',
    (await ui.locator('.tab .dot').count()) === 1 && (await tabs.nth(1).locator('.dot').evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(217, 119, 6)');
  await name();
  const widths = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('four tabs are 200px wide each', widths.every((x) => near(x, 200)), JSON.stringify(widths));
  const closeOf = (i) => tabs.nth(i).locator('.x');
  const shown = (loc) => loc.evaluate((el) => el.checkVisibility());
  await page.mouse.move(600, 400);
  await settle(page);
  check('the active tab shows its close button, the others don\'t', (await shown(closeOf(0))) && !(await shown(closeOf(1))) && !(await shown(closeOf(3))));
  await hover(tabs.nth(3));
  check('a tab pointed at shows its close button', await until(() => shown(closeOf(3))));
  const active = await tabs.first().evaluate((el) => ({ bg: getComputedStyle(el).backgroundColor, h: el.getBoundingClientRect().height }));
  const toolbarBg = await ui.locator('.toolbar').evaluate((el) => getComputedStyle(el).backgroundColor);
  const other = await tabs.nth(1).evaluate((el) => el.getBoundingClientRect().height);
  check('the active tab has the toolbar\'s tone and covers the bar\'s hairline to join it', active.bg === toolbarBg && active.h === 35 && other === 34, JSON.stringify({ active, toolbarBg, other }));
  check('with the tabs on top every control is the shell\'s', await until(async () => (await strays()).length === 0), (await strays()).join(' | '));
  await shots('top-4');

  // Close, on the active tab.
  await tabs.nth(3).click();
  await until(async () => (await regions()).active === (await regions()).tabs[3].id);
  const closing = (await regions()).active;
  await closeOf(3).click();
  check('Close on the active tab closes it, and the shell marks the tab Go makes active', await tabCount(3) && !(await regions()).tabs.some((t) => t.id === closing) &&
    await until(async () => (await tabs.evaluateAll((els) => els.filter((el) => el.getAttribute('aria-selected') === 'true').map((el) => Number(el.dataset.tab)))).join() === String((await regions()).active)));
  await fire('menu', 'tab.new');
  await tabCount(4);
  await visit('example.com/weather', 'https://example.com/weather');
  await name();
  await tabs.first().click();
  await until(async () => (await regions()).active === (await regions()).tabs[0].id);

  // Suggestions, from the pages visited.
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  check('typing lists suggestions', await until(async () => (await options.count()) >= 2), String(await options.count()));
  const fieldBox = await ui.locator('.address input').boundingBox();
  const listBox = await ui.getByRole('listbox').boundingBox();
  check('in a plain panel under the field, as wide as it', near(listBox.y, fieldBox.y + fieldBox.height) && near(listBox.x, fieldBox.x) && near(listBox.width, fieldBox.width), JSON.stringify({ fieldBox, listBox }));
  check('a press on it reaches the shell, over the page', listBox.y + listBox.height > (await regions()).visible[0].rect.y && await becomes('shell', () => hitOf(options.last())));
  await shots('suggestions');
  await page.keyboard.press('ArrowDown');
  check('Down chooses one, and the field shows it', (await ui.locator('[role=option][aria-selected=true]').count()) === 1 && (await field.inputValue()).startsWith('https://example'), await field.inputValue());
  await page.keyboard.press('ArrowUp');
  check('Up from the first comes back to what was typed', (await field.inputValue()) === 'exam', await field.inputValue());
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  const chosen = await field.inputValue();
  await page.keyboard.press('Enter');
  check('Enter goes to the one chosen', await until(async () => (await activeURL()) === chosen && (await options.count()) === 0), JSON.stringify({ chosen, now: await activeURL() }));
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  await until(async () => (await options.count()) >= 2);
  const target = await options.first().locator('.url').innerText();
  await options.first().click();
  check('a click on one goes there', await until(async () => (await activeURL()) === target && (await options.count()) === 0), JSON.stringify({ target, now: await activeURL() }));
  const before = await activeURL();
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  await until(async () => (await options.count()) >= 2);
  await page.keyboard.press('Escape');
  check('Escape puts the address back, closes the list and gives the page the keyboard',
    await until(async () => (await field.inputValue()) === before && (await options.count()) === 0 && (await regions()).focused === (await regions()).active),
    JSON.stringify({ value: await field.inputValue(), focused: (await regions()).focused }));
  // Back to the first page's address for the pictures.
  await visit('example.com/news/today', 'https://example.com/news/today');
  await name();

  // The extensions: the Extensions button, its menu, the prompt an install asks with.
  const extensions = ui.getByRole('button', { name: 'Extensions', exact: true });
  const menu = ui.getByRole('group', { name: 'Extensions menu' });
  check('the toolbar has the Extensions button', await extensions.isVisible());
  await extensions.click();
  check('it opens its menu, with Manage extensions', await until(() => menu.locator('.pbx-manage').isVisible()) && (await extensions.getAttribute('aria-expanded')) === 'true');
  check('a press on the menu reaches the shell, over the page', await becomes('shell', () => hitOf(menu.locator('.pbx-manage'))));
  await extensions.click();
  check('its button closes it again', await until(async () => (await menu.count()) === 0));
  await extensions.click();
  await menu.waitFor();
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await menu.count()) === 0));

  // An install from the mock store's package, begun on a settings page of the test's own,
  // asks in the shell too. That page talks to Go once Go has opened settings, and stays
  // usable once Go has closed the one over the shell, which would cover the prompt.
  const closeSettings = async () => {
    await page.frameLocator('iframe.mock-settings').getByRole('button', { name: 'Close settings' }).click();
    await until(async () => (await regions()).settingsOpen === false);
  };
  await ui.getByRole('button', { name: 'Settings' }).click();
  await until(async () => (await regions()).settingsOpen === true);
  const settings = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  const settingsErrors = [];
  lib.watch(settings, settingsErrors);
  await settings.goto(lib.base + '/settings/');
  await settings.getByRole('button', { name: 'Extensions' }).click();
  await closeSettings();
  const install = async () => {
    await fetch(`${lib.base}/mock/extensions/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'choose', extension: '', value: packaged }) });
    await settings.getByRole('button', { name: 'Install package' }).click();
  };
  const prompt = ui.locator('.pbx-prompt');
  await install();
  check('an install asks in the shell, naming the extension and what it may do', await until(() => prompt.isVisible()) &&
    /Add "Project B test extension"\?/.test(await prompt.innerText()) && (await prompt.innerText()).includes('storage'), await prompt.innerText().catch(() => ''));
  check('while it asks, a press anywhere in the window reaches the shell', await becomes('shell', () => hitAt(550, 600)));
  await shots('prompt');
  await prompt.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel answers no: the prompt goes, nothing installed', await until(async () => (await prompt.count()) === 0) && await until(async () => {
    await extensions.click();
    const none = (await menu.locator('.row').count()) === 0;
    await page.keyboard.press('Escape');
    return none;
  }));
  await install();
  await prompt.waitFor();
  await prompt.locator('.primary').click();
  check('Add extension, its primary button, installs it', await until(async () => (await prompt.count()) === 0) && await until(async () => {
    await extensions.click();
    const listed = (await menu.locator('.row').count()) === 1 && (await menu.innerText()).includes(extensionName);
    if (!listed) await page.keyboard.press('Escape');
    return listed;
  }));
  const action = ui.locator('.pbx-action');
  const pinned = (on) => menu.getByRole('button', { name: `${on ? 'Unpin' : 'Pin'} ${extensionName}` });
  await pinned(false).click();
  check('Pin puts its button on the toolbar, with its icon', await until(async () => (await action.count()) === 1 && (await action.getAttribute('aria-label')) === extensionName && (await action.locator('img').count()) === 1));
  check('and the menu shows it pinned', await until(async () => (await pinned(true).count()) === 1));
  await page.keyboard.press('Escape');
  await until(async () => (await menu.count()) === 0);
  await fetch(`${lib.base}/mock/extensions/call?extension=${extension}&name=action.setBadgeText&tab=0`, { method: 'POST', body: JSON.stringify([{ text: '7' }]) });
  check('a badge the extension sets shows on its button', await until(async () => (await action.locator('.pbx-badge').count()) === 1 && (await action.locator('.pbx-badge').innerText()) === '7'));
  await action.click();
  check('a press on it opens its popup, and it looks pressed while it is open', await until(async () => (await (await fetch(`${lib.base}/mock/extensions`)).json()).popup?.extension === extension && (await action.getAttribute('class')).includes('pbx-open')));
  await field.click();
  check('a press elsewhere in the shell closes it', await until(async () => (await (await fetch(`${lib.base}/mock/extensions`)).json()).popup === null && !(await action.getAttribute('class')).includes('pbx-open')));
  await page.keyboard.press('Escape');
  await extensions.click();
  await menu.locator('.pbx-manage').click();
  check('Manage extensions opens the settings page, and closes the menu', await until(async () => (await regions()).settingsOpen === true) && (await menu.count()) === 0);
  await closeSettings();
  await settings.close();
  check('the settings page logged no errors', settingsErrors.length === 0, settingsErrors.join(' | '));
  await shots('extensions');

  // Thirty tabs on top: each shrinks to 96px, then the strip scrolls; the active one stays
  // in view, and New Tab and the drag space stay.
  await opened(30);
  await name();
  const narrow = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('with thirty tabs each is 96px wide', narrow.every((x) => near(x, 96)), JSON.stringify(narrow));
  let s = await strip('.tabs');
  check('the strip scrolls', s.scrollWidth > s.clientWidth, JSON.stringify(s));
  let last = (await regions()).active;
  check('the active tab, the last, is scrolled into view', await until(() => inView('.tabs', last)));
  const newBox = await newTab.boundingBox();
  const dragBox = await ui.locator('.tabbar .drag').boundingBox();
  check('New Tab stays in view after the strip', newBox.x >= (await ui.locator('.tabs').boundingBox()).x + s.clientWidth && newBox.x + newBox.width <= 1100 && (await hitOf(newTab)) === 'shell', JSON.stringify(newBox));
  check('and so does the space that drags, at least 48px', dragBox.width >= 48 && dragBox.x + dragBox.width <= 1100 && (await hitOf(ui.locator('.tabbar .drag'))) === 'drag', JSON.stringify(dragBox));
  const end = s.scrollLeft;
  const stripBox = await ui.locator('.tabs').boundingBox();
  await page.mouse.move(stripBox.x + 100, stripBox.y + 10);
  await page.mouse.wheel(0, -300);
  check('a wheel turns the strip sideways', await until(async () => (await strip('.tabs')).scrollLeft < end), JSON.stringify(await strip('.tabs')));
  await fire('menu', 'tab.select1');
  const firstId = (await regions()).tabs[0].id;
  check('making a far tab active scrolls it into view', await until(async () => (await regions()).active === firstId && (await inView('.tabs', firstId)) && (await strip('.tabs')).scrollLeft === 0), JSON.stringify(await strip('.tabs')));
  // As the reference has it: the eighteenth active.
  await fire('menu', 'tab.selectLast');
  await until(async () => inView('.tabs', (await regions()).active));
  await tabs.nth(29).waitFor();
  const eighteenth = (await regions()).tabs[17].id;
  await tabs.nth(17).click();
  check('a click on a tab makes it active, in view', await until(async () => (await regions()).active === eighteenth && (await inView('.tabs', eighteenth))));
  check('every page lies inside the window, under the bars', (await regions()).visible.every((v) => v.rect.y === 74 && v.rect.x === 0));
  await shots('top-30');

  // Thirty down the side: the list scrolls and keeps the active one in view.
  await fire('menu', 'view.tabsSide');
  check('the tabs move to the side', await until(async () => (await ui.locator('.side [role=tab]').count()) === 30 && (await ui.locator('.tabbar').count()) === 0));
  s = await strip('.list');
  check('the list scrolls', s.scrollHeight > s.clientHeight, JSON.stringify(s));
  check('the active tab is in view in it', await until(() => inView('.list', eighteenth)));
  await fire('menu', 'tab.selectLast');
  last = (await regions()).tabs[29].id;
  check('making the last tab active scrolls the list to it', await until(async () => (await regions()).active === last && (await inView('.list', last))));
  await fire('menu', 'tab.select1');
  check('and the first, back up', await until(async () => (await inView('.list', firstId)) && (await strip('.list')).scrollTop === 0));
  const sideNew = await newTab.boundingBox();
  check('New Tab stays at the top of the list', sideNew.y === 39 && (await hitOf(newTab)) === 'shell', JSON.stringify(sideNew));
  await tabs.nth(17).click();
  await until(async () => (await regions()).active === eighteenth && (await inView('.list', eighteenth)));
  await shots('side-30');
  await newTab.click();
  check('New Tab in the list opens a tab, active and in view', await tabCount(31) && await until(async () => inView('.list', (await regions()).active)));

  // Four down the side.
  while ((await regions()).tabs.length > 4) {
    const n = (await regions()).tabs.length;
    await fire('menu', 'tab.close');
    await tabCount(n - 1);
  }
  await fire('menu', 'tab.select1');
  await until(async () => (await regions()).active === (await regions()).tabs[0].id);
  await name();
  await shots('side-4');

  // Light and dark: the bars take the tones of the theme.
  const tone = () => ui.locator('.toolbar').evaluate((el) => {
    const [r, g, b] = getComputedStyle(el).backgroundColor.match(/\d+/g).map(Number);
    return (r + g + b) / 3;
  });
  await page.emulateMedia({ colorScheme: 'dark' });
  const dark = await until(async () => (await tone()) < 64);
  await page.emulateMedia({ colorScheme: 'light' });
  check('the bars are dark in dark and light in light', dark && await until(async () => (await tone()) > 192));
  check('no errors logged', w.errors.length === 0, w.errors.join(' | '));
  await page.close();

  // The notices. The last run crashed: Go finds the crash log it left in the profile. And
  // an update is ready: the real updater fetches a release from a server of the test's own
  // and stages it.
  await lib.stop();
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-13 09:00:00\n');
  const releases = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-releases-'));
  const fakeApp = (dir, version) => {
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
  const zip = path.join(releases, 'release.zip');
  execFileSync('ditto', ['-c', '-k', '--keepParent', fakeApp(path.join(releases, 'new'), '0.2.0'), zip]);
  const body = fs.readFileSync(zip);
  const server = http.createServer((req, res) => {
    if (req.url !== '/releases.json') return res.end(body);
    res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' },
      releases: { '0.2.0': { assets: { 'darwin/arm64': { url: `http://127.0.0.1:${server.address().port}/asset.zip`, sha256: crypto.createHash('sha256').update(body).digest('hex'), size: body.length } } } } }));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const installed = fakeApp(path.join(releases, 'installed'), '0.1.0');
  await lib.start([`-use=${id}`, '-version=0.1.0', `-releases=http://127.0.0.1:${server.address().port}/releases.json`, `-app=${installed}`]);
  w = await lib.openWindow(browser);
  ({ page, ui } = w);
  page.setDefaultTimeout(5000);
  await until(async () => (await regions()).shell === id && (await ui.locator('[role=tab]').count()) === 1, 10000);
  const crash = ui.locator('.crash');
  check('last run\'s crash shows as one line under the toolbar', await until(() => crash.isVisible()) &&
    (await crash.innerText()).includes('crashed last time') && near((await crash.boundingBox()).y, (await box('.toolbar')).bottom) && (await crash.boundingBox()).height === 37);
  check('the page lies under it', await until(async () => (await regions()).visible[0].rect.y === (await crash.boundingBox()).y + 37));
  const relaunch = ui.getByRole('button', { name: 'Relaunch to update' });
  check('an update that is ready shows Relaunch to update in the toolbar', await until(() => relaunch.isVisible(), 15000) &&
    (await relaunch.evaluate((el) => !!el.closest('.toolbar'))) && (await hitOf(relaunch)) === 'shell');
  await shots('notices');
  await crash.getByRole('button', { name: 'Dismiss' }).click();
  check('Dismiss closes the crash notice', await until(async () => (await crash.count()) === 0 && (await regions()).visible[0].rect.y === 39));
  await relaunch.click();
  check('Relaunch to update puts the release in place and quits, to start it', await until(async () => (await regions()).quit === true, 10000));
  check('no errors logged with the notices', w.errors.length === 0, w.errors.join(' | '));
  server.close();
  fs.rmSync(releases, { recursive: true, force: true });

  await browser.close();
  await lib.done();
})();
