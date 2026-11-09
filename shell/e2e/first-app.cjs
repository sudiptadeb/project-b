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
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 } });
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
  check('opens empty with only the bar as shell', r.regions.length === 1 && (await bar.inputValue()) === '', JSON.stringify(r));
  check('page sits below the bar, inside the frame', r.bounds.y === r.regions[0].h && r.bounds.x === 10 && r.bounds.w === 1100 - 20 && r.bounds.y + r.bounds.h === 700 - 10, JSON.stringify(r));

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
  check('the box becomes a second shell rectangle', r.regions.length === 2, JSON.stringify(r.regions));
  const box = await list.boundingBox();
  const over = await regions(`?x=${box.x + 20}&y=${box.y + 10}`);
  check('the box extends over the page', box.y + box.height > r.bounds.y + 40, JSON.stringify({ box, bounds: r.bounds }));
  check('a click on the box goes to the shell', over.hitShell === true);
  const beside = await regions(`?x=${box.x - 40}&y=${box.y + 10}`);
  check('a click beside the box goes to the page', beside.hitShell === false);

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
  check('escape closes the box and its rectangle goes', (await list.count()) === 0 && r.regions.length === 1, JSON.stringify(r.regions));

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

  // Events from the page side.
  await mock.getByRole('button', { name: 'Cmd+L' }).click();
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

  // History survives a restart.
  await stopServer();
  await startServer();
  await page.goto(base + '/');
  errors.length = 0; // Stopping the server breaks the event stream, as it should.
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  check('history survives a restart', (await options.count()) === 2);
  check('titles survive a restart', /Mock example/.test(await options.first().innerText()));

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
