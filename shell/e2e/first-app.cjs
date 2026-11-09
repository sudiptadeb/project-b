// End-to-end test of the first app on the mock engine, in a real browser.
// Run: node shell/e2e/first-app.cjs   (from the repo root; Playwright is preinstalled)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { chromium } = require(path.join(os.homedir(), '.local/playwright/node_modules/playwright'));

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
const port = 8799;
const base = `http://127.0.0.1:${port}`;

fs.mkdirSync(out, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-e2e-'));
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

let server;
async function startServer() {
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, '-block=blocked.test'], { stdio: 'ignore' });
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
const settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

(async () => {
  await startServer();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 }, screen: { width: 1920, height: 1080 } });
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));

  await page.goto(base + '/');
  const bar = page.getByRole('combobox', { name: 'Address' });
  const list = page.locator('#suggestions');
  const options = page.getByRole('option');
  const mock = page.frameLocator('iframe');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(page);
  await page.screenshot({ path: path.join(out, '1-empty.png') });

  const back = page.getByRole('button', { name: 'Back' });
  const forward = page.getByRole('button', { name: 'Forward' });
  const reload = page.getByRole('button', { name: 'Reload' });
  check('opens with nothing to go back to or reload', (await back.isDisabled()) && (await forward.isDisabled()) && (await reload.isDisabled()));

  let r = await regions();
  check('opens empty with the top bar and the sidebar as shell', r.regions.length === 2 && (await bar.inputValue()) === '', JSON.stringify(r));
  check('page sits right of the sidebar, below the bar, inside the frame', r.bounds.x === 200 && r.bounds.y === 32 && r.bounds.w === 1100 - 200 - 4 && r.bounds.y + r.bounds.h === 700 - 4, JSON.stringify(r));
  check('opens with the address bar focused, as a new tab does', await bar.evaluate((el) => el === document.activeElement));

  // Waits for the commit. The URL shows as soon as loading starts, and a new navigation
  // before the commit would correctly replace this one.
  const loaded = async (expected) => {
    await mock.getByText(expected, { exact: true }).waitFor();
    await mock.getByText('Loaded', { exact: true }).waitFor();
    await page.waitForFunction((v) => document.querySelector('input').value === v, expected);
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
  await page.waitForFunction(() => document.querySelector('[aria-label=Forward]').disabled);
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
  check('down moves into the list', (await bar.inputValue()).startsWith('https://example') && (await page.locator('li.active').count()) === 1);
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  check('up from the first item returns to what was typed', (await bar.inputValue()) === 'exam' && (await page.locator('li.active').count()) === 0, await bar.inputValue());
  check('focus never left the address bar', await bar.evaluate((el) => el === document.activeElement));
  await page.keyboard.press('Escape');
  await settle(page);
  r = await regions();
  check('escape closes the box and its rectangle goes', (await list.count()) === 0 && r.regions.length === 2, JSON.stringify(r.regions));

  // Clicking a suggestion.
  await bar.fill('');
  await page.keyboard.type('go.d');
  await options.first().click();
  await page.waitForFunction(() => document.querySelector('input').value === 'https://go.dev/learn');
  check('clicking a suggestion navigates', (await list.count()) === 0);

  // Clicking the page closes the box.
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await page.mouse.click(1050, 650);
  await settle(page);
  check('clicking the page closes the box', (await list.count()) === 0 && (await bar.inputValue()) === 'https://go.dev/learn');

  // Events from the page side. Shortcuts arrive from the engine, whichever page had focus.
  const shortcut = async (name) => {
    await mock.getByLabel('Shortcut name').fill(name);
    await mock.getByRole('button', { name: 'Shortcut' }).click();
  };
  await shortcut('focusAddressBar');
  await page.waitForFunction(() => document.activeElement === document.querySelector('input'));
  check('the shortcut focuses the address bar from the page', true);
  await page.keyboard.press('Escape');

  await mock.getByRole('button', { name: 'Fail' }).click();
  await page.getByText('Failed', { exact: true }).waitFor();
  check('a failed navigation shows', true);
  await mock.getByRole('button', { name: 'Crash' }).click();
  await page.getByText('Crashed', { exact: true }).waitFor();
  check('a crashed page shows', true);

  // Policy.
  await bar.fill('https://www.blocked.test/secret');
  await bar.press('Enter');
  await page.getByText('Blocked', { exact: true }).waitFor();
  await bar.click();
  await bar.fill('');
  await page.keyboard.type('secret');
  await settle(page);
  check('a blocked URL stays out of history', (await options.count()) === 0);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');

  // The engine says where the window controls are; the shell leaves exactly that space.
  // In full screen there are none, so the first control moves into their place.
  const toggle = page.getByRole('button', { name: /sidebar/ });
  const windowLeft = (await toggle.boundingBox()).x;
  check('the first control starts after the window controls the engine reported', windowLeft === 9 + 60 + 8, String(windowLeft));
  r = await regions('?x=74&y=16');
  check('the empty strip between the window buttons and the controls drags the window', r.drag.length === 7 && r.hit === 'drag', JSON.stringify(r.drag));
  check('the empty part of the sidebar drags the window', (await regions('?x=100&y=600')).hit === 'drag');
  check('the control itself is shell, not drag', (await regions(`?x=${windowLeft + 12}&y=16`)).hit === 'shell');
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await page.waitForFunction(() => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x < 20);
  check('in full screen the controls move into the buttons\' place', (await toggle.boundingBox()).x === 8, String((await toggle.boundingBox()).x));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await page.waitForFunction((x) => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x === x, windowLeft);
  check('leaving full screen gives the buttons their space back', true);

  // Tabs.
  const rows = page.getByRole('tab');
  const activeRow = page.locator('.row.tab.active');
  const rowTitles = () => rows.evaluateAll((els) => els.map((el) => el.querySelector('.title').textContent));
  check('one tab to start with, active', (await rows.count()) === 1 && (await activeRow.count()) === 1);
  await visit('https://example.org/', 'https://example.org/');
  await page.getByRole('button', { name: 'New Tab', exact: true }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('New Tab adds a tab and makes it active', (await activeRow.innerText()).startsWith('New Tab'), await activeRow.innerText());
  check('a new tab has an empty, focused address bar', (await bar.inputValue()) === '' && (await bar.evaluate((el) => el === document.activeElement)));
  await mock.getByText('tab 2', { exact: true }).waitFor();
  r = await regions();
  check('the first tab was told it is hidden', JSON.stringify(r.hidden) === '[1]', JSON.stringify(r.hidden));
  check('back and reload follow the new, empty tab', (await back.isDisabled()) && (await reload.isDisabled()));
  await visit('https://example.net/', 'https://example.net/');
  check('each tab keeps its own title', JSON.stringify(await rowTitles()) === JSON.stringify(['Mock example.org', 'Mock example.net']), JSON.stringify(await rowTitles()));
  await rows.first().click();
  await page.waitForFunction((v) => document.querySelector('input').value === v, 'https://example.org/');
  check('clicking a row switches tabs and the address bar follows', (await rows.first().getAttribute('aria-selected')) === 'true');
  check('back follows the tab that has history', await back.isEnabled());
  await mock.getByText('tab 1', { exact: true }).waitFor();
  check('the page view shows the active tab', true);

  // Favicons come from the page; a tab without one shows none.
  check('a tab with no favicon shows no icon', (await page.locator('[role=tab] img').count()) === 0);
  await mock.getByRole('button', { name: 'Favicon' }).click();
  await page.locator('[role=tab][aria-selected=true] img').waitFor();
  check('the favicon the page gives shows on its row only', (await page.locator('[role=tab] img').count()) === 1);
  await visit('https://example.org/', 'https://example.org/');
  await page.waitForFunction(() => document.querySelectorAll('[role=tab] img').length === 0);
  check('a new page drops the old favicon', true);

  // The page opens a tab.
  await mock.getByRole('button', { name: 'Open in new tab' }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  await loaded('https://opened.test/');
  check('a tab the page opens appears and is active', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());

  await shortcut('tab1');
  await page.waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('Cmd+1 picks the first tab', true);
  await shortcut('nextTab');
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]')[1].getAttribute('aria-selected') === 'true');
  check('Ctrl+Tab picks the next tab', true);
  await shortcut('closeTab');
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('Cmd+W closes the active tab and its right neighbour takes over', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());
  await shortcut('newTab');
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  check('Cmd+T opens a tab', (await bar.inputValue()) === '');

  // Close with the button on the row.
  await rows.nth(1).hover();
  await rows.nth(1).getByRole('button', { name: /^Close/ }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a row closes that tab', !(await rowTitles()).some((t) => t.includes('opened.test')), JSON.stringify(await rowTitles()));

  // The sidebar collapses and the page follows.
  await toggle.click();
  await page.waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 0);
  await settle(page);
  r = await regions();
  check('collapsing the sidebar gives its width to the page', r.bounds.x === 4 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '4-collapsed.png') });
  await toggle.click();
  await page.waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 196);
  await settle(page);
  r = await regions();
  check('expanding it takes the width back', r.bounds.x === 200, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '5-tabs.png') });

  // The same tabs on top. One switch, kept by the managers, drives both layouts.
  const strip = page.locator('.strip');
  await page.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();
  await settle(page);
  check('switching puts the same tabs in a strip on top', (await rows.count()) === 2 && (await activeRow.count()) === 0 && (await page.locator('.strip [aria-selected=true]').count()) === 1);
  check('the strip leaves the window controls their space', (await rows.first().boundingBox()).x === 9 + 60 + 8, String((await rows.first().boundingBox()).x));
  r = await regions();
  check('without a sidebar the page spans the width below two rows', r.bounds.x === 4 && r.bounds.y === 64 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  check('the empty strip drags the window', (await regions('?x=900&y=16')).hit === 'drag');
  await page.getByRole('button', { name: 'New Tab', exact: true }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  check('New Tab works from the strip', (await bar.inputValue()) === '');
  await rows.first().click();
  await page.waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('clicking a strip tab activates it', true);
  await rows.nth(2).hover();
  await rows.nth(2).getByRole('button', { name: /^Close/ }).click();
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a strip tab closes it', true);
  await page.screenshot({ path: path.join(out, '6-horizontal.png') });
  await page.getByRole('button', { name: 'Tabs on the side' }).click();
  await page.locator('nav').waitFor();
  await settle(page);
  check('switching back brings the sidebar and the page follows', (await rows.count()) === 2 && (await regions()).bounds.x === 200);
  await page.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();

  // Closing the last tab closes the window.
  await shortcut('closeTab');
  await shortcut('closeTab');
  await page.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 0);
  await mock.getByText('Window closed', { exact: true }).waitFor();
  check('closing the last tab closes the window', (await regions()).closed === true);

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
  check('the tab layout survives a restart', (await page.locator('.strip').count()) === 1);
  await page.getByRole('button', { name: 'Tabs on the side' }).click();
  await page.locator('nav').waitFor();
  check('and can be switched back', (await page.locator('.strip').count()) === 0);

  // Phone width.
  const phone = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
  await phone.goto(base + '/');
  const phoneBar = phone.getByRole('combobox', { name: 'Address' });
  await phone.frameLocator('iframe').getByRole('button', { name: 'Navigate' }).waitFor();
  await phoneBar.click();
  await phone.keyboard.type('exam');
  await phone.getByRole('option').first().waitFor();
  await settle(phone);
  await phone.screenshot({ path: path.join(out, '3-phone.png') });
  const scroll = await phone.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  check('no horizontal scroll on a phone', scroll);

  check('no page errors', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  fs.rmSync(profile, { recursive: true, force: true });
  console.log(failed ? `\n${failed} FAILED` : '\nall passed');
  process.exit(failed ? 1 : 0);
})().catch(async (e) => {
  console.error(e);
  server?.kill('SIGKILL');
  process.exit(1);
});
