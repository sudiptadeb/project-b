// The settings page on the mock engine, in a real browser: what it shows, what it may ask
// Go, and that it follows changes made anywhere.
// Run: node shell/e2e/settings.cjs   (from the repo root, after `npm run build` in shell/)
//
// The dev server serves the settings page in tab <id> at /tab/<id>/ and tags what it sends
// with that tab, as the engine does with the page's WebContents.

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require(path.join(os.homedir(), '.local/playwright/node_modules/playwright'));

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
const port = 8797;
const base = `http://127.0.0.1:${port}`;

fs.mkdirSync(out, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-settings-'));
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

// A release server whose stable channel is the version the app runs, so a check finds it
// up to date. It counts manifest requests: each is one check.
let manifests = 0;
const releasePort = 8796;
const releaseServer = http.createServer((req, res) => {
  manifests++;
  res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' }, releases: { '0.2.0': { assets: {} } } }));
});
const installed = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-installed-'));
fs.mkdirSync(path.join(installed, 'Project B.app/Contents/MacOS'), { recursive: true });

let server;
async function startServer() {
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`,
    '-version=0.2.0', `-releases=http://127.0.0.1:${releasePort}/releases.json`, `-app=${path.join(installed, 'Project B.app')}`], { stdio: 'ignore' });
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
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function until(fn, ms = 5000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await fn()) return true;
    await sleep(30);
  }
  return false;
}
const prefsFile = () => JSON.parse(fs.readFileSync(path.join(profile, 'prefs.json'), 'utf8'));

(async () => {
  await new Promise((r) => releaseServer.listen(releasePort, '127.0.0.1', r));
  await startServer();
  const browser = await chromium.launch();
  const errors = [];
  const watch = (p) => {
    p.on('pageerror', (e) => errors.push(e.message));
    p.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
    return p;
  };
  const shell = watch(await browser.newPage({ viewport: { width: 1100, height: 700 } }));
  await shell.goto(base + '/');
  const bar = shell.getByRole('combobox', { name: 'Address' });
  const mock = shell.frameLocator('iframe');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  const mockShows = (url) => until(async () => (await mock.locator('#url').textContent()) === url && (await mock.locator('#state').textContent()) === 'Loaded');

  // Typed into the address bar, the settings page loads in the tab.
  await bar.fill('chrome://settings/');
  await bar.press('Enter');
  check('typing chrome://settings/ loads it in the tab', await mockShows('chrome://settings/'));

  // The page itself, as tab 1 holds it.
  const settings = watch(await browser.newPage({ viewport: { width: 1000, height: 640 } }));
  await settings.goto(base + '/tab/1/');
  const radio = (p, name) => p.getByRole('radio', { name });
  await until(async () => await radio(settings, 'Vertical').isChecked());
  check('the settings page shows the current layout', (await radio(settings, 'Vertical').isChecked()) && !(await radio(settings, 'Horizontal').isChecked()));
  check('it shows the running version', (await settings.locator('.version').textContent()) === '0.2.0', await settings.locator('.version').textContent());
  check('it shows automatic updates on, the default', await settings.getByRole('switch', { name: 'Automatic updates' }).isChecked());
  await settings.screenshot({ path: path.join(out, 'settings-light.png') });

  // Changing it there goes through the prefs manager: saved, and the shell follows.
  await radio(settings, 'Horizontal').check();
  await shell.locator('.strip').waitFor();
  check('choosing Horizontal there switches the shell', true);
  check('and the prefs manager saved it', prefsFile().tabLayout === 'horizontal', JSON.stringify(prefsFile()));
  await until(async () => await radio(settings, 'Horizontal').isChecked());
  check('the page shows what Go now holds', await radio(settings, 'Horizontal').isChecked());

  // A second settings tab, and a change from the shell: both pages follow.
  await shell.getByRole('button', { name: 'New Tab', exact: true }).click();
  await shell.waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  await bar.fill('chrome://settings/');
  await bar.press('Enter');
  await mockShows('chrome://settings/');
  const second = watch(await browser.newPage({ viewport: { width: 1000, height: 640 } }));
  await second.goto(base + '/tab/2/');
  check('a second settings tab shows the same layout', await until(async () => await radio(second, 'Horizontal').isChecked()));
  await shell.getByRole('button', { name: 'Tabs on the side' }).click();
  await shell.locator('nav').waitFor();
  check('a change from the shell reaches the first settings tab', await until(async () => await radio(settings, 'Vertical').isChecked()));
  check('and the second', await until(async () => await radio(second, 'Vertical').isChecked()));
  await radio(second, 'Horizontal').check();
  check('a change in one settings tab reaches the other', await until(async () => await radio(settings, 'Horizontal').isChecked()));
  await radio(second, 'Vertical').check();
  await shell.locator('nav').waitFor();

  // The settings page may read and set prefs, read the update state and ask for a check.
  // Anything else it sends is dropped: here, the shell's commands to open a tab and to
  // navigate.
  const sendAs = (p, msg) => p.evaluate((body) => fetch('shell/send', { method: 'POST', body }), JSON.stringify(msg));
  const rows = () => shell.evaluate(() => document.querySelectorAll('[role=tab]').length);
  await sendAs(settings, { t: 'newTab' });
  await sendAs(settings, { t: 'navigate', text: 'https://evil.test/' });
  await sendAs(settings, { t: 'activateTab', id: 1 });
  await sleep(500);
  check('a shell command from the settings page is dropped', (await rows()) === 2 && (await bar.inputValue()) === 'chrome://settings/' && (await shell.locator('[role=tab][aria-selected=true]').count()) === 1 && (await shell.locator('[role=tab]').nth(1).getAttribute('aria-selected')) === 'true',
    `${await rows()} ${await bar.inputValue()}`);

  // Updates. A check asked for goes to the update manager, which says what it found. The
  // manager's own check two seconds after launch comes first, so it isn't counted here.
  const status = settings.locator('.status');
  await until(() => manifests > 0);
  const before = manifests;
  await settings.getByRole('button', { name: 'Check for updates' }).click();
  check('Check asks the update manager, which says it is up to date', await until(async () => (await status.textContent()) === 'Up to date'), await status.textContent());
  check('one check reached the release server', manifests === before + 1, `${before} -> ${manifests}`);

  // Automatic updates off: saved, and the manager's timer stops. It stays off after a
  // restart; a check asked for still runs; on again, the timer starts as at launch.
  await settings.getByRole('switch', { name: 'Automatic updates' }).uncheck();
  check('turning automatic updates off saves it', await until(() => prefsFile().autoUpdate === false), JSON.stringify(prefsFile()));
  await second.close();
  await stopServer();
  await startServer();
  const atStart = manifests;
  await shell.goto(base + '/');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await bar.fill('chrome://settings/');
  await bar.press('Enter');
  await mockShows('chrome://settings/');
  await settings.goto(base + '/tab/1/');
  await until(async () => (await settings.locator('.version').textContent()) === '0.2.0');
  errors.length = 0; // Stopping the server broke the old pages' event streams, as it should.
  check('after a restart the switch is still off', !(await settings.getByRole('switch', { name: 'Automatic updates' }).isChecked()));
  await sleep(3500);
  check('and the update manager made no check of its own', manifests === atStart, `${atStart} -> ${manifests}`);
  await settings.getByRole('button', { name: 'Check for updates' }).click();
  check('a check asked for still runs', await until(async () => manifests === atStart + 1 && (await status.textContent()) === 'Up to date'), `${manifests} ${await status.textContent()}`);
  await settings.getByRole('switch', { name: 'Automatic updates' }).check();
  check('turning it on again saves it', await until(() => prefsFile().autoUpdate === true));
  check('and the timer checks again, as at launch', await until(() => manifests === atStart + 2, 4000), `${atStart} -> ${manifests}`);

  // The toolbar's settings button opens the page in a new, active tab, in both layouts.
  const openFromToolbar = async (n) => {
    await shell.getByRole('button', { name: 'Settings', exact: true }).click();
    await shell.waitForFunction((n) => document.querySelectorAll('[role=tab]').length === n, n);
    const active = await shell.locator('[role=tab]').nth(n - 1).getAttribute('aria-selected');
    return active === 'true' && (await mockShows('chrome://settings/')) && (await bar.inputValue()) === 'chrome://settings/';
  };
  check('the toolbar\'s settings button opens settings in a new, active tab', await openFromToolbar(2));
  await shell.screenshot({ path: path.join(out, 'settings-button-vertical.png'), clip: { x: 0, y: 0, width: 1100, height: 60 } });
  await shell.getByRole('button', { name: 'Tabs on top' }).click();
  await shell.locator('.strip').waitFor();
  check('and with the tabs on top', await openFromToolbar(3));
  await shell.screenshot({ path: path.join(out, 'settings-button-horizontal.png'), clip: { x: 0, y: 0, width: 1100, height: 100 } });
  const right = await shell.evaluate(() => [document.querySelector('[aria-label=Settings]').getBoundingClientRect().right, window.innerWidth]);
  check('it sits at the right end of the bar, inside the frame and the end gap', right[0] === right[1] - 4 - 8, JSON.stringify(right));
  await shell.getByRole('button', { name: 'Tabs on the side' }).click();
  await shell.locator('nav').waitFor();

  await settings.emulateMedia({ colorScheme: 'dark' });
  await settings.screenshot({ path: path.join(out, 'settings-dark.png') });

  check('no page errors', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  releaseServer.close();
  fs.rmSync(profile, { recursive: true, force: true });
  fs.rmSync(installed, { recursive: true, force: true });
  console.log(failed ? `\n${failed} FAILED` : '\nall passed');
  process.exit(failed ? 1 : 0);
})().catch(async (e) => {
  console.error(e);
  server?.kill('SIGKILL');
  process.exit(1);
});
