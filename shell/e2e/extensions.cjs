// Extensions on the mock engine, in a real browser: the settings page installs from the
// test extension's package, loads an unpacked one, turns them off and on and removes them; the toolbar's
// puzzle menu pins a button, which shows a badge and opens its popup where it sits. Last,
// a build with WITHOUT=extensions has none of this in it.
// Run: node shell/e2e/extensions.cjs   (from the repo root, after `npm run build` in shell/, and tools/fetch-extensions.sh and tools/pack-test-extension.py for the store)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
const port = 8811;
const base = `http://127.0.0.1:${port}`;
const storeDir = path.join(root, '.tools/extensions');
const unpacked = path.join(root, 'tools/test-extension');
// The same extension as a package, as tools/pack-test-extension.py packs it.
const packaged = path.join(storeDir, 'crx/oepjjojhhaclfhmopocgmdfbeiffpheh_1.0.crx');

// The test extension, as tools/pack-test-extension.py packs it into the store.
const id = 'oepjjojhhaclfhmopocgmdfbeiffpheh';
const name = 'Project B test extension';

function cannotRun(why) {
  console.error(why);
  process.exit(2);
}
if (!fs.existsSync(path.join(storeDir, 'catalogue.json'))) {
  cannotRun('No mock store: run tools/fetch-extensions.sh and tools/pack-test-extension.py first.');
}
if (!fs.readFileSync(path.join(root, 'shell/dist/shell/shell.js'), 'utf8').includes('pbx-')) {
  cannotRun('shell/dist was built without extensions: run `npm run build` in shell/ without WITHOUT=extensions first.');
}

fs.mkdirSync(out, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-extensions-'));
// The Extensions flag, as the settings page saves it.
const setFlag = (on) => fs.writeFileSync(path.join(profile, 'flags.json'), JSON.stringify(on ? { extensions: 'on' } : {}));
setFlag(true);
const bin = path.join(out, 'dev-extensions');
execFileSync('go', ['build', '-tags', 'extensions', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

let server;
async function startServers() {
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`], { stdio: 'ignore' });
  for (const url of [base + '/mock/regions']) {
    for (let i = 0; ; i++) {
      try {
        await fetch(url);
        break;
      } catch {
        if (i === 100) throw new Error(`${url} did not start`);
        await new Promise((r) => setTimeout(r, 50));
      }
    }
  }
}
async function stopServers() {
  for (const p of [server]) {
    const gone = new Promise((r) => p.once('exit', r));
    p.kill('SIGTERM');
    await gone;
  }
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
const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
const runtime = async () => (await fetch(`${base}/mock/extensions`)).json();
const fire = (kind, extension, value) => fetch(`${base}/mock/extensions/fire`, { method: 'POST', body: new URLSearchParams({ kind, extension, value }) });
const call = async (extension, fn, args) =>
  (await fetch(`${base}/mock/extensions/call?extension=${extension}&name=${fn}&tab=0`, { method: 'POST', body: JSON.stringify(args) })).json();
const rectOf = (locator) => locator.evaluate((el) => {
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
});
const same = (a, b) => !!a && !!b && ['x', 'y', 'w', 'h'].every((k) => Math.abs(a[k] - b[k]) < 0.5);

(async () => {
  await startServers();
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

  // The mock has no window to put the settings page over, so the test opens /settings
  // itself once Go has asked for it.
  const open = async () => {
    const p = await browser.newPage({ viewport: { width: 1100, height: 800 } });
    watch(p);
    await p.goto(base + '/settings/');
    await p.getByRole('heading', { name: 'Settings' }).waitFor();
    return p;
  };
  await shell.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  let page = await open();

  await page.getByRole('button', { name: 'Extensions' }).click();
  let installed = page.getByRole('region', { name: 'Installed' });
  const storeRow = page.getByRole('region', { name: 'Store' });
  check('settings shows the Extensions section', await installed.getByText('No extensions installed').isVisible() && await storeRow.getByText('Chrome Web Store').isVisible());

  // The store is a website, which opens in a tab.
  await storeRow.getByRole('button', { name: 'Open' }).click();
  check('the store opens in a tab, from under the settings page', await until(async () => (await regions()).settingsOpen === false && (await shell.getByRole('tab').count()) === 2));
  await shell.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  page = await open();
  await page.getByRole('button', { name: 'Extensions' }).click();
  installed = page.getByRole('region', { name: 'Installed' });

  // A package file, which the engine asks about before it installs.
  const pick = async () => {
    await fire('choose', '', packaged);
    await page.getByRole('button', { name: 'Install package' }).click();
  };
  const dialog = page.getByRole('dialog');
  await pick();
  check('Install package asks first, with what it can do', await until(async () => (await dialog.count()) === 1 && (await dialog.textContent()).includes(`Add "${name}"?`)));
  const asked = await dialog.locator('li').allTextContents();
  check('the prompt lists the permissions', asked.includes('storage') && asked.includes('tabs'), asked.join(', '));
  check('the shell has the prompt too, for an install a store\'s page begins', await until(async () => (await shell.getByRole('dialog').count()) === 1));
  await page.keyboard.press('Escape');
  check('Escape answers Cancel', await until(async () => (await dialog.count()) === 0));
  await sleep(200);
  check('and leaves the settings page open', (await regions()).settingsOpen === true);
  await pick();
  await dialog.waitFor();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel installs nothing', await until(async () => (await dialog.count()) === 0) && await installed.getByText('No extensions installed').isVisible());

  // The shell's copy answers as the settings page's does.
  await pick();
  await shell.getByRole('dialog').getByRole('button', { name: 'Add extension' }).click();
  const rows = installed.getByRole('group', { name });
  check('Add extension, in the shell, installs it', await until(async () => (await rows.count()) === 1 && (await shell.getByRole('dialog').count()) === 0));
  await rows.first().getByRole('button', { name: 'Details' }).click();
  const details = page.getByRole('definition');
  check('Details shows its ID, source and toolbar button', (await details.allTextContents()).join('|').startsWith(`${id}|1.0|Package|`) && (await page.locator('.pbx-details').textContent()).includes('Yes'), (await details.allTextContents()).join('|'));
  await rows.first().getByRole('button', { name: 'Details' }).click();

  await fire('choose', '', unpacked);
  await page.getByRole('button', { name: 'Load unpacked' }).click();
  check('Load unpacked adds a second one', await until(async () => (await rows.count()) === 2));
  await rows.nth(1).getByRole('button', { name: 'Details' }).click();
  check('which says where it was loaded from', (await page.locator('.pbx-details').textContent()).includes(`Unpacked, from ${unpacked}`), await page.locator('.pbx-details').textContent());
  await page.screenshot({ path: path.join(out, 'extensions-settings.png'), fullPage: true });
  await rows.nth(1).getByRole('button', { name: `Remove ${name}` }).click();
  check('Remove takes the unpacked one away', await until(async () => (await rows.count()) === 1));

  const enabled = rows.first().getByRole('switch');
  await enabled.click();
  check('the switch turns it off', await until(async () => !(await enabled.isChecked())));
  const puzzle = shell.getByRole('button', { name: 'Extensions', exact: true });
  const menu = shell.getByRole('group', { name: 'Extensions menu' });
  await puzzle.click();
  check('an extension that is off is not in the menu', await until(async () => (await menu.textContent()).includes('No extensions installed')));
  await puzzle.click();
  check('the puzzle piece closes its menu', await until(async () => (await menu.count()) === 0));
  await enabled.click();
  check('and on again', await until(async () => await enabled.isChecked()));

  // The toolbar.
  check('the shell shows the puzzle piece', await puzzle.isVisible());
  await puzzle.click();
  check('its menu lists the extension', await until(async () => (await menu.locator('.pbx-row').count()) === 1 && (await menu.textContent()).includes(name)));
  const m = await rectOf(menu);
  check('the menu is shell, over the page', await until(async () => (await regions(`?x=${m.x + m.w / 2}&y=${m.y + m.h / 2}`)).hit === 'shell'));
  await shell.screenshot({ path: path.join(out, 'extensions-menu.png') });
  await menu.getByRole('button', { name: `Pin ${name}` }).click();
  const button = shell.locator('.pbx-action');
  check('pin puts its button on the toolbar', await until(async () => (await button.count()) === 1 && (await button.getAttribute('aria-label')) === name));
  check('and the menu shows it pinned', await until(async () => (await menu.getByRole('button', { name: `Unpin ${name}` }).count()) === 1));
  await shell.keyboard.press('Escape');
  check('Escape closes the menu', await until(async () => (await menu.count()) === 0));

  const set = await call(id, 'action.setBadgeText', [{ text: '7' }]);
  check('a badge the extension sets shows on its button', await until(async () => (await button.locator('.pbx-badge').count()) === 1 && (await button.locator('.pbx-badge').textContent()) === '7'), JSON.stringify(set));

  await button.click();
  const anchor = await rectOf(button);
  check('clicking the button opens its popup under it', await until(async () => {
    const p = (await runtime()).popup;
    return p && p.extension === id && same(p.anchor, anchor);
  }), JSON.stringify((await runtime()).popup) + ' vs ' + JSON.stringify(anchor));
  check('and it looks pressed while the popup is open', await until(async () => (await button.getAttribute('class')).includes('pbx-open')));
  await shell.screenshot({ path: path.join(out, 'extensions-toolbar.png') });
  await fire('closePopup', '', '');
  check('closing the popup lets the button go', await until(async () => !(await button.getAttribute('class')).includes('pbx-open') && (await runtime()).popup === null));

  // An unpinned extension's popup opens under the puzzle piece.
  await puzzle.click();
  await menu.getByRole('button', { name: `Unpin ${name}` }).click();
  check('unpin takes the button off the toolbar', await until(async () => (await button.count()) === 0));
  await menu.locator('.pbx-item', { hasText: name }).click();
  const puzzleRect = await rectOf(puzzle);
  check('a click in the menu opens the popup under the puzzle piece', await until(async () => {
    const p = (await runtime()).popup;
    return p && p.extension === id && same(p.anchor, puzzleRect);
  }), JSON.stringify((await runtime()).popup));
  check('and closes the menu', (await menu.count()) === 0);
  await fire('closePopup', '', '');
  await until(async () => (await runtime()).popup === null);
  await puzzle.click();
  await shell.getByRole('combobox', { name: 'Address' }).click();
  check('a click elsewhere in the shell closes the menu', await until(async () => (await menu.count()) === 0));

  // Manage extensions opens the settings page at its Extensions section, from closed and
  // from another section.
  await page.getByRole('button', { name: 'Close settings' }).click();
  await until(async () => (await regions()).settingsOpen === false);
  await page.close();
  await puzzle.click();
  await menu.getByRole('button', { name: 'Manage extensions' }).click();
  check('Manage extensions opens the settings page', await until(async () => (await regions()).settingsOpen === true));
  page = await open();
  const nav = (label) => page.getByRole('navigation').getByRole('button', { name: label });
  check('at its Extensions section', (await nav('Extensions').getAttribute('aria-current')) === 'page' && await page.getByRole('region', { name: 'Installed' }).isVisible());
  await nav('General').click();
  await puzzle.click();
  await menu.getByRole('button', { name: 'Manage extensions' }).click();
  check('and goes back there when it is already open', await until(async () => (await nav('Extensions').getAttribute('aria-current')) === 'page'));

  await page.getByRole('region', { name: 'Installed' }).getByRole('button', { name: `Remove ${name}` }).click();
  check('Remove takes it away', await until(async () => await page.getByRole('region', { name: 'Installed' }).getByText('No extensions installed').isVisible()));
  await puzzle.click();
  check('and from the menu', await until(async () => (await menu.textContent()).includes('No extensions installed')));

  // With the flag off, the build's extensions don't start. The server's restart cuts the
  // pages' event streams, which is no error of theirs.
  const before = errors.length;
  await page.close();
  await stopServers();
  setFlag(false);
  await startServers();
  await shell.goto(base + '/');
  await shell.frameLocator('iframe').getByRole('button', { name: 'Navigate' }).waitFor();
  errors.splice(before);
  check('with the flag off the shell has no extensions toolbar', (await shell.locator('.pbx-toolbar').count()) === 0);
  await shell.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  page = await open();
  check('and settings no Extensions section', (await page.getByRole('navigation').getByRole('button', { name: 'Extensions' }).count()) === 0);
  await page.getByRole('button', { name: 'Flags' }).click();
  check('but the flag is there to turn on', await page.getByRole('switch', { name: /Extensions/ }).isEnabled());

  check('no errors in the pages', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServers();
  fs.rmSync(profile, { recursive: true, force: true });

  // A build with WITHOUT=extensions has none of the extensions UI in it.
  const off = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-no-extensions-'));
  const env = { ...process.env, WITHOUT: 'extensions' };
  const shellDir = path.join(root, 'shell');
  for (const app of fs.readdirSync(path.join(shellDir, 'src/apps'))) {
    execFileSync('npx', ['vite', 'build', '--logLevel', 'error', '--outDir', path.join(off, app)], { cwd: shellDir, env: { ...env, VITE_APP: app }, stdio: 'inherit' });
  }
  const leaked = fs.readdirSync(off, { recursive: true })
    .filter((f) => fs.statSync(path.join(off, f)).isFile() && fs.readFileSync(path.join(off, f), 'utf8').includes('pbx-'));
  check('a build without extensions has no extensions UI', leaked.length === 0, leaked.join(', '));
  fs.rmSync(off, { recursive: true, force: true });

  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})().catch(async (err) => {
  console.error(err);
  server?.kill('SIGTERM');
  process.exit(1);
});
