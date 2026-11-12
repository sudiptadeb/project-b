// End-to-end test of the first app on the mock engine, in a real browser.
// Run: node shell/e2e/first-app.cjs   (from the repo root; Playwright is preinstalled)

const { spawn, execFileSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
const port = 8799;
const base = `http://127.0.0.1:${port}`;

fs.mkdirSync(out, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-e2e-'));
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

let server;
async function startServer(extra = []) {
  // A server left over from another run would answer in ours' place.
  if (await fetch(base + '/mock/regions').then(() => true, () => false)) {
    throw new Error(`port ${port} already has a server: stop it and run again`);
  }
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, '-block=blocked.test', ...extra], { stdio: 'ignore' });
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(base + '/mock/regions');
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 50));
    }
  }
  throw new Error('server did not start');
}
async function stopServer() {
  const gone = new Promise((r) => server.once('exit', r));
  server.kill('SIGTERM');
  await gone;
}

let failed = 0;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${ok || !detail ? '' : ' -> ' + detail}`);
  if (!ok) failed++;
}
const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
const until = async (test, ms = 5000) => {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await test()) return true;
    await new Promise((r) => setTimeout(r, 50));
  }
  return false;
};
const settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

(async () => {
  await startServer();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 }, screen: { width: 1920, height: 1080 } });
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));

  // The page is the OS window: the shell is in a frame of its own, over the mock page's,
  // both at the window's coordinates. Locators reach the shell through its frame, and
  // survive it loading again; keyboard, mouse and screenshots are the window's.
  await page.goto(base + '/');
  const ui = page.frameLocator('iframe.shell');
  const shellFrame = () => page.frame({ url: /\/shell\/$/ });
  // A person's pointer arrives in many moves. The window gives the shell the pointer on
  // the first move into its region, so the shell's hover begins with the next; Playwright's
  // hover is one move, so this makes two.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  const bar =ui.getByRole('combobox', { name: 'Address' });
  const list = ui.locator('#suggestions');
  const options = ui.getByRole('option');
  const mock = page.frameLocator('iframe.mock-page');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(page);
  await page.screenshot({ path: path.join(out, '1-empty.png') });

  const back = ui.getByRole('button', { name: 'Back' });
  const forward = ui.getByRole('button', { name: 'Forward' });
  const reload = ui.getByRole('button', { name: 'Reload' });
  check('opens with nothing to go back to or reload', (await back.isDisabled()) && (await forward.isDisabled()) && (await reload.isDisabled()));

  let r = await regions();
  check('opens empty with the top bar and the sidebar as shell', r.regions.length === 2 && (await bar.inputValue()) === '', JSON.stringify(r));
  check('page sits right of the sidebar, below the bar, inside the frame', r.bounds.x === 224 && r.bounds.y === 48 && r.bounds.w === 1100 - 224 - 4 && r.bounds.y + r.bounds.h === 700 - 4, JSON.stringify(r));
  // The dev window places the mock page where the layout puts it, as the real window does.
  const placed = async () => {
    const want = (await regions()).bounds;
    const box = await page.locator('iframe.mock-page').boundingBox();
    return !!box && box.x === want.x && box.y === want.y && box.width === want.w && box.height === want.h;
  };
  check('the dev window puts the page where the layout says', await until(placed), JSON.stringify(await page.locator('iframe.mock-page').boundingBox()));
  check('opens with the address bar focused, as a new tab does', await bar.evaluate((el) => el === document.activeElement));
  check('no update bubble when nothing is happening', (await ui.locator('.update').count()) === 0);
  const pageBounds = JSON.stringify(r.bounds);

  // Waits for the commit. The URL shows as soon as loading starts, and a new navigation
  // before the commit would correctly replace this one.
  const loaded = async (expected) => {
    await mock.getByText(expected, { exact: true }).waitFor();
    await mock.getByText('Loaded', { exact: true }).waitFor();
    await shellFrame().waitForFunction((v) => document.querySelector('input').value === v, expected);
  };
  const visit = async (text, expected) => {
    await bar.fill(text);
    await bar.press('Enter');
    await loaded(expected);
  };
  await visit('example.com/docs', 'https://example.com/docs');
  await visit('https://go.dev/learn', 'https://go.dev/learn');
  await visit('https://example.org/', 'https://example.org/');
  check('address bar shows the final URL', (await bar.inputValue()) === 'https://example.org/');
  check('the progress bar shows the page\'s real load progress', (await ui.locator('.progress').evaluate((el) => el.style.width)) === '100%');

  // Back, forward and reload.
  check('back is on and forward is off after new pages', (await back.isEnabled()) && (await forward.isDisabled()) && (await reload.isEnabled()));
  await back.click();
  await loaded('https://go.dev/learn');
  check('back returns to the previous page', await forward.isEnabled());
  await forward.click();
  await loaded('https://example.org/');
  check('forward returns to the next page', await forward.isDisabled());
  await reload.click();
  await mock.getByText('Loading', { exact: true }).waitFor();
  await loaded('https://example.org/');
  check('reload loads the page again', (await bar.inputValue()) === 'https://example.org/');
  await back.click();
  await loaded('https://go.dev/learn');
  await visit('https://example.org/', 'https://example.org/');
  await shellFrame().waitForFunction(() => document.querySelector('[aria-label=Forward]').disabled);
  check('a new page drops what was ahead', await forward.isDisabled());

  // Suggestions come from real history, over the page.
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await settle(page);
  await page.screenshot({ path: path.join(out, '2-suggestions.png') });
  check('suggestions come from visited pages', (await options.count()) === 2, String(await options.count()));
  check('titles arrive after the visit', /Mock example/.test(await options.first().innerText()));

  r = await regions();
  check('the box becomes a third shell rectangle', r.regions.length === 3, JSON.stringify(r.regions));
  const box = await list.boundingBox();
  const over = await regions(`?x=${box.x + 20}&y=${box.y + 10}`);
  check('the box extends over the page', box.y + box.height > r.bounds.y + 40, JSON.stringify({ box, bounds: r.bounds }));
  check('a click on the box goes to the shell', over.hit === 'shell');
  const below = await regions(`?x=${box.x + 20}&y=${box.y + box.height + 40}`);
  check('a click below the box goes to the page', below.hit === 'page');

  // Bar and list behave as one UI.
  await page.keyboard.press('ArrowDown');
  check('down moves into the list', (await bar.inputValue()).startsWith('https://example') && (await ui.locator('li.active').count()) === 1);
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  check('up from the first item returns to what was typed', (await bar.inputValue()) === 'exam' && (await ui.locator('li.active').count()) === 0, await bar.inputValue());
  check('focus never left the address bar', await bar.evaluate((el) => el === document.activeElement));
  await page.keyboard.press('Escape');
  await settle(page);
  r = await regions();
  check('escape closes the box and its rectangle goes', (await list.count()) === 0 && r.regions.length === 2, JSON.stringify(r.regions));

  // Clicking a suggestion.
  await bar.fill('');
  await page.keyboard.type('go.d');
  await options.first().click();
  await shellFrame().waitForFunction(() => document.querySelector('input').value === 'https://go.dev/learn');
  check('clicking a suggestion navigates', (await list.count()) === 0);

  // Clicking the page closes the box.
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await page.mouse.click(1050, 650);
  await settle(page);
  check('clicking the page closes the box', (await list.count()) === 0 && (await bar.inputValue()) === 'https://go.dev/learn');

  // The window sends each press where its regions say, with the mouse moved there first,
  // as a person's is: beside the suggestions the page takes it, on them the shell.
  const titleButton = await page.locator('iframe.mock-page').contentFrame().getByRole('button', { name: 'Title' }).boundingBox();
  await page.mouse.click(titleButton.x + titleButton.width / 2, titleButton.y + titleButton.height / 2);
  check('a press in the page area reaches the page', await until(async () => (await ui.locator('.row.tab.active').innerText()).includes('A new title')), await ui.locator('.row.tab.active').innerText());
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await settle(page);
  const optionBox = await options.last().boundingBox();
  check('the suggestions lie over the page', optionBox.y > (await regions()).bounds.y, JSON.stringify(optionBox));
  await page.mouse.click(optionBox.x + 40, optionBox.y + optionBox.height / 2);
  check('a press on them, over the page, reaches the shell', await until(async () => (await list.count()) === 0 && (await bar.inputValue()).startsWith('https://example')), await bar.inputValue());

  // Events from the page side. Shortcuts arrive from the engine, whichever page had focus.
  const shortcut = async (name) => {
    await mock.getByLabel('Key name').fill(name);
    await mock.getByRole('button', { name: 'Key', exact: true }).click();
  };
  await shortcut('focusAddressBar');
  await shellFrame().waitForFunction(() => document.activeElement === document.querySelector('input'));
  check('the shortcut focuses the address bar from the page', true);
  await page.keyboard.press('Escape');

  await mock.getByRole('button', { name: 'Fail' }).click();
  await ui.getByText('Failed', { exact: true }).waitFor();
  check('a failed navigation shows', true);
  await mock.getByRole('button', { name: 'Crash' }).click();
  await ui.getByText('Crashed', { exact: true }).waitFor();
  check('a crashed page shows', true);

  // Policy.
  await bar.fill('https://www.blocked.test/secret');
  await bar.press('Enter');
  await ui.getByText('Blocked', { exact: true }).waitFor();
  await bar.click();
  await bar.fill('');
  await page.keyboard.type('secret');
  await settle(page);
  check('a blocked URL stays out of history', (await options.count()) === 0);
  await bar.fill('');
  await page.keyboard.type('blank');
  await settle(page);
  check('the blank page a new tab holds stays out of history', (await options.count()) === 0);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');

  // The shell's own page never loads in a tab: a copy there would talk to Go as the
  // shell. Content's own internal pages still load when typed.
  const mockShows = async (state, url) => {
    for (let i = 0; i < 100; i++) {
      if ((await mock.locator('#url').textContent()) === url && (await mock.locator('#state').textContent()) === state) return true;
      await new Promise((r) => setTimeout(r, 30));
    }
    return false;
  };
  await bar.fill('chrome://gpu/');
  await bar.press('Enter');
  check('typing an internal page content serves, such as chrome://gpu/, is let through', await mockShows('Loaded', 'chrome://gpu/'));
  await bar.fill('chrome://shell/');
  await bar.press('Enter');
  check('typing chrome://shell/ is refused by the gate', await mockShows('Blocked', 'chrome://shell/'));
  await bar.fill('https://links.test/');
  await bar.press('Enter');
  await mockShows('Loaded', 'https://links.test/');
  await mock.locator('form[data-kind=navigate] input').fill('chrome://shell/');
  await mock.getByRole('button', { name: 'Navigate' }).click();
  check('a page linking to chrome://shell/ is refused', await mockShows('Blocked', 'chrome://shell/'));
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await ui.locator('.strip').waitFor();
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await settle(page);
  check('after those attempts the shell still takes clicks: the layout toggle works', (await ui.locator('.strip').count()) === 0);

  // The engine says where the window controls are; the shell leaves exactly that space.
  // In full screen there are none, so the first control moves into their place.
  const toggle = ui.getByRole('button', { name: /sidebar/ });
  const windowLeft = (await toggle.boundingBox()).x;
  check('the first control starts after the window controls the engine reported', windowLeft === 19 + 60 + 8, String(windowLeft));
  r = await regions('?x=74&y=16');
  check('the empty strip between the window buttons and the controls drags the window', r.drag.length === 5 && r.hit === 'drag', JSON.stringify(r.drag));
  check('the empty part of the sidebar is shell and never drags', (await regions('?x=100&y=600')).hit === 'shell');
  check('the frame\'s left, right and bottom edges never drag', (await regions('?x=2&y=300')).hit !== 'drag' && (await regions('?x=1098&y=300')).hit !== 'drag' && (await regions('?x=600&y=698')).hit !== 'drag');
  check('the control itself is shell, not drag', (await regions(`?x=${windowLeft + 12}&y=16`)).hit === 'shell');
  // The frame colour has one owner: Go gave it to the window and to the shell.
  const frame = await shellFrame().evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--frame').trim());
  check('the window and the shell paint the frame colour Go owns', frame === '#4f9e9e' && (await regions()).windowColor === frame, `${frame} ${(await regions()).windowColor}`);
  check('the window already had the frame colour when it went on screen', (await regions()).shownColor === frame, (await regions()).shownColor);

  // The shell is not a page: nothing scrolls, and a trackpad swipe must not rubber-band it.
  const rootStyle = await shellFrame().evaluate(() => { const s = getComputedStyle(document.documentElement); return [s.overflow, s.overscrollBehaviorY, document.scrollingElement.scrollHeight - window.innerHeight]; });
  check('the shell document neither scrolls nor bounces', rootStyle[0] === 'hidden' && rootStyle[1] === 'none' && rootStyle[2] === 0, JSON.stringify(rootStyle));

  // The engine also says how round the window's corners are; the frame's inner edge
  // follows, one frame width inside.
  const windowRadius = () => shellFrame().evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--window-radius').trim());
  check('the shell hears the window\'s corner radius from the engine', (await windowRadius()) === '26px', await windowRadius());
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await shellFrame().waitForFunction(() => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x < 20);
  check('in full screen the controls move into the buttons\' place', (await toggle.boundingBox()).x === 4 + 8, String((await toggle.boundingBox()).x));
  check('in full screen the corners are square', (await windowRadius()) === '0px', await windowRadius());
  check('in full screen nothing drags: the empty top row is shell', (await regions('?x=600&y=2')).hit === 'shell' && (await regions()).drag.length === 0, JSON.stringify((await regions()).drag));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await shellFrame().waitForFunction((x) => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x === x, windowLeft);
  check('leaving full screen gives the buttons their space back', true);
  check('and the corners their curve', (await windowRadius()) === '26px', await windowRadius());
  check('and the empty top row drags again', (await regions('?x=600&y=2')).hit === 'drag');

  // The window draws its frame band over the shell and the page, and its controls where
  // the engine reports them; full screen has none.
  const band = await page.locator('.band').evaluate((el) => { const s = getComputedStyle(el); return [s.borderTopColor, s.borderLeftWidth, s.borderRadius]; });
  check('the window draws the frame band in the frame colour, the frame width wide', JSON.stringify(band) === JSON.stringify(['rgb(79, 158, 158)', '4px', '26px']), JSON.stringify(band));
  const controls = page.locator('.controls');
  const controlsBox = await controls.boundingBox();
  check('and its controls where the engine reports them', JSON.stringify(controlsBox) === JSON.stringify({ x: 19, y: 19, width: 60, height: 14 }), JSON.stringify(controlsBox));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  check('in full screen the window has no controls', await until(async () => !(await controls.isVisible())));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  check('and out of it they are back', await until(() => controls.isVisible()));

  // The window's size reaches Go as the real window's does, and the shell's layout follows.
  const sized = async (w, h) => {
    const r = await regions();
    return r.window.W === w && r.window.H === h && r.bounds.w === w - 224 - 4 && r.bounds.y + r.bounds.h === h - 4 && (await placed());
  };
  await page.setViewportSize({ width: 900, height: 600 });
  check('resizing the window reaches Go, and the page follows the shell\'s layout', await until(() => sized(900, 600)), JSON.stringify(await regions()));
  await page.setViewportSize({ width: 1100, height: 700 });
  check('and back', await until(() => sized(1100, 700)), JSON.stringify(await regions()));

  // Tabs.
  const rows = ui.getByRole('tab');
  const activeRow = ui.locator('.row.tab.active');
  const rowTitles = () => rows.evaluateAll((els) => els.map((el) => el.querySelector('.title').textContent));
  check('one tab to start with, active', (await rows.count()) === 1 && (await activeRow.count()) === 1);
  await visit('https://example.org/', 'https://example.org/');
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('New Tab adds a tab and makes it active', (await activeRow.innerText()).startsWith('New Tab'), await activeRow.innerText());
  check('a new tab has an empty, focused address bar', (await bar.inputValue()) === '' && (await bar.evaluate((el) => el === document.activeElement)));
  await mock.getByText('tab 2', { exact: true }).waitFor();
  // A new tab holds the blank page, so its page area is a page in the page's own
  // colours, never the frame colour showing through. It still reads as a new tab.
  let blank = false;
  for (let i = 0; i < 100 && !blank; i++) {
    blank = (await mock.locator('#state').textContent()) === 'Loaded' && (await mock.locator('#url').textContent()) === 'about:blank';
    if (!blank) await new Promise((r) => setTimeout(r, 30));
  }
  const pageColour = await mock.locator('body').evaluate((b) => getComputedStyle(b).backgroundColor);
  check("a new tab holds the blank page, in the page's colour, not the frame colour", blank && pageColour !== 'rgb(79, 158, 158)', pageColour);
  check('and reads as a new tab: no address, titled New Tab', (await bar.inputValue()) === '' && (await activeRow.innerText()).startsWith('New Tab'), await activeRow.innerText());
  r = await regions();
  check('the first tab was told it is hidden', JSON.stringify(r.hidden) === '[1]', JSON.stringify(r.hidden));
  check('the layout lists the new tab as the one visible tab', r.visible.length === 1 && r.visible[0].tab === 2 && JSON.stringify(r.visible[0].rect) === pageBounds, JSON.stringify(r.visible));
  check('back and reload follow the new, empty tab', (await back.isDisabled()) && (await reload.isDisabled()));
  await visit('https://example.net/', 'https://example.net/');
  check('each tab keeps its own title', JSON.stringify(await rowTitles()) === JSON.stringify(['Mock example.org', 'Mock example.net']), JSON.stringify(await rowTitles()));
  await rows.first().click();
  await shellFrame().waitForFunction((v) => document.querySelector('input').value === v, 'https://example.org/');
  check('clicking a row switches tabs and the address bar follows', (await rows.first().getAttribute('aria-selected')) === 'true');
  check('back follows the tab that has history', await back.isEnabled());
  await mock.getByText('tab 1', { exact: true }).waitFor();
  check('the page view shows the active tab', true);

  // Favicons come from the page; a tab without one shows none.
  check('a tab with no favicon shows no icon', (await ui.locator('[role=tab] img').count()) === 0);
  await mock.getByRole('button', { name: 'Favicon' }).click();
  await ui.locator('[role=tab][aria-selected=true] img').waitFor();
  check('the favicon the page gives shows on its row only', (await ui.locator('[role=tab] img').count()) === 1);
  await visit('https://example.org/', 'https://example.org/');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab] img').length === 0);
  check('a new page drops the old favicon', true);

  // The page opens a tab.
  await mock.getByRole('button', { name: 'Open in new tab' }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  await loaded('https://opened.test/');
  check('a tab the page opens appears and is active', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());

  await shortcut('tab1');
  await shellFrame().waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('Cmd+1 picks the first tab', true);
  await shortcut('nextTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]')[1].getAttribute('aria-selected') === 'true');
  check('Shift+Cmd+] picks the next tab', true);
  await shortcut('closeTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('Cmd+W closes the active tab and its right neighbour takes over', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());
  await shortcut('newTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  check('Cmd+T opens a tab', (await bar.inputValue()) === '');

  // Close with the button on the row.
  await hover(rows.nth(1));
  await rows.nth(1).getByRole('button', { name: /^Close/ }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a row closes that tab', !(await rowTitles()).some((t) => t.includes('opened.test')), JSON.stringify(await rowTitles()));

  // The sidebar collapses and the page follows.
  await toggle.click();
  await shellFrame().waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 0);
  await settle(page);
  r = await regions();
  check('collapsing the sidebar gives its width to the page', r.bounds.x === 4 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '4-collapsed.png') });
  await toggle.click();
  await shellFrame().waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 220);
  await settle(page);
  r = await regions();
  check('expanding it takes the width back', r.bounds.x === 224, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '5-tabs.png') });

  // The same tabs on top. One switch, kept by the managers, drives both layouts.
  const strip = ui.locator('.strip');
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();
  await settle(page);
  check('switching puts the same tabs in a strip on top', (await rows.count()) === 2 && (await activeRow.count()) === 0 && (await ui.locator('.strip [aria-selected=true]').count()) === 1);
  check('the strip leaves the window controls their space', (await rows.first().boundingBox()).x === 19 + 60 + 8, String((await rows.first().boundingBox()).x));
  const joint = await shellFrame().evaluate(() => { const shape = document.querySelector('.strip .shape').getBoundingClientRect(); const strip = document.querySelector('.strip').getBoundingClientRect(); return [shape.bottom, strip.bottom]; });
  check('the active tab\'s shape runs under the bar, so no line shows between them', joint[0] > joint[1], JSON.stringify(joint));
  r = await regions();
  check('without a sidebar the page spans the width below two rows', r.bounds.x === 4 && r.bounds.y === 92 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  check('and the dev window moves the page there', await until(placed), JSON.stringify(await page.locator('iframe.mock-page').boundingBox()));
  check('the empty strip drags the window', (await regions('?x=900&y=16')).hit === 'drag');
  check('the bands beside the toolbar under the strip are shell, not drag', (await regions('?x=2&y=60')).hit === 'shell');
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  check('New Tab works from the strip', (await bar.inputValue()) === '');
  await rows.first().click();
  await shellFrame().waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('clicking a strip tab activates it', true);
  await hover(rows.nth(2));
  await rows.nth(2).getByRole('button', { name: /^Close/ }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a strip tab closes it', true);
  await page.screenshot({ path: path.join(out, '6-horizontal.png') });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await settle(page);
  check('switching back brings the sidebar and the page follows', (await rows.count()) === 2 && (await regions()).bounds.x === 224);
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();

  // Closing the last tab closes the window.
  await shortcut('closeTab');
  await shortcut('closeTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 0);
  check('closing the last tab closes the window', await until(async () => (await regions()).closed === true));

  // History survives a restart.
  await stopServer();
  await startServer();
  await page.goto(base + '/');
  errors.length = 0; // Stopping the server breaks the event stream, as it should.
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  check('history survives a restart', (await options.count()) === 3, String(await options.count()));
  check('titles survive a restart', /Mock example/.test(await options.first().innerText()));
  await page.keyboard.press('Escape');
  check('the tab layout survives a restart', (await ui.locator('.strip').count()) === 1);
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  check('and can be switched back', (await ui.locator('.strip').count()) === 0);

  // Phone width.
  const phone = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
  await phone.goto(base + '/');
  const phoneUi = phone.frameLocator('iframe.shell');
  const phoneBar = phoneUi.getByRole('combobox', { name: 'Address' });
  await phone.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await phoneBar.click();
  await phone.keyboard.type('exam');
  await phoneUi.getByRole('option').first().waitFor();
  await settle(phone);
  await phone.screenshot({ path: path.join(out, '3-phone.png') });
  const scroll = await phone.frame({ url: /\/shell\/$/ }).evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  check('no horizontal scroll on a phone', scroll);
  // The mock engine keeps one layout, the last any page sent. Left open, this page sends its
  // own again when the server restarts below, and now and then it lands after the other's.
  await phone.close();

  // Updates. The real manager runs against a release server here: a manifest and a zip of a
  // bundle, as tools/release.sh publishes, served slowly enough to watch the download.
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
    fs.writeFileSync(path.join(app, 'Contents/MacOS/project_b'), `#!/bin/sh\necho ${version} > "$(dirname "$0")/ran"\n`, { mode: 0o755 });
    return app;
  };
  const installed = path.join(releases, 'installed');
  const version = (app) => /CFBundleShortVersionString<\/key><string>([^<]+)/.exec(fs.readFileSync(path.join(app, 'Contents/Info.plist'), 'utf8'))[1];
  const zip = path.join(releases, 'Project-B-0.2.0-darwin-arm64.zip');
  execFileSync('ditto', ['-c', '-k', '--keepParent', fakeApp(path.join(releases, 'new'), '0.2.0'), zip]);
  const good = fs.readFileSync(zip);
  let served = good;
  const releasePort = 8798;
  const releaseServer = http.createServer((req, res) => {
    if (req.url === '/releases.json') {
      res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' },
        releases: { '0.2.0': { assets: { 'darwin/arm64': { url: `http://127.0.0.1:${releasePort}/asset.zip`, sha256: crypto.createHash('sha256').update(good).digest('hex'), size: good.length } } } } }));
    } else {
      // Half at once and the rest later, so the download is seen part-way.
      const body = served;
      res.writeHead(200, { 'Content-Length': body.length });
      res.write(body.subarray(0, body.length >> 1));
      setTimeout(() => res.end(body.subarray(body.length >> 1)), 1500);
    }
  });
  await new Promise((r) => releaseServer.listen(releasePort, '127.0.0.1', r));
  const updateArgs = () => ['-version=0.1.0', `-releases=http://127.0.0.1:${releasePort}/releases.json`, `-app=${path.join(installed, 'Project B.app')}`];
  const bubble = ui.locator('.update');

  fs.rmSync(installed, { recursive: true, force: true });
  fakeApp(installed, '0.1.0');
  await stopServer();
  await startServer(updateArgs());
  await page.goto(base + '/');
  errors.length = 0;
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await ui.getByText(/Downloading Project B 0\.2\.0… [1-9]\d?%/).waitFor({ timeout: 10000 });
  await settle(page);
  const progress = Number(await ui.getByRole('progressbar', { name: 'Update download' }).getAttribute('aria-valuenow'));
  check('the bubble shows how much of the release has arrived', progress > 0 && progress < 100, String(progress));
  await page.screenshot({ path: path.join(out, '7-update-downloading.png') });
  r = await regions();
  let bb = await bubble.boundingBox();
  check('the download shows as a bubble in the page\'s bottom-left corner', bb.x === r.bounds.x && bb.y + bb.height === r.bounds.y + r.bounds.h, JSON.stringify({ bb, bounds: r.bounds }));
  check('the bubble takes no layout space', JSON.stringify(r.bounds) === pageBounds, JSON.stringify(r.bounds));
  check('the bubble is shell, so its click does not reach the page', r.regions.length === 3 && (await regions(`?x=${bb.x + 10}&y=${bb.y + 10}`)).hit === 'shell', JSON.stringify(r.regions));
  check('just above the bubble is still the page', (await regions(`?x=${bb.x + 10}&y=${bb.y - 10}`)).hit === 'page');
  const relaunch = ui.getByRole('button', { name: /Relaunch to update/ });
  await relaunch.waitFor({ timeout: 10000 });
  await settle(page);
  await page.screenshot({ path: path.join(out, '8-update-ready.png') });
  check('the bubble offers relaunch once the release is checked and staged', /0\.2\.0 is ready/.test(await bubble.innerText()), await bubble.innerText());
  check('the installed app is untouched until relaunch', version(path.join(installed, 'Project B.app')) === '0.1.0');
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();
  await settle(page);
  r = await regions();
  bb = await bubble.boundingBox();
  check('with the tabs on top the bubble follows the page\'s corner', bb.x === r.bounds.x && bb.x === 4 && bb.y + bb.height === r.bounds.y + r.bounds.h, JSON.stringify({ bb, bounds: r.bounds }));
  await page.screenshot({ path: path.join(out, '9-update-ready-horizontal.png') });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await relaunch.click();
  await until(async () => (await regions()).closed === true);
  check('relaunch puts the new release in place and closes the window', version(path.join(installed, 'Project B.app')) === '0.2.0');
  check('the running release is kept as the previous one', version(path.join(profile, 'updates/previous/Project B.app')) === '0.1.0');
  await stopServer();
  const ran = path.join(installed, 'Project B.app/Contents/MacOS/ran');
  for (let i = 0; i < 60 && !fs.existsSync(ran); i++) await new Promise((r) => setTimeout(r, 50));
  check('the new release starts once the old process has exited', fs.existsSync(ran) && fs.readFileSync(ran, 'utf8').trim() === '0.2.0');

  // A corrupted download is refused and the installed release keeps working.
  served = Buffer.from(good);
  served[Math.floor(served.length / 2)] ^= 0xff;
  fs.rmSync(installed, { recursive: true, force: true });
  fakeApp(installed, '0.1.0');
  await startServer(updateArgs());
  await page.goto(base + '/');
  errors.length = 0;
  await ui.getByText(/Update to 0\.2\.0 failed/).waitFor({ timeout: 10000 });
  await settle(page);
  await page.screenshot({ path: path.join(out, '10-update-failed.png') });
  check('a corrupted download is refused and the bubble says so', /sha256/.test(await bubble.innerText()), await bubble.innerText());
  check('the installed app is left alone', version(path.join(installed, 'Project B.app')) === '0.1.0' && !fs.existsSync(path.join(profile, 'updates/stage')));
  await bar.fill('https://example.org/');
  await bar.press('Enter');
  await loaded('https://example.org/');
  check('the running release keeps working', (await bar.inputValue()) === 'https://example.org/');
  releaseServer.close();

  check('no page errors', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  fs.rmSync(profile, { recursive: true, force: true });
  fs.rmSync(releases, { recursive: true, force: true });
  console.log(failed ? `\n${failed} FAILED` : '\nall passed');
  process.exit(failed ? 1 : 0);
})().catch(async (e) => {
  console.error(e);
  server?.kill('SIGKILL');
  process.exit(1);
});
