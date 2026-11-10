// The settings page on the mock engine, in a real browser: the gear opens it, what it
// shows, that its changes go through Go, and the keyboard section. The mock has no window
// to put the page over, so the test opens /settings itself once Go has asked for it.
// Run: node shell/e2e/settings.cjs   (from the repo root, after `npm run build` in shell/)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');

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

const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
const settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

const keymapFile = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(profile, 'keymap.json'), 'utf8'));
  } catch {
    return null;
  }
};
const menuHas = (menu, title) => JSON.stringify(menu).includes(`"title":"${title}"`);

(async () => {
  await new Promise((r) => releaseServer.listen(releasePort, '127.0.0.1', r));
  await startServer();
  const browser = await chromium.launch();
  const errors = [];
  const watch = (p) => {
    p.on('pageerror', (e) => errors.push(e.message));
    p.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  };
  const shell = await browser.newPage({ viewport: { width: 1100, height: 700 } });
  watch(shell);
  await shell.goto(base + '/');
  await shell.frameLocator('iframe').getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(shell);
  const root = (name) => shell.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);

  // The gear asks Go, and Go opens the settings page.
  check('settings starts closed', (await regions()).settingsOpen === false);
  await shell.getByRole('button', { name: 'Settings', exact: true }).click();
  check('the gear opens the settings page', await until(async () => (await regions()).settingsOpen === true));

  const open = async () => {
    const p = await browser.newPage({ viewport: { width: 1100, height: 700 } });
    watch(p);
    await p.goto(base + '/settings');
    await p.getByRole('heading', { name: 'Settings' }).waitFor();
    return p;
  };
  let page = await open();
  const radio = (label) => page.getByRole('radio', { name: label });
  const autoUpdate = page.getByRole('switch', { name: 'Automatic updates' });

  check('it shows the current layout', (await radio('Vertical').isChecked()) && !(await radio('Horizontal').isChecked()));
  check('it shows the running version', (await page.locator('.version').textContent()) === '0.2.0');
  check('it shows automatic updates on, the default', await autoUpdate.isChecked());
  await page.screenshot({ path: path.join(out, 'settings-general.png') });

  await radio('Horizontal').check();
  check('choosing Horizontal switches the shell and is saved', await until(async () => (await shell.locator('.strip').count()) > 0 && prefsFile().tabLayout === 'horizontal'), JSON.stringify(prefsFile()));
  await shell.getByRole('button', { name: 'Tabs on the side' }).click();
  check('a change from the toolbar reaches the settings page', await until(async () => await radio('Vertical').isChecked()));

  const counted = manifests;
  await page.getByRole('radio', { name: 'Blue' }).click();
  check('a frame colour recolours the window and the shell, and is saved', await until(async () => (await root('--frame')) === '#3470d0' && (await regions()).windowColor === '#3470d0' && prefsFile().frameColor === '#3470d0'));
  await page.getByRole('button', { name: 'Check for updates' }).click();
  check('Check asks the update manager, which says it is up to date', await until(async () => (await page.locator('.status').textContent()) === 'Up to date'));
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);
  await autoUpdate.click();
  check('the switch turns automatic updates off and saves it', await until(() => prefsFile().autoUpdate === false));

  // Keyboard.
  await page.getByRole('button', { name: 'Keyboard' }).click();
  check('the keymap follows its template until an edit', keymapFile() === null);
  await page.getByRole('button', { name: 'Add a key for New Tab' }).click();
  await page.keyboard.press('Meta+Shift+KeyK');
  check('a recorded key is added, and the keymap becomes the user\'s', await until(() => {
    const k = keymapFile();
    return k && !k.follows && k.template === 'project-b' && k.actions['tab.new'].mac.some((b) => b.keys === 'Shift+Cmd+K');
  }), JSON.stringify(keymapFile()?.actions?.['tab.new']));
  check('the page shows it', await until(async () => (await page.getByRole('region', { name: 'File' }).textContent()).includes('⇧⌘K')));
  check('Quit\'s key is locked: no remove button', (await page.getByRole('button', { name: 'Remove Cmd+Q' }).count()) === 0);
  check('Quit can\'t leave the menu', await page.getByRole('switch', { name: 'Quit Project B in the menu' }).isDisabled());

  await page.getByRole('switch', { name: 'Reload This Page in the menu', exact: true }).click();
  check('hiding an action takes it out of the menu', await until(async () => !menuHas((await regions()).menu, 'Reload This Page')));
  await page.getByRole('button', { name: 'Reset Reload This Page', exact: true }).click();
  check('reset puts it back', await until(async () => menuHas((await regions()).menu, 'Reload This Page')));

  await page.getByLabel('Template').selectOption('chrome');
  check('picking a template resets to following it', await until(() => keymapFile()?.follows === 'chrome'), JSON.stringify(keymapFile()));
  check('and the added key is gone', await until(async () => !(await page.getByRole('region', { name: 'File' }).textContent()).includes('⇧⌘K')));
  await page.screenshot({ path: path.join(out, 'settings-keyboard.png'), fullPage: true });

  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await regions()).settingsOpen === false));
  await shell.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  await page.close();
  page = await open();
  await page.getByRole('button', { name: 'Close settings' }).click();
  check('the close button closes it', await until(async () => (await regions()).settingsOpen === false));

  check('no errors in the pages', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  releaseServer.close();
  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})();
