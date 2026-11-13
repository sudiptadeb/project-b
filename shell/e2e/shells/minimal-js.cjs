// The minimal-js shell's own tests: the tab bar on top with one tab and many, where the
// tabs shrink and then scroll with New Tab and the drag space staying put; the list down
// the side with many; the close button; the address field and its suggestions; the
// extensions' button, menu and prompt from the mock store; the crash and update notices;
// light and dark. The check covers what every shell does.
// Run: node shell/e2e/shells/minimal-js.cjs   (from the repo root, after `npm run build` in
// shell/, and tools/pack-test-extension.py for the store's package)

const { execFileSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, out, regions, settle, until } = lib;

const id = 'minimal-js';
// The test extension's package, as tools/pack-test-extension.py packs it into the store.
const extension = 'oepjjojhhaclfhmopocgmdfbeiffpheh';
const extensionName = 'Project B test extension';
const packaged = path.join(lib.root, `.tools/extensions/crx/${extension}_1.0.crx`);
// The tab names the look's mock-up has, so the screenshots compare with it.
const names = ['Example Domain', 'Mail · Work', 'Project B on GitHub', 'Weather', 'News today', 'Docs: shells', 'Calendar', 'Maps', 'Music', 'Search results', 'Shopping cart', 'Bank', 'Recipes', 'Flights', 'Hotel booking', 'Wiki: browsers', 'Video', 'Photos', 'Chat', 'Notes', 'Tasks', 'Release notes', 'Issue 42', 'Pull request 7', 'Design system', 'Travel', 'Podcasts', 'Recipes 2', 'Code review', 'Inbox'];

(async () => {
  if (!fs.existsSync(packaged)) {
    console.error('No test extension in the mock store: run tools/pack-test-extension.py first.');
    process.exit(2);
  }
  // A workspace, so a tab can be in one; the Extensions flag on; and a crash.log left by a
  // run that didn't stop, which the next start reports.
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#d97706' }] }));
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ extensions: 'on' }));
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-13 09:00:00\n');
  await lib.start([`-use=${id}`]);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const tabs = ui.locator('[role=tab]');
  const field = ui.getByRole('combobox', { name: 'Address' });
  const options = ui.getByRole('option');
  const go = () => regions();
  const box = (sel) => w.shell().evaluate((sel) => {
    const r = document.querySelector(sel).getBoundingClientRect();
    return { x: r.x, y: r.y, w: r.width, h: r.height, right: r.right, bottom: r.bottom };
  }, sel);
  const drawn = () => tabs.evaluateAll((els) => els.map((el) => ({ id: Number(el.dataset.tab), selected: el.getAttribute('aria-selected') === 'true' })));
  const agrees = async () => {
    const r = await go();
    const d = await drawn();
    return JSON.stringify(d.map((t) => t.id)) === JSON.stringify(r.tabs.map((t) => t.id)) && d.every((t) => t.selected === (t.id === r.active));
  };
  const tabCount = (n) => until(async () => (await go()).tabs.length === n && (await tabs.count()) === n && (await agrees()));
  const activeTab = async () => {
    const r = await go();
    return r.tabs.find((t) => t.id === r.active);
  };
  // Whether a press at the middle of locator, or at x, y, is the shell's, a drag's or a page's.
  const hitAt = async (x, y) => (await regions({ x, y })).hit;
  const hitOn = async (locator) => {
    const b = await locator.boundingBox();
    return hitAt(b.x + b.width / 2, b.y + b.height / 2);
  };
  // A person's pointer arrives in many moves; the window gives the shell the pointer on
  // the first move into its region, so the hover begins with the second.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  const shown = (locator) => locator.evaluate((el) => el.checkVisibility({ visibilityProperty: true }));
  // Whether tab id lies inside what scrolls its tabs, along the way it scrolls.
  const inView = (tabId, sel) => w.shell().evaluate(([tabId, sel]) => {
    const s = document.querySelector(sel).getBoundingClientRect();
    const t = document.querySelector(`[role=tab][data-tab="${tabId}"]`).getBoundingClientRect();
    return t.left >= s.left - 1 && t.right <= s.right + 1 && t.top >= s.top - 1 && t.bottom <= s.bottom + 1;
  }, [tabId, sel]);
  const scroller = (sel) => w.shell().evaluate((sel) => {
    const s = document.querySelector(sel);
    return { scrollWidth: s.scrollWidth, clientWidth: s.clientWidth, scrollLeft: s.scrollLeft, scrollHeight: s.scrollHeight, clientHeight: s.clientHeight, scrollTop: s.scrollTop };
  }, sel);
  const shot = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await w.shell().evaluate(() => document.fonts.ready);
      await settle(page);
      await page.screenshot({ path: path.join(out, `${id}-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  check('the window loads the minimal-js shell with its one tab', await tabCount(1) && (await go()).shell === id);

  // The last run's crash: one line under the toolbar until it is dismissed.
  const crash = ui.locator('.crash');
  await crash.waitFor();
  const toolbarBox = await box('.toolbar');
  const crashBox = await box('.crash');
  check('a crash last time shows as one line under the toolbar, across the window',
    Math.abs(crashBox.y - toolbarBox.bottom) < 1 && crashBox.x === 0 && crashBox.w === 1100 && crashBox.h <= 40 && (await crash.textContent()).includes('crash-'), JSON.stringify({ crashBox, toolbarBox }));
  check('a press on it is the shell\'s', (await hitOn(crash)) === 'shell');
  await shot('crash');
  await ui.getByRole('button', { name: 'Dismiss' }).click();
  check('Dismiss closes it', await until(async () => (await crash.count()) === 0));

  // No band: the bars and the page reach the window's edges. The tabs go down the side by
  // default.
  let r = await go();
  const sideBox = await box('.side');
  check('the tabs are down the side at first, as the pref says', sideBox.x === 0 && sideBox.w === 221 && (await ui.locator('.tabbar').count()) === 0);
  check('the shell asks for no band', r.frameWidth === 0, String(r.frameWidth));
  const toolbarFoot = (await box('.toolbar')).bottom;
  check('the page lies right of the list and under the toolbar, to the window\'s edges', await until(async () => {
    const p = (await go()).visible[0].rect;
    return p.x === sideBox.right && p.y === toolbarFoot && p.x + p.w === 1100 && p.y + p.h === 700;
  }), JSON.stringify((await go()).visible));
  const controls = r.window.Controls;
  const firstButton = await ui.getByRole('button', { name: 'Back' }).boundingBox();
  check('the toolbar starts after the window buttons', firstButton.x >= controls.x + controls.w, JSON.stringify({ firstButton, controls }));
  check('its empty space drags the window, its controls are the shell\'s',
    (await hitAt(controls.x + controls.w + 4, 19)) === 'drag' && (await hitOn(ui.getByRole('button', { name: 'Back' }))) === 'shell' && (await hitOn(field)) === 'shell');

  // Tabs on top.
  await fire('menu', 'view.tabsTop');
  await ui.locator('.tabbar').waitFor();
  check('Tabs on Top puts them in a bar and takes the list away', await until(async () => (await ui.locator('.side').count()) === 0 && (await tabCount(1))));
  const bar = await box('.tabbar');
  const toolbar = await box('.toolbar');
  check('the tab bar is 34px and the toolbar 38px, each with a hairline under it', bar.h === 35 && toolbar.h === 39 && toolbar.y === bar.bottom, JSON.stringify({ bar, toolbar }));
  check('the page lies under both, to the window\'s edges', await until(async () => {
    const v = (await go()).visible[0].rect;
    return v.x === 0 && v.y === toolbar.bottom && v.w === 1100 && v.y + v.h === 700;
  }), JSON.stringify((await go()).visible));
  const tabsBox = await box('.tabs');
  check('the tabs start after the window buttons', tabsBox.x >= controls.x + controls.w, JSON.stringify({ tabsBox, controls }));
  check('the space around the window buttons and after the tabs drags; the tabs are the shell\'s',
    (await hitAt(controls.x + controls.w + 2, 10)) === 'drag' && (await hitAt(1080, 17)) === 'drag' && (await hitOn(tabs.first())) === 'shell');
  check('the one tab shows its close button', await shown(tabs.first().locator('.x')));

  // New Tab: after the last, active, with the keyboard in the address.
  const newTab = ui.getByRole('button', { name: 'New Tab', exact: true });
  await newTab.click();
  check('New Tab adds a tab at the end and makes it active', await tabCount(2) && (await drawn())[1].selected);
  check('and puts the keyboard in the address', await until(() => field.evaluate((el) => el === document.activeElement && document.hasFocus())));

  // The address: a visit, the loading mark while it loads, Stop, then Reload.
  const loading = w.shell().waitForFunction(() => document.querySelector('.tab.active .loading') && document.querySelector('[aria-label=Stop]'), null, { polling: 10, timeout: 3000 }).then(() => true, () => false);
  await page.keyboard.type('example.com/news/today');
  await page.keyboard.press('Enter');
  check('while the page loads its tab has the loading mark, and Reload is Stop', await loading);
  check('once loaded the mark goes and Stop is Reload again', await until(async () => (await activeTab()).url === 'https://example.com/news/today' && !(await activeTab()).loading &&
    (await ui.locator('.tab .loading').count()) === 0 && (await ui.getByRole('button', { name: 'Reload' }).isEnabled())));
  check('the address shows the active tab\'s address', (await field.inputValue()) === 'https://example.com/news/today', await field.inputValue());
  await tabs.first().click();
  check('the address follows another tab made active', await until(async () => (await field.inputValue()) === '' && (await go()).active === (await go()).tabs[0].id));
  await tabs.nth(1).click();
  check('and back', await until(async () => (await field.inputValue()) === 'https://example.com/news/today'));

  // focusAddressBar: the keyboard in the field with all of it selected.
  await fire('key', 'focusAddressBar', { tab: (await go()).active });
  check('focusAddressBar selects the whole address', await until(() => field.evaluate((el) => el === document.activeElement && el.selectionStart === 0 && el.selectionEnd === el.value.length && el.value.length > 0)));

  // The close button: always on the active tab, on hover on the others.
  await page.mouse.move(600, 400);
  await settle(page);
  check('the active tab shows its close button, an inactive one doesn\'t', (await shown(tabs.nth(1).locator('.x'))) && !(await shown(tabs.first().locator('.x'))));
  await hover(tabs.first());
  check('a hovered tab shows its close button', await until(() => shown(tabs.first().locator('.x'))));

  // The active tab joins the toolbar: it covers the bar's hairline, in the toolbar's tone.
  const joined = await w.shell().evaluate(() => {
    const t = document.querySelector('.tab.active');
    const bar = document.querySelector('.tabbar').getBoundingClientRect();
    return t.getBoundingClientRect().bottom === bar.bottom && getComputedStyle(t).backgroundColor === getComputedStyle(document.querySelector('.toolbar')).backgroundColor;
  });
  check('the active tab joins the toolbar', joined);

  // The hovered tab's close button closes it.
  const empty = (await go()).tabs[0].id;
  await tabs.first().getByRole('button', { name: 'Close New Tab' }).click();
  check('a hovered tab\'s close button closes it', await tabCount(1) && !(await go()).tabs.some((t) => t.id === empty));

  // Four tabs, as the mock-up has them: Example Domain, Mail · Work in a workspace,
  // Project B on GitHub, Weather.
  await fire('menu', 'workspaces:0a1b2c3d');
  await tabCount(2);
  const inWork = (await go()).active;
  await fire('navigate', 'https://mail.test/', { tab: inWork });
  await fire('menu', 'tab.select1');
  for (const url of ['https://github.test/', 'https://weather.test/']) {
    const had = (await go()).tabs.length;
    await fire('menu', 'tab.new');
    await tabCount(had + 1);
    await fire('navigate', url, { tab: (await go()).active });
  }
  await until(async () => (await go()).tabs.every((t) => t.url !== '' && !t.loading));
  r = await go();
  for (const [i, t] of r.tabs.entries()) await fire('title', names[i], { tab: t.id });
  await until(async () => (await go()).tabs.every((t, i) => t.title === names[i]));
  const dot = ui.locator(`[role=tab][data-tab="${inWork}"] .dot`);
  check('a tab in a workspace has a small square in its colour', (await dot.count()) === 1 && (await dot.evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(217, 119, 6)' && (await dot.getAttribute('aria-label')) === 'In Work');
  check('a tab in the default workspace has none', (await tabs.first().locator('.dot').count()) === 0);
  const fourWidths = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('with few tabs each is 200px wide', fourWidths.every((x) => x === 200), JSON.stringify(fourWidths));
  const first = r.tabs[0].id;
  await tabs.first().click();
  await until(async () => (await go()).active === first);
  await page.mouse.move(600, 400);
  await shot('top-4');

  // The active tab's close button closes it, and Go's next active tab is marked.
  await fire('menu', 'tab.new');
  await tabCount(5);
  const closing = (await go()).active;
  await ui.locator(`[role=tab][data-tab="${closing}"]`).getByRole('button', { name: 'Close New Tab' }).click();
  check('Close on the active tab closes it, and the shell marks the tab Go makes active',
    await tabCount(4) && !(await go()).tabs.some((t) => t.id === closing) && (await drawn()).find((t) => t.selected)?.id === (await go()).active);

  // Suggestions, from the pages visited.
  await tabs.first().click();
  await until(async () => (await go()).active === first);
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  check('typing lists suggestions', await until(async () => (await options.count()) >= 1), String(await options.count()));
  const fieldBox = await ui.locator('.address').boundingBox();
  const listBox = await ui.locator('#suggestions').boundingBox();
  check('in a panel under the field, as wide as it', Math.abs(listBox.y - (fieldBox.y + fieldBox.height)) <= 1 && Math.abs(listBox.width - fieldBox.width) <= 2, JSON.stringify({ fieldBox, listBox }));
  check('a press on them is the shell\'s, over the page', listBox.y + listBox.height > (await go()).visible[0].rect.y && await until(async () => (await hitOn(options.first())) === 'shell'));
  await shot('suggestions');
  await page.keyboard.press('ArrowDown');
  check('Down chooses one, and the field shows it', (await ui.locator('[role=option][aria-selected=true]').count()) === 1 && (await field.inputValue()).startsWith('https://example'), await field.inputValue());
  await page.keyboard.press('ArrowUp');
  check('Up from the first comes back to what was typed', (await field.inputValue()) === 'exam', await field.inputValue());
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  check('Enter goes to the one chosen', await until(async () => (await activeTab()).url === 'https://example.com/news/today' && (await options.count()) === 0));
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  await until(async () => (await options.count()) >= 1);
  await page.keyboard.press('Escape');
  check('Escape puts the address back and gives the page the keyboard',
    await until(async () => (await field.inputValue()) === 'https://example.com/news/today' && (await options.count()) === 0 && (await go()).focused === (await go()).active),
    JSON.stringify({ value: await field.inputValue(), focused: (await go()).focused }));

  // The extensions, from the mock store: a package install asks in the shell first.
  const store = (kind, value) => fetch(`${lib.base}/mock/extensions/fire`, { method: 'POST', body: new URLSearchParams({ kind, extension: '', value }) });
  const installPackage = async () => {
    await store('choose', packaged);
    await fetch(`${lib.base}/settings/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'extensionInstallPackage' }) });
  };
  const prompt = ui.locator('.pbx-prompt');
  await installPackage();
  check('an install asks in the shell, with what it can do', await until(async () => (await prompt.count()) === 1 && (await prompt.textContent()).includes(`Add "${extensionName}"?`)));
  check('its answer is the last button, .primary', (await prompt.locator('button').last().getAttribute('class')).includes('primary') && (await prompt.locator('.primary').textContent()) === 'Add extension');
  check('the prompt makes the whole window the shell\'s', await until(async () => (await hitAt(550, 600)) === 'shell'));
  await shot('prompt');
  await page.keyboard.press('Escape');
  check('Escape answers no, and the prompt goes', await until(async () => (await prompt.count()) === 0 && (await hitAt(550, 600)) === 'page'));
  await installPackage();
  await prompt.waitFor();
  await prompt.locator('.primary').click();
  check('Add extension installs it', await until(async () => (await prompt.count()) === 0));
  const extensionsButton = ui.getByRole('button', { name: 'Extensions', exact: true });
  const menu = ui.locator('.pbx-menu');
  await extensionsButton.click();
  check('Extensions opens its menu, listing the extension', await until(async () => (await menu.count()) === 1 && (await menu.textContent()).includes(extensionName)));
  check('the menu is the shell\'s, over the page', await until(async () => (await hitOn(menu)) === 'shell'));
  await shot('extensions-menu');
  await menu.getByRole('button', { name: `Pin ${extensionName}` }).click();
  const action = ui.locator(`.pbx-action[data-extension="${extension}"]`);
  check('Pin puts its button on the toolbar', await until(async () => (await action.count()) === 1));
  check('and the menu says Unpin', await until(async () => (await menu.getByRole('button', { name: `Unpin ${extensionName}` }).count()) === 1));
  await page.keyboard.press('Escape');
  check('Escape closes the menu', await until(async () => (await menu.count()) === 0));
  await extensionsButton.click();
  await menu.waitFor();
  check('the open menu shows its button pressed', (await extensionsButton.getAttribute('class')).includes('pbx-open'));
  await extensionsButton.click();
  check('and its button closes it', await until(async () => (await menu.count()) === 0 && !(await extensionsButton.getAttribute('class')).includes('pbx-open')));
  const popup = async () => (await (await fetch(`${lib.base}/mock/extensions`)).json()).popup;
  await action.click();
  check('its button opens its popup, and shows it open', await until(async () => (await popup()) !== null && (await action.getAttribute('class')).includes('pbx-open')));
  await tabs.first().click();
  check('a press elsewhere in the shell closes the popup', await until(async () => (await popup()) === null && !(await action.getAttribute('class')).includes('pbx-open')));
  await extensionsButton.click();
  await menu.getByRole('button', { name: `Unpin ${extensionName}` }).click();
  check('Unpin takes its button away', await until(async () => (await action.count()) === 0));
  await menu.getByRole('button', { name: 'Manage extensions' }).click();
  check('Manage extensions opens settings and closes the menu', await until(async () => (await go()).settingsOpen === true && (await menu.count()) === 0));
  await page.frameLocator('iframe.mock-settings').getByRole('button', { name: 'Close settings' }).click();
  await until(async () => (await go()).settingsOpen === false);

  // Thirty tabs: each shrinks to 96px, then the bar scrolls; the active one is in view, and
  // New Tab and the drag space stay.
  r = await go();
  for (let i = r.tabs.length; i < 30; i++) await fire('open', `https://site${i}.test/`, { tab: (await go()).active });
  check('thirty tabs', await tabCount(30));
  await until(async () => (await go()).tabs.every((t) => t.url !== '' && !t.loading));
  r = await go();
  for (const [i, t] of r.tabs.entries()) await fire('title', names[i], { tab: t.id });
  await until(async () => (await go()).tabs.every((t, i) => t.title === names[i]));
  const widths = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('each tab shrinks to 96px and no less', widths.every((x) => x === 96), JSON.stringify(widths));
  let s = await scroller('.tabs');
  check('the bar scrolls sideways', s.scrollWidth > s.clientWidth, JSON.stringify(s));
  const last = (await go()).active;
  check('the active tab, the last opened, is in view', await until(() => inView(last, '.tabs')));
  const tabsNow = await box('.tabs');
  const newBox = await newTab.boundingBox();
  const dragBox = await box('.drag');
  check('New Tab stays right after the tabs, in the window', newBox.x >= tabsNow.right - 1 && newBox.x + newBox.width <= 1100, JSON.stringify({ tabsNow, newBox }));
  check('and the drag space after it, at least 48px, drags', dragBox.w >= 48 && dragBox.right <= 1100 && (await hitAt(dragBox.x + dragBox.w / 2, 17)) === 'drag', JSON.stringify(dragBox));
  // The mock-up's many-tabs picture has the eighteenth active.
  const eighteenth = r.tabs[17].id;
  await fire('menu', 'tab.select1');
  check('making the first tab active scrolls it into view', await until(async () => (await go()).active === r.tabs[0].id && (await inView(r.tabs[0].id, '.tabs')) && (await scroller('.tabs')).scrollLeft === 0));
  const before = (await scroller('.tabs')).scrollLeft;
  const strip = await box('.tabs');
  await page.mouse.move(strip.x + 100, strip.y + 17);
  await page.mouse.wheel(0, 300);
  check('the wheel scrolls the bar sideways', await until(async () => (await scroller('.tabs')).scrollLeft > before), JSON.stringify(await scroller('.tabs')));
  await ui.locator(`[role=tab][data-tab="${eighteenth}"]`).click();
  check('a tab clicked is made active and kept in view', await until(async () => (await go()).active === eighteenth && (await inView(eighteenth, '.tabs'))));
  await page.mouse.move(600, 400);
  await shot('top-30');

  // Down the side with thirty: New Tab at the top, the list scrolling, the active one in
  // view.
  await fire('menu', 'view.tabsSide');
  await ui.locator('.side').waitFor();
  check('Tabs on Side puts them in a list and takes the bar away', await until(async () => (await ui.locator('.tabbar').count()) === 0 && (await tabCount(30))));
  check('the active tab is in view in the list', await until(() => inView(eighteenth, '.list')));
  s = await scroller('.list');
  check('the list scrolls', s.scrollHeight > s.clientHeight, JSON.stringify(s));
  const sideNew = await ui.locator('.side .new').boundingBox();
  check('New Tab stays at its top', sideNew.y === (await box('.toolbar')).bottom && (await ui.locator('.side .new').textContent()) === 'New Tab');
  await page.mouse.move(600, 400);
  await shot('side-30');
  await fire('menu', 'tab.selectLast');
  check('the last tab made active is scrolled into view', await until(async () => (await go()).active === r.tabs[29].id && (await inView(r.tabs[29].id, '.list'))));
  await fire('menu', 'tab.select1');
  check('and the first', await until(async () => (await go()).active === r.tabs[0].id && (await inView(r.tabs[0].id, '.list')) && (await scroller('.list')).scrollTop === 0));
  await ui.locator('.side .new').click();
  check('New Tab in the list adds a tab and makes it active', await tabCount(31) && (await drawn())[30].selected && (await inView((await go()).active, '.list')));

  // Down the side with four.
  while ((await go()).tabs.length > 4) await fire('menu', 'tab.close');
  await tabCount(4);
  await fire('menu', 'tab.select2');
  await until(async () => (await go()).active === (await go()).tabs[1].id);
  await page.mouse.move(600, 400);
  await shot('side-4');

  // The tabs on top again, for the update.
  await fire('menu', 'view.tabsTop');
  await ui.locator('.tabbar').waitFor();
  check('no errors logged', w.errors.length === 0, w.errors.join(' | '));
  await page.close();

  // An update: the real updater against a release server, as tools/release.sh publishes,
  // with a bundle that says its version. Ready, it shows Relaunch to update in the toolbar.
  const releases = path.join(lib.profile, '..', 'releases');
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
  const installed = fakeApp(path.join(releases, 'installed'), '0.1.0');
  const body = fs.readFileSync(zip);
  const server = http.createServer((req, res) => {
    const { port } = server.address();
    if (req.url === '/releases.json') {
      res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-13T00:00:00Z', channels: { stable: '0.2.0' },
        releases: { '0.2.0': { assets: { 'darwin/arm64': { url: `http://127.0.0.1:${port}/release.zip`, sha256: crypto.createHash('sha256').update(body).digest('hex'), size: body.length } } } } }));
    } else {
      res.end(body);
    }
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  await lib.stop();
  await lib.start([`-use=${id}`, '-version=0.1.0', `-releases=http://127.0.0.1:${server.address().port}/releases.json`, `-app=${installed}`]);
  const w2 = await lib.openWindow(browser);
  const relaunch = w2.ui.getByRole('button', { name: 'Relaunch to update' });
  await relaunch.waitFor({ timeout: 10000 });
  const settings = await w2.ui.getByRole('button', { name: 'Settings' }).boundingBox();
  const relaunchBox = await relaunch.boundingBox();
  check('a release ready shows Relaunch to update in the toolbar, before Settings',
    Math.abs(relaunchBox.y + relaunchBox.height / 2 - (settings.y + settings.height / 2)) < 1 && relaunchBox.x < settings.x, JSON.stringify({ relaunchBox, settings }));
  check('it is the shell\'s', await until(async () => (await regions({ x: relaunchBox.x + 10, y: relaunchBox.y + 10 })).hit === 'shell'));
  for (const scheme of ['light', 'dark']) {
    await w2.page.emulateMedia({ colorScheme: scheme });
    await w2.shell().evaluate(() => document.fonts.ready);
    await settle(w2.page);
    await w2.page.screenshot({ path: path.join(out, `${id}-update-${scheme}.png`) });
  }
  await relaunch.click();
  check('Relaunch to update puts the release in place and ends the app', await until(async () => (await go()).quit === true) &&
    /<string>0\.2\.0</.test(fs.readFileSync(path.join(installed, 'Contents/Info.plist'), 'utf8')));
  check('no errors logged after the restart', w2.errors.length === 0, w2.errors.join(' | '));

  server.close();
  await browser.close();
  await lib.done();
})();
