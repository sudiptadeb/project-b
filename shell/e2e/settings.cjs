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
// The names of the shells the user can choose, as the registry lists them.
const listedShells = JSON.parse(fs.readFileSync(path.join(root, 'core/shells/registry.json'), 'utf8')).filter((s) => !s.example).map((s) => s.name);
const port = 8797;
const base = `http://127.0.0.1:${port}`;

fs.mkdirSync(out, { recursive: true });
// A listed profile, as a copy started on one has: its folder in a root beside the list.
const profilesRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-settings-'));
const profileID = 'a1b2c3d4';
const profile = path.join(profilesRoot, profileID);
fs.mkdirSync(profile);
fs.writeFileSync(path.join(profilesRoot, 'profiles.json'), JSON.stringify({ profiles: [{ id: profileID, name: 'Personal', color: '#4f9e9e', template: 'Personal' }] }));
const listed = () => JSON.parse(fs.readFileSync(path.join(profilesRoot, 'profiles.json'), 'utf8')).profiles[0];
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
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, `-root=${profilesRoot}`, `-id=${profileID}`, `-flags=${moreFlags}`,
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
const updateSettings = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(profilesRoot, 'update/settings.json'), 'utf8'));
  } catch {
    return null;
  }
};
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
// WCAG 2's contrast ratio of two computed colours, rgb(r, g, b).
function contrast(a, b) {
  const luminance = (c) => {
    const [r, g, bl] = c.match(/\d+/g).slice(0, 3).map((v) => {
      const s = v / 255;
      return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
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
  const shellFrame = () => shell.frame({ url: /\/shell\/standard\/\?window=1$/ });
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
  check('a frame colour recolours the window and the shell, and is saved', await until(async () => (await root('--frame')) === '#5b7fd6' && (await regions()).windowColor === '#5b7fd6' && prefsFile().frameColor === '#5b7fd6'));
  const swatch = page.getByRole('radio', { name: 'Blue' });
  await swatch.press('ArrowRight');
  check('an arrow key moves the colour, and focus with it', await until(async () => prefsFile().frameColor === '#faa804' && (await page.getByRole('radio', { name: 'Amber' }).evaluate((e) => e === document.activeElement))));
  await page.getByRole('radio', { name: 'Blue' }).click();
  await page.getByRole('button', { name: 'Check for updates' }).click();
  const updateStatus = page.getByRole('region', { name: 'Updates' }).getByRole('status');
  check('Check asks the update manager, which says it is up to date', await until(async () => (await updateStatus.textContent()).trim() === 'Up to date'));
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);
  await autoUpdate.click();
  check('the switch turns automatic updates off and saves it in the root\'s update folder, for every copy', await until(() => updateSettings()?.autoUpdate === false));

  // A dialog's answer reads on its fill in every preset colour, light and dark: its words
  // reach WCAG's 4.5 for text, by the colours the page computes. New workspace's dialog
  // has one.
  const presets = await page.getByRole('radiogroup', { name: 'Frame colour' }).getByRole('radio')
    .evaluateAll((els) => els.map((e) => ({ name: e.getAttribute('aria-label'), rgb: getComputedStyle(e).backgroundColor })));
  const toHex = (rgb) => '#' + rgb.match(/\d+/g).map((v) => Number(v).toString(16).padStart(2, '0')).join('');
  const rootOf = (p, name) => p.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);
  for (const theme of ['Light', 'Dark']) {
    for (const p of presets) {
      await page.getByRole('button', { name: 'General', exact: true }).click();
      await page.getByRole('radio', { name: theme }).check();
      await page.getByRole('radio', { name: p.name, exact: true }).click();
      const shown = await until(async () => (await rootOf(page, '--frame')) === toHex(p.rgb) && (await rootOf(page, '--text')) === (theme === 'Dark' ? '#e8eaed' : '#1f1f1f'));
      await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
      await page.getByRole('button', { name: 'New workspace…' }).click();
      const answer = page.getByRole('dialog').getByRole('button', { name: 'Create', exact: true });
      const [fg, bg] = await answer.evaluate((b) => [getComputedStyle(b).color, getComputedStyle(b).backgroundColor]);
      const ratio = contrast(fg, bg);
      check(`a primary button reads in ${p.name}, ${theme.toLowerCase()}: ${ratio.toFixed(2)}`, shown && ratio >= 4.5, `${fg} on ${bg}`);
      await page.keyboard.press('Escape');
    }
  }
  await page.getByRole('button', { name: 'General', exact: true }).click();
  await page.getByRole('radio', { name: 'System' }).check();
  await page.getByRole('radio', { name: 'Slate' }).click();
  await until(() => prefsFile().theme === 'system' && prefsFile().frameColor === '#384562');

  // Profile: the name in the list, and reset to a template, which takes its values and
  // nothing else.
  await page.getByRole('button', { name: 'Profile', exact: true }).click();
  const profileSection = page.locator('main');
  const nameField = page.getByLabel('Name', { exact: true });
  const templateNow = () => profileSection.locator('.row', { hasText: 'Template' }).locator('.secondary').textContent();
  check('Profile shows the name and the template', (await nameField.inputValue()) === 'Personal' && (await templateNow()).trim() === 'Personal');
  await nameField.fill('');
  await nameField.press('Enter');
  check('a blank name is refused under the field, and nothing is saved', await until(async () => (await page.getByRole('alert').textContent()) === 'A profile needs a name') && listed().name === 'Personal');
  await nameField.press('Escape');
  check('Escape puts the name back, and the page stays open', await until(async () => (await nameField.inputValue()) === 'Personal') && (await regions()).settingsOpen === true);
  await nameField.fill('Home');
  await nameField.press('Enter');
  check('a new name is the list\'s', await until(() => listed().name === 'Home'), JSON.stringify(listed()));
  // The shell it loads: the ones the user can choose, the example not among them. A choice
  // is the profile's pref, and every window loads that shell at once.
  const shellChoice = page.getByLabel('Shell', { exact: true });
  check('Profile shows the shell in use, and offers only the listed shells', (await shellChoice.inputValue()) === 'standard' &&
    JSON.stringify(await shellChoice.locator('option').allTextContents()) === JSON.stringify(listedShells));
  await fetch(`${base}/settings/send`, { method: 'POST', body: JSON.stringify({ t: 'setShell', text: 'example' }) });
  check('choosing a shell saves it and the window loads it', await until(async () => prefsFile().shell === 'example' && (await regions()).shell === 'example'));
  await shellChoice.selectOption('standard');
  check('and choosing the standard one loads it again', await until(async () => prefsFile().shell === 'standard' && (await regions()).shell === 'standard' && (await ui.getByRole('button', { name: 'Settings', exact: true }).count()) === 1));
  const resetButton = page.getByRole('button', { name: 'Reset to template…' });
  await resetButton.click();
  const confirm = page.getByRole('dialog', { name: "Reset this profile's settings?" });
  check('reset asks first, with the template to reset to in the question', await confirm.isVisible());
  await confirm.getByLabel('Template').selectOption('Work');
  await page.screenshot({ path: path.join(out, 'settings-profile-reset.png') });
  await page.keyboard.press('Escape');
  check('Escape answers Cancel, gives focus back and leaves the page open', await until(async () => (await confirm.count()) === 0) && (await regions()).settingsOpen === true && prefsFile().frameColor === '#384562' &&
    (await resetButton.evaluate((e) => e === document.activeElement)));
  await resetButton.click();
  await confirm.getByLabel('Template').selectOption('Work');
  await confirm.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset writes the template\'s values over the prefs', await until(() => prefsFile().frameColor === '#5b7fd6' && prefsFile().tabLayout === 'horizontal'), JSON.stringify(prefsFile()));
  check('the shell follows at once', await until(async () => (await root('--frame')) === '#5b7fd6' && (await ui.locator('.strip').count()) > 0));
  check('and the Dock and the list', await until(async () => (await regions()).dock === '#5b7fd6' && listed().color === '#5b7fd6'), JSON.stringify(listed()));
  check('the list and the page name the template reset to', await until(async () => listed().template === 'Work' && (await templateNow()).trim() === 'Work'));
  await settle(page);
  await page.screenshot({ path: path.join(out, 'settings-profile.png') });

  // Workspaces: the list, Main in a new profile, New Workspace with a name and a colour, and
  // rename. Go keeps them in the profile's folder, and the shell has them at once: a tab
  // opened in one, as Tab, New Tab in Workspace does, shows its dot, and a rename reaches it.
  const workspacesFile = () => {
    try {
      return JSON.parse(fs.readFileSync(path.join(profile, 'workspaces.json'), 'utf8')).workspaces;
    } catch {
      return [];
    }
  };
  await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
  const workspacesSection = page.getByRole('region', { name: 'Workspaces' });
  const mainRow = workspacesSection.locator('.row', { hasText: 'Main' });
  check('Workspaces lists Main at first, with the tab open in it',
    (await workspacesSection.locator('.row').count()) === 1 && (await mainRow.textContent()).includes('1 tab') && workspacesFile().length === 0);
  await page.getByRole('button', { name: 'New workspace…' }).click();
  const wsDialog = page.getByRole('dialog', { name: 'New workspace' });
  const wsName = wsDialog.getByLabel('Name', { exact: true });
  check('New workspace… opens a dialog with the name ready to type', await until(async () => await wsName.evaluate((e) => e === document.activeElement)));
  await wsDialog.getByLabel('Custom colour').fill('#c2410c');
  await wsName.fill('Work');
  await wsName.press('Enter');
  check('Create makes one with its name and colour, kept in the profile after Main', await until(() => workspacesFile().length === 2 && workspacesFile()[0].name === 'Main' &&
    workspacesFile()[1].name === 'Work' && workspacesFile()[1].color === '#c2410c'), JSON.stringify(workspacesFile()));
  check('and lists it as a row with its menu', await until(async () => (await page.getByRole('button', { name: 'Actions for Work' }).count()) === 1 && (await wsDialog.count()) === 0));
  await page.getByRole('button', { name: 'New workspace…' }).click();
  await wsName.fill('work');
  await wsName.press('Enter');
  check('a name the list has is refused under the field', await until(async () => (await wsDialog.getByRole('alert').textContent()).includes('already') && workspacesFile().length === 2));
  await page.keyboard.press('Escape');
  check('Escape cancels the dialog and leaves the page open', await until(async () => (await wsDialog.count()) === 0) && (await regions()).settingsOpen === true);
  const made = workspacesFile()[1].id;
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'menu', value: 'workspaces:' + made }) });
  const dots = ui.locator('[role=tab] .workspace');
  const dot = dots.nth(1);
  check('a tab opened in it shows its dot in the shell at once', await until(async () => (await dots.count()) === 2 && (await dot.getAttribute('aria-label')) === 'In Work' &&
    (await dot.evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(194, 65, 12)'));
  check('and the row counts it', await until(async () => (await workspacesSection.locator('.row', { hasText: 'Work' }).textContent()).includes('1 tab')));
  await workspacesSection.locator('.row', { hasText: 'Work' }).click({ button: 'right' });
  check('a right-click opens the row\'s menu', await until(async () => (await page.getByRole('menu', { name: 'Actions for Work' }).count()) === 1));
  await page.getByRole('menuitem', { name: 'Edit…' }).click();
  const editDialog = page.getByRole('dialog', { name: 'Edit workspace' });
  await editDialog.getByLabel('Name', { exact: true }).fill('Office');
  await editDialog.getByRole('button', { name: 'Save' }).click();
  check('Edit renames it, under the same id', await until(() => workspacesFile()[1].name === 'Office' && workspacesFile()[1].id === made), JSON.stringify(workspacesFile()));
  check('and the dot in the shell says it at once', await until(async () => (await dot.getAttribute('aria-label')) === 'In Office'));
  await page.getByRole('button', { name: 'Actions for Office' }).click();
  await page.getByRole('menuitem', { name: 'New tab in workspace' }).click();
  check('New tab in workspace opens a tab in it and closes the page', await until(async () => (await dots.count()) === 3 && (await regions()).settingsOpen === false));
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(async () => (await regions()).settingsOpen === true);
  await page.close();
  page = await open();
  await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
  await settle(page);
  await page.screenshot({ path: path.join(out, 'settings-workspaces.png') });

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
  check('Flags lists the flags, none waiting', await page.getByRole('region', { name: 'Flags' }).getByText('Chromium features', { exact: true }).isVisible() && (await relaunch.count()) === 0);
  check('Extensions is off and can\'t be turned on in a build without them', await page.getByRole('switch', { name: /Extensions/ }).isDisabled());
  check('and says so', (await page.locator('.row', { hasText: 'Extensions' }).textContent()).includes('Not in this build'));
  const port = page.getByLabel('Remote debugging port');
  await port.fill('80');
  await port.press('Enter');
  check('a port it can\'t use is refused under the field, and what was typed stays', await until(async () => (await port.inputValue()) === '80' && (await page.locator('.row', { has: port }).getByRole('alert').count()) === 1) && flagsFile() === null);
  await port.fill('9333');
  await port.press('Enter');
  check('a port is saved, and Relaunch shows', await until(async () => flagsFile()?.remoteDebugging === '9333' && await relaunch.isVisible()), JSON.stringify(flagsFile()));
  check('once, at the top, with the flag tagged', (await relaunch.count()) === 1 && (await page.getByText('Relaunch to apply 1 change').isVisible()) && (await page.locator('.row', { has: port }).getByText('Relaunch to apply').count()) === 1);
  check('each flag says what it does', await page.getByText('Let DevTools clients connect on this port').isVisible());
  await page.screenshot({ path: path.join(out, 'settings-flags.png') });
  await port.fill('');
  await port.press('Enter');
  check('cleared, nothing waits', await until(async () => (await relaunch.count()) === 0 && JSON.stringify(flagsFile()) === '{}'));

  // The other types.
  const mode = page.getByLabel('Mode');
  check('an enum is a select, Off first', (await mode.evaluate((s) => [...s.options].map((o) => o.text).join())) === 'Off,fast,slow');
  await mode.selectOption('slow');
  check('picking a choice saves it', await until(async () => flagsFile()?.mode === 'slow' && (await page.locator('.row', { has: mode }).getByText('Relaunch to apply').count()) === 1));
  await mode.selectOption('');
  const label = page.getByLabel('Label');
  await label.fill('Not Lower');
  await label.press('Enter');
  check('a string the pattern refuses is not saved, and says why', await until(async () => (await page.locator('.row', { has: label }).getByRole('alert').count()) === 1) && !('label' in (flagsFile() || {})));
  await label.press('Escape');
  const sizes = page.getByLabel('Sizes');
  await sizes.fill('1, 2 ,3');
  await sizes.press('Enter');
  check('a list of numbers is saved without the spaces', await until(async () => flagsFile()?.sizes === '1,2,3' && (await sizes.inputValue()) === '1,2,3'), JSON.stringify(flagsFile()));
  await sizes.fill('1,0');
  await sizes.press('Enter');
  check('and refused when one is out of range', await until(async () => (await page.locator('.row', { has: sizes }).getByRole('alert').count()) === 1) && flagsFile()?.sizes === '1,2,3');
  await sizes.press('Escape');
  await page.screenshot({ path: path.join(out, 'settings-flags-types.png') });
  await sizes.fill('');
  await sizes.press('Enter');
  await until(() => JSON.stringify(flagsFile()) === '{}');

  // Keyboard.
  await page.getByRole('button', { name: 'Keyboard' }).click();
  // The list only shows; clicking an action opens its editor.
  const openRow = (title) => page.locator('button.action').filter({ has: page.locator('.name', { hasText: new RegExp('^\\s*' + title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*$') }) }).click();
  check('a key the browser wins carries a tag, and a page-first key none', await until(async () => (await page.locator('button.action', { hasText: 'Close Tab' }).textContent()).includes('Browser first, not in full screen') && !(await page.locator('button.action', { hasText: 'Open Location' }).textContent()).includes('Browser first')));
  check('the list has no buttons of its own: no plus, remove or reset', (await page.getByRole('button', { name: /^(Add a key for|Remove |Reset (?!all))/ }).count()) === 0);
  check('the keymap follows its template until an edit', keymapFile() === null);
  check('a shell\'s own actions are listed with their keys, hidden from the menu', await until(async () => {
    const row = (name) => page.getByRole('region', { name: 'View › Terminal', exact: true }).locator('button.action', { hasText: name }).textContent();
    const right = await row('Split Right');
    const down = await row('Split Down');
    return right.includes('⌘D') && right.includes('Hidden') && down.includes('⇧⌘D') && down.includes('Hidden');
  }));
  await openRow('New Tab');
  await page.getByRole('button', { name: 'Add key' }).click();
  await page.keyboard.press('Meta+Shift+KeyK');
  check('a recorded key is added, and the keymap becomes the user\'s', await until(() => {
    const k = keymapFile();
    return k && !k.follows && k.template === 'project-b' && k.actions['tab.new'].mac.some((b) => b.keys === 'Shift+Cmd+K');
  }), JSON.stringify(keymapFile()?.actions?.['tab.new']));
  check('the page shows it', await until(async () => (await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await openRow('Quit Project B');
  check('Quit\'s key is locked: no remove button', (await page.getByRole('button', { name: 'Remove Cmd+Q' }).count()) === 0 && (await page.getByText('Locked', { exact: true }).count()) >= 1);
  check('Quit is always in the menu, said by a tag rather than a dead switch', (await page.getByRole('switch', { name: 'Quit Project B in the menu' }).count()) === 0 && (await page.getByText('Always in menu', { exact: true }).count()) === 1);

  await openRow('Reload This Page');
  await page.getByRole('switch', { name: 'Reload This Page in the menu', exact: true }).click();
  check('hiding an action takes it out of the menu', await until(async () => !menuHas((await regions()).menu, 'Reload This Page')));
  check('and the list marks it Hidden', await until(async () => (await page.locator('button.action', { hasText: 'Reload This Page' }).filter({ hasText: 'Hidden' }).count()) === 1));
  await page.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset puts it back', await until(async () => menuHas((await regions()).menu, 'Reload This Page')));

  await page.getByRole('combobox', { name: 'Template' }).selectOption('chrome');
  const discard = page.getByRole('dialog', { name: /^Discard 1 changed shortcut and use/ });
  check('picking a template with a changed shortcut asks first, saying how many', await until(async () => (await discard.count()) === 1) && keymapFile()?.follows === undefined);
  await page.keyboard.press('Escape');
  check('Escape keeps the shortcuts and the page', await until(async () => (await discard.count()) === 0) && keymapFile()?.follows === undefined && (await regions()).settingsOpen === true);
  await page.getByRole('combobox', { name: 'Template' }).selectOption('chrome');
  await discard.getByRole('button', { name: 'Discard' }).click();
  check('Discard resets to following the template', await until(() => keymapFile()?.follows === 'chrome'), JSON.stringify(keymapFile()));
  check('and the added key is gone', await until(async () => !(await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await page.screenshot({ path: path.join(out, 'settings-keyboard.png'), fullPage: true });

  await page.locator('body').click({ position: { x: 5, y: 300 } });
  await page.keyboard.press('Escape');
  check('Escape closes the open editor first, and leaves the page', await until(async () => (await page.locator('.editor').count()) === 0) && (await regions()).settingsOpen === true);
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
