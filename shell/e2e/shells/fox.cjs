// The fox shell's own tests: the tab strip that scrolls when the tabs overflow, the list of
// all tabs, the address field with its suggestions, the progress line, the workspace's
// line and name, and the menu button's commands. The check covers what every shell does.
// Run: node shell/e2e/shells/fox.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, out, regions, settle, until } = lib;

(async () => {
  // A workspace, so a tab can be opened in it as Tab, New Tab in Workspace does. The list
  // is from before Main, so Go puts Main above it.
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#5b7fd6' }] }));
  await lib.start(['-use=fox']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const tabs = ui.locator('[role=tab]');
  const field = ui.getByRole('combobox', { name: 'Address' });
  const options = ui.getByRole('option');
  const list = ui.locator('#suggestions');
  const go = async () => regions();
  // The shell draws what Go has: a tab for each, the active one selected.
  const drawn = () => tabs.evaluateAll((els) => els.map((el) => ({ id: Number(el.dataset.tab), selected: el.getAttribute('aria-selected') === 'true' })));
  const agrees = async () => {
    const r = await go();
    const d = await drawn();
    return JSON.stringify(d.map((t) => t.id)) === JSON.stringify(r.tabs.map((t) => t.id)) && d.every((t) => t.selected === (t.id === r.active));
  };
  const tabCount = (n) => until(async () => (await go()).tabs.length === n && (await tabs.count()) === n && (await agrees()));
  // A person's pointer arrives in many moves; the window gives the shell the pointer on
  // the first move into its region, so the hover begins with the second.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  const activeURL = async () => {
    const r = await go();
    return r.tabs.find((t) => t.id === r.active).url;
  };
  // Whether a press in the middle of locator reaches the shell.
  const pressReachesShell = async (locator) => {
    const b = await locator.boundingBox();
    return until(async () => (await regions({ x: b.x + b.width / 2, y: b.y + b.height / 2 })).hit === 'shell');
  };
  const strip = () => w.shell().evaluate(() => {
    const s = document.querySelector('.tabs');
    return { scrollWidth: s.scrollWidth, clientWidth: s.clientWidth, scrollLeft: s.scrollLeft };
  });
  const inView = (id) => w.shell().evaluate((id) => {
    const s = document.querySelector('.tabs').getBoundingClientRect();
    const t = document.querySelector(`[role=tab][data-tab="${id}"]`).getBoundingClientRect();
    return t.left >= s.left - 1 && t.right <= s.right + 1;
  }, id);
  // Light and dark, from the system's setting, which the shell follows by default.
  const shots = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(out, `fox-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };
  const visit = async (text, url) => {
    await field.click();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type(text);
    await page.keyboard.press('Enter');
    return until(async () => (await activeURL()) === url && (await field.inputValue()) === url && !(await go()).tabs.find((t) => t.url === url).loading);
  };

  check('the window loads the fox shell with its one tab', await tabCount(1) && (await go()).shell === 'fox');

  // The page under both bars, inside the frame.
  let r = await go();
  const barsFoot = await w.shell().evaluate(() => document.querySelector('.nav').getBoundingClientRect().bottom);
  const stripFoot = await w.shell().evaluate(() => document.querySelector('.strip').getBoundingClientRect().bottom);
  const p = r.visible[0].rect;
  const f = r.frameWidth;
  check('the page lies below the tab strip and the navigation bar', stripFoot <= barsFoot && p.y >= barsFoot, JSON.stringify({ stripFoot, barsFoot, page: p }));
  check('and inside the frame', p.x >= f && p.x + p.w <= r.window.W - f && p.y + p.h <= r.window.H - f, JSON.stringify({ page: p, frame: f, window: r.window }));

  // The progress line and Stop while a page loads, then Reload once it has.
  await field.click();
  await page.keyboard.type('example.com/docs');
  const loading = w.shell().waitForFunction(() => document.querySelector('.progress.on') && document.querySelector('[aria-label=Stop]'), null, { polling: 10, timeout: 3000 }).then(() => true, () => false);
  await page.keyboard.press('Enter');
  check('while the page loads the progress line shows, and reload is Stop', await loading);
  check('once it has loaded the line goes, and Stop is Reload again',
    await until(async () => (await activeURL()) === 'https://example.com/docs' && (await ui.locator('.progress.on').count()) === 0 && (await ui.getByRole('button', { name: 'Reload' }).isEnabled())));
  check('the address field shows the active tab\'s address', (await field.inputValue()) === 'https://example.com/docs', await field.inputValue());

  // New Tab: after the last tab, and active.
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  r = await go();
  check('a new tab appears after the last tab and becomes active',
    await tabCount(2) && (await drawn())[1].id === (await go()).active && (await drawn())[1].selected, JSON.stringify(await drawn()));
  check('a new tab puts the keyboard in the address field', await until(() => field.evaluate((el) => el === document.activeElement)));
  check('a visit there shows in the field', await visit('example.org', 'https://example.org'), JSON.stringify({ tabs: (await go()).tabs, value: await field.inputValue() }));
  const [first, second] = (await go()).tabs;
  await tabs.first().click();
  check('the field follows a switch to another tab', await until(async () => (await field.inputValue()) === first.url), await field.inputValue());
  await tabs.nth(1).click();
  check('and back', await until(async () => (await field.inputValue()) === second.url), await field.inputValue());

  // The close control: always on the active tab, on hover on the others.
  const closeOf = (i) => tabs.nth(i).locator('.close');
  const shown = (loc) => loc.evaluate((el) => el.checkVisibility({ visibilityProperty: true }));
  await page.mouse.move(600, 400);
  await settle(page);
  check('the active tab shows its close control, an inactive one doesn\'t', (await shown(closeOf(1))) && !(await shown(closeOf(0))));
  await hover(tabs.first());
  check('a hovered inactive tab shows its close control', await until(() => shown(closeOf(0))));
  await shots('few');

  // Middle-click closes.
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  await tabCount(3);
  const third = (await go()).active;
  await tabs.nth(2).click({ button: 'middle' });
  check('a middle-click closes a tab', await tabCount(2) && !(await go()).tabs.some((t) => t.id === third), JSON.stringify(await drawn()));

  // Closing the active tab: the shell marks whichever tab Go makes active.
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  await tabCount(3);
  await tabs.nth(1).click();
  await until(async () => (await go()).active === second.id);
  await tabs.nth(1).locator('.close').click();
  check('closing the active tab makes another active as Go decides, and the shell marks it',
    await tabCount(2) && !(await go()).tabs.some((t) => t.id === second.id) && (await drawn()).find((t) => t.selected)?.id === (await go()).active, JSON.stringify({ drawn: await drawn(), go: (await go()).active }));

  // Suggestions, from the pages visited.
  await tabs.first().click();
  await until(async () => (await go()).active === first.id);
  await visit('example.org/more', 'https://example.org/more');
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('exam');
  check('typing lists suggestions', await until(async () => (await options.count()) >= 2), String(await options.count()));
  const fieldBox = await ui.locator('.field').boundingBox();
  const listBox = await list.boundingBox();
  check('they open attached under the field, as wide as it',
    Math.abs(listBox.y - (fieldBox.y + fieldBox.height)) <= 1 && Math.abs(listBox.width - fieldBox.width) <= 2, JSON.stringify({ fieldBox, listBox }));
  check('a press on them reaches the shell, over the page', listBox.y + listBox.height > (await go()).visible[0].rect.y && await pressReachesShell(options.last()));
  await shots('suggestions');
  await page.keyboard.press('ArrowDown');
  check('Down chooses one, and the field shows it', (await ui.locator('li.active').count()) === 1 && (await field.inputValue()).startsWith('https://example'), await field.inputValue());
  await page.keyboard.press('ArrowUp');
  check('Up from the first comes back to what was typed', (await field.inputValue()) === 'exam', await field.inputValue());
  const target = (await options.last().locator('.url').innerText()) || (await options.last().innerText());
  await options.last().click();
  check('a chosen suggestion navigates', await until(async () => (await activeURL()) === target && (await list.count()) === 0), JSON.stringify({ target, now: await activeURL() }));

  // Escape puts the address back and gives the page the keyboard.
  const before = await activeURL();
  await field.click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type('qqq');
  await settle(page);
  check('what matches no visit lists nothing', (await options.count()) === 0);
  await page.keyboard.press('Escape');
  check('Escape restores the address and gives the keyboard to the page',
    await until(async () => (await field.inputValue()) === before && (await go()).focused === (await go()).active), JSON.stringify({ value: await field.inputValue(), focused: (await go()).focused }));

  // A tab in a workspace: its line, and the workspace's name in the field. The tabs before
  // it are in Main.
  await fire('menu', 'workspaces:0a1b2c3d');
  await tabCount(3);
  const inWork = (await go()).active;
  const line = ui.locator(`[role=tab][data-tab="${inWork}"] .workspace`);
  check('a tab in a workspace has a line in its colour', (await line.count()) === 1 && (await line.evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(91, 127, 214)');
  check('a tab in Main has Main\'s', (await tabs.first().locator('.workspace').evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(52, 112, 208)');
  check('the field shows the workspace\'s name', (await ui.locator('.field .workspace').innerText()).trim() === 'Work');
  await tabs.first().click();
  check('and Main\'s for a tab in Main', await until(async () => (await ui.locator('.field .workspace').innerText()).trim() === 'Main'));

  // Many tabs: each at least the minimum width, the strip scrolls with arrows, and the
  // active tab is scrolled into view.
  for (let i = 0; i < 20; i++) await fire('menu', 'tab.new');
  await tabCount(23);
  const widths = await tabs.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().width));
  check('with many tabs each keeps at least the minimum width', widths.every((x) => x >= 76), JSON.stringify(widths));
  let s = await strip();
  check('the strip overflows', s.scrollWidth > s.clientWidth, JSON.stringify(s));
  const left = ui.getByRole('button', { name: 'Scroll tabs left' });
  const right = ui.getByRole('button', { name: 'Scroll tabs right' });
  check('the arrows appear', (await left.isVisible()) && (await right.isVisible()));
  const last = (await go()).active;
  check('the new active tab, at the end, is scrolled into view', await until(async () => (await inView(last)) && (await right.isDisabled())), JSON.stringify({ strip: await strip(), last, active: (await go()).active }));
  const newTab = await ui.getByRole('button', { name: 'New Tab', exact: true }).boundingBox();
  const scrollerBox = await ui.locator('.tabs').boundingBox();
  check('New Tab stays at the strip\'s end', newTab.x >= scrollerBox.x + scrollerBox.width);
  await shots('many');
  const atEnd = s.scrollLeft;
  await left.click();
  check('the left arrow scrolls the strip', await until(async () => (await strip()).scrollLeft < atEnd), JSON.stringify(await strip()));
  await fire('menu', 'tab.select1');
  const firstId = (await go()).tabs[0].id;
  check('making a far tab active scrolls it into view', await until(async () => (await go()).active === firstId && (await inView(firstId)) && (await strip()).scrollLeft === 0), JSON.stringify(await strip()));
  await right.click();
  check('the right arrow scrolls the strip', await until(async () => (await strip()).scrollLeft > 0), JSON.stringify(await strip()));

  // The list of all tabs.
  const allButton = ui.getByRole('button', { name: 'List all tabs' });
  await allButton.click();
  const items = ui.getByRole('group', { name: 'All tabs' }).getByRole('button');
  r = await go();
  check('"List all tabs" lists every tab', (await items.count()) === r.tabs.length, String(await items.count()));
  check('with the active one marked', (await items.nth(0).getAttribute('aria-current')) === 'true' && (await ui.locator('.all [aria-current=true]').count()) === 1);
  check('a press on the list reaches the shell, not the page', await pressReachesShell(items.nth(3)));
  await shots('all-tabs');
  const chosen = r.tabs[15].id;
  await items.nth(15).click();
  check('choosing one makes it active and scrolls it into view', await until(async () => (await go()).active === chosen && (await inView(chosen))));
  check('and closes the list', (await items.count()) === 0);
  await allButton.click();
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await items.count()) === 0));

  // The menu button: only real actions; Settings opens the settings page.
  const menuButton = ui.getByRole('button', { name: 'Menu', exact: true });
  await menuButton.click();
  const rows = ui.getByRole('menuitem');
  const actions = await rows.evaluateAll((els) => els.map((el) => el.dataset.action));
  const known = ['tab.new', 'window.new', 'view.zoomIn', 'view.zoomOut', 'view.zoomReset', 'view.fullScreen', 'app.settings'];
  const real = new Set(fs.readFileSync(path.join(lib.root, 'core/app/actions.go'), 'utf8').match(/add\("([a-zA-Z.]+)"/g).map((m) => m.slice(5, -1)));
  check('the menu lists its commands, each an action Go has', JSON.stringify(actions) === JSON.stringify(known) && actions.every((a) => real.has(a)), JSON.stringify(actions));
  check('a press on the menu reaches the shell, not the page', await pressReachesShell(rows.last()));
  await shots('menu');
  await rows.filter({ hasText: 'New Tab' }).click();
  check('New Tab in the menu opens a tab', await tabCount(24));
  await menuButton.click();
  await rows.filter({ hasText: 'Settings' }).click();
  check('Settings opens the settings page', await until(async () => (await go()).settingsOpen));

  check('no errors logged', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
