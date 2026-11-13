// The settings page on the mock engine, in a real browser: the gear opens it, what it
// shows, that its changes go through Go, and the keyboard section. The dev window draws the
// page over the shell once Go asks for it, as the real window does
// (core/cmd/dev/window.html); the test drives a page of its own at /settings, which talks
// to Go the same way.
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

// Flags of the types registry.json has no row of yet, so the page draws every type.
const moreFlags = path.join(profile, 'more-flags.json');
fs.writeFileSync(moreFlags, JSON.stringify([
  { id: 'mode', name: 'Mode', description: 'An enum', switch: 'pb-mode', type: 'enum', choices: ['fast', 'slow'] },
  { id: 'label', name: 'Label', description: 'A string', switch: 'pb-label', type: 'string', pattern: '^[a-z]+$' },
  { id: 'sizes', name: 'Sizes', description: 'Numbers', switch: 'pb-sizes', type: 'numbers', min: 1 },
]));

let server;
async function startServer() {
  // A server left over from another run would answer in ours' place.
  if (await fetch(base + '/mock/regions').then(() => true, () => false)) {
    throw new Error(`port ${port} already has a server: stop it and run again`);
  }
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, `-flags=${moreFlags}`,
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
  // The window page, which holds the shell in a frame: see first-app.cjs.
  const ui = shell.frameLocator('iframe.shell');
  const shellFrame = () => shell.frame({ url: /\/shell\/\?window=1$/ });
  await shell.goto(base + '/');
  await shell.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(shell);
  const root = (name) => shellFrame().evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);

  // The gear asks Go, and Go opens the settings page.
  check('settings starts closed', (await regions()).settingsOpen === false);
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  check('the gear opens the settings page', await until(async () => (await regions()).settingsOpen === true));
  check('and the dev window shows it over the shell', await until(() => shell.locator('iframe.mock-settings').isVisible()));
  await settle(shell);
  await shell.screenshot({ path: path.join(out, 'window-settings.png') });

  // The window under the settings page, saved as name: the middle of the page area looks
  // the same with the shell's frame over it as without.
  const seesThrough = async (name) => {
    const b = (await regions()).bounds;
    const clip = { x: b.x + b.w / 4, y: b.y + b.h / 2, width: b.w / 2, height: b.h / 4 };
    const show = (selector, on) => shell.locator(selector).evaluate((el, v) => (el.style.visibility = v), on ? '' : 'hidden');
    await show('iframe.mock-settings', false);
    await settle(shell);
    await shell.screenshot({ path: path.join(out, name) });
    const over = await shell.screenshot({ clip });
    await show('iframe.shell', false);
    await settle(shell);
    const under = await shell.screenshot({ clip });
    await show('iframe.shell', true);
    await show('iframe.mock-settings', true);
    return over.equals(under);
  };

  const open = async () => {
    const p = await browser.newPage({ viewport: { width: 1100, height: 700 } });
    watch(p);
    await p.goto(base + '/settings/');
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

  await page.getByRole('radio', { name: 'Dark' }).check();
  check('Dark turns the shell dark and is saved', await until(async () => (await root('--text')) === '#e8eaed' && prefsFile().theme === 'dark'), await root('--text'));
  check('and the settings page too', await until(async () => (await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)) === 'dark'));
  await shell.screenshot({ path: path.join(out, 'window-settings-dark.png') });
  check('in dark the page shows through the shell, which Chrome paints opaque if their schemes differ', await seesThrough('window-dark.png'));
  await page.getByRole('radio', { name: 'Light' }).check();
  check('Light turns it back', await until(async () => (await root('--text')) === '#1f1f1f' && prefsFile().theme === 'light'));
  check('and in light too', await seesThrough('window-light.png'));
  await page.getByRole('radio', { name: 'System' }).check();
  await until(() => prefsFile().theme === 'system');

  await radio('Horizontal').check();
  check('choosing Horizontal switches the shell and is saved', await until(async () => (await ui.locator('.strip').count()) > 0 && prefsFile().tabLayout === 'horizontal'), JSON.stringify(prefsFile()));
  // The settings page covers the toolbar, as in the real window, so the button is pressed
  // without the mouse, as its menu command would.
  await ui.getByRole('button', { name: 'Tabs on the side' }).dispatchEvent('click');
  check('a change from the toolbar reaches the settings page', await until(async () => await radio('Vertical').isChecked()));

  // The update manager checks by itself two seconds after it starts. Let that one land
  // first, so the count below is only the button's.
  await until(() => manifests > 0);
  const counted = manifests;
  await page.getByRole('radio', { name: 'Blue' }).click();
  check('a frame colour recolours the window and the shell, and is saved', await until(async () => (await root('--frame')) === '#3470d0' && (await regions()).windowColor === '#3470d0' && prefsFile().frameColor === '#3470d0'));
  await page.getByRole('button', { name: 'Check for updates' }).click();
  check('Check asks the update manager, which says it is up to date', await until(async () => (await page.locator('.status').textContent()) === 'Up to date'));
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);
  await autoUpdate.click();
  check('the switch turns automatic updates off and saves it', await until(() => prefsFile().autoUpdate === false));

  // Flags: saved for the next launch, with Relaunch offered until they are what runs.
  const flagsFile = () => {
    try {
      return JSON.parse(fs.readFileSync(path.join(profile, 'flags.json'), 'utf8'));
    } catch {
      return null;
    }
  };
  await page.getByRole('button', { name: 'Flags' }).click();
  const relaunch = page.getByRole('button', { name: 'Relaunch' });
  check('Flags lists the flags, none waiting', await page.getByRole('region', { name: 'Flags' }).getByText('Chromium features').isVisible() && (await relaunch.count()) === 0);
  check('Extensions is off and can\'t be turned on in a build without them', await page.getByRole('switch', { name: /Extensions/ }).isDisabled());
  check('and says so', (await page.locator('.row', { hasText: 'Extensions' }).textContent()).includes('Not in this build'));
  const port = page.getByLabel('Remote debugging port');
  await port.fill('80');
  await port.press('Enter');
  check('a port it can\'t use is refused', await until(async () => (await port.inputValue()) === '') && flagsFile() === null);
  await port.fill('9333');
  await port.press('Enter');
  check('a port is saved, and Relaunch shows', await until(async () => flagsFile()?.remoteDebugging === '9333' && await relaunch.isVisible()), JSON.stringify(flagsFile()));
  check('beside that flag only', (await relaunch.count()) === 1 && (await page.locator('.row', { has: port }).getByRole('button', { name: 'Relaunch' }).count()) === 1);
  check('each flag says what it does', await page.getByText('Let DevTools clients connect on this port').isVisible());
  await page.screenshot({ path: path.join(out, 'settings-flags.png') });
  await port.fill('');
  await port.press('Enter');
  check('cleared, nothing waits', await until(async () => (await relaunch.count()) === 0 && JSON.stringify(flagsFile()) === '{}'));

  // The other types.
  const mode = page.getByLabel('Mode');
  check('an enum is a select, Off first', (await mode.evaluate((s) => [...s.options].map((o) => o.text).join())) === 'Off,fast,slow');
  await mode.selectOption('slow');
  check('picking a choice saves it', await until(async () => flagsFile()?.mode === 'slow' && (await page.locator('.row', { has: mode }).getByRole('button', { name: 'Relaunch' }).count()) === 1));
  await mode.selectOption('');
  const label = page.getByLabel('Label');
  await label.fill('Not Lower');
  await label.press('Enter');
  check('a string the pattern refuses is not saved', await until(async () => (await label.inputValue()) === '') && !('label' in (flagsFile() || {})));
  const sizes = page.getByLabel('Sizes');
  await sizes.fill('1, 2 ,3');
  await sizes.press('Enter');
  check('a list of numbers is saved without the spaces', await until(async () => flagsFile()?.sizes === '1,2,3' && (await sizes.inputValue()) === '1,2,3'), JSON.stringify(flagsFile()));
  await sizes.fill('1,0');
  await sizes.press('Enter');
  check('and refused when one is out of range', await until(async () => (await sizes.inputValue()) === '1,2,3'));
  await page.screenshot({ path: path.join(out, 'settings-flags-types.png') });
  await sizes.fill('');
  await sizes.press('Enter');
  await until(() => JSON.stringify(flagsFile()) === '{}');

  // Keyboard.
  await page.getByRole('button', { name: 'Keyboard' }).click();
  // The list only shows; clicking an action opens its editor.
  const openRow = (title) => page.locator('button.action').filter({ has: page.locator('.name', { hasText: new RegExp('^\\s*' + title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*$') }) }).click();
  check('a key the browser wins carries a tag, and a page-first key none', await until(async () => (await page.locator('button.action', { hasText: 'Close Tab' }).textContent()).includes('Browser · not full screen') && !(await page.locator('button.action', { hasText: 'Open Location' }).textContent()).includes('Browser')));
  check('the list has no buttons of its own: no plus, remove or reset', (await page.getByRole('button', { name: /^(Add a key for|Remove |Reset (?!all))/ }).count()) === 0);
  check('the keymap follows its template until an edit', keymapFile() === null);
  await openRow('New Tab');
  await page.getByRole('button', { name: 'Add key' }).click();
  await page.keyboard.press('Meta+Shift+KeyK');
  check('a recorded key is added, and the keymap becomes the user\'s', await until(() => {
    const k = keymapFile();
    return k && !k.follows && k.template === 'project-b' && k.actions['tab.new'].mac.some((b) => b.keys === 'Shift+Cmd+K');
  }), JSON.stringify(keymapFile()?.actions?.['tab.new']));
  check('the page shows it', await until(async () => (await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await openRow('Quit Project B');
  check('Quit\'s key is locked: no remove button', (await page.getByRole('button', { name: 'Remove Cmd+Q' }).count()) === 0 && (await page.getByText('Locked', { exact: true }).count()) === 1);
  check('Quit can\'t leave the menu', await page.getByRole('switch', { name: 'Quit Project B in the menu' }).isDisabled());

  await openRow('Reload This Page');
  await page.getByRole('switch', { name: 'Reload This Page in the menu', exact: true }).click();
  check('hiding an action takes it out of the menu', await until(async () => !menuHas((await regions()).menu, 'Reload This Page')));
  check('and the list marks it Hidden', await until(async () => (await page.locator('button.action', { hasText: 'Reload This Page' }).filter({ hasText: 'Hidden' }).count()) === 1));
  await page.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset puts it back', await until(async () => menuHas((await regions()).menu, 'Reload This Page')));

  await page.getByLabel('Template').selectOption('chrome');
  check('picking a template resets to following it', await until(() => keymapFile()?.follows === 'chrome'), JSON.stringify(keymapFile()));
  check('and the added key is gone', await until(async () => !(await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await page.screenshot({ path: path.join(out, 'settings-keyboard.png'), fullPage: true });

  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await regions()).settingsOpen === false));
  check('and the dev window takes it off the shell', await until(async () => (await shell.locator('iframe.mock-settings').count()) === 0));
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
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
