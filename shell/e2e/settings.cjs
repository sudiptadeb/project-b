// The settings panel on the mock engine, in a real browser: how it opens and closes, that
// it is shell over the page, what it shows, and that its changes go through Go.
// Run: node shell/e2e/settings.cjs   (from the repo root, after `npm run build` in shell/)

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

const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
const settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

(async () => {
  await new Promise((r) => releaseServer.listen(releasePort, '127.0.0.1', r));
  await startServer();
  const browser = await chromium.launch();
  const errors = [];
  const shell = await browser.newPage({ viewport: { width: 1100, height: 700 } });
  shell.on('pageerror', (e) => errors.push(e.message));
  shell.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  await shell.goto(base + '/');
  const mock = shell.frameLocator('iframe');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(shell);

  const gear = shell.getByRole('button', { name: 'Settings', exact: true });
  const panel = shell.getByRole('dialog', { name: 'Settings' });
  const panelRect = () => panel.evaluate((e) => { const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, w: r.width, h: r.height }; });
  const shellRegions = async () => (await regions()).regions.length;
  const closed = shell.getByRole('button', { name: 'Close settings' });
  const before = await shellRegions();

  // The gear opens the panel over the page, and the panel tells Go it is shell.
  await gear.click();
  await panel.waitFor();
  await settle(shell);
  check('the gear opens the settings panel', (await panel.count()) === 1 && (await gear.getAttribute('aria-expanded')) === 'true');
  check('the panel has the keyboard, so Escape reaches it', await panel.evaluate((e) => e === document.activeElement));
  const rect = await panelRect();
  const page = (await regions()).bounds;
  check('it sits inside the page area, under the gear', rect.x > page.x && rect.y > page.y && rect.x + rect.w < page.x + page.w && rect.y + rect.h < page.y + page.h, JSON.stringify([rect, page]));
  let r = await regions();
  check('it reports its rectangle as a shell region', r.regions.length === before + 1 && r.regions.some((g) => g.x === rect.x && g.y === rect.y && g.w === rect.w && g.h === rect.h), JSON.stringify(r.regions));
  r = await regions(`?x=${rect.x + rect.w / 2}&y=${rect.y + rect.h / 2}`);
  check('so a click inside it goes to the shell, not the page', r.hit === 'shell', r.hit);
  r = await regions(`?x=${page.x + 40}&y=${page.y + page.h - 40}`);
  check('and the page beside it still gets its own clicks', r.hit === 'page', r.hit);
  await shell.screenshot({ path: path.join(out, 'settings-panel-vertical.png') });

  // It shows what Go holds.
  const radio = (name) => panel.getByRole('radio', { name });
  check('it shows the current layout', (await radio('Vertical').isChecked()) && !(await radio('Horizontal').isChecked()));
  check('it shows the running version', (await panel.locator('.version').textContent()) === '0.2.0', await panel.locator('.version').textContent());
  const autoUpdate = panel.getByRole('switch', { name: 'Automatic updates' });
  check('it shows automatic updates on, the default', await autoUpdate.isChecked());
  const switchAttrs = await autoUpdate.evaluate((e) => [e.type, e.getAttribute('role'), e.getAttribute('aria-checked')].join(' '));
  check('the control is a checkbox with role="switch" and aria-checked', switchAttrs === 'checkbox switch true', switchAttrs);
  const edges = await panel.evaluate((p) => {
    const box = (e) => e.getBoundingClientRect();
    const sw = box(p.querySelector('[role=switch]'));
    const label = box(p.querySelector('[role=switch]').closest('.row').querySelector('.label'));
    const button = box([...p.querySelectorAll('button')].find((b) => b.textContent === 'Check for updates'));
    return [sw.right, button.right, sw.top + sw.height / 2, label.top + label.height / 2];
  });
  check('the switch sits at the right edge, under the button, level with its label', edges[0] === edges[1] && edges[2] === edges[3], JSON.stringify(edges));

  // One panel only: the gear again closes it, and so do Escape and the close button.
  await gear.click();
  await settle(shell);
  check('the gear again closes it', (await panel.count()) === 0 && (await gear.getAttribute('aria-expanded')) === 'false');
  check('and its region goes with it', (await shellRegions()) === before, await shellRegions());
  await gear.click();
  await panel.waitFor();
  check('it opens once, never twice', (await panel.count()) === 1);
  await shell.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await panel.count()) === 0));
  await gear.click();
  await panel.waitFor();
  await closed.click();
  check('the close button closes it', await until(async () => (await panel.count()) === 0));
  await gear.click();
  await panel.waitFor();

  // Changing the layout there goes through the prefs manager: saved, and the shell follows;
  // the panel stays open, now under the gear in the other layout.
  await radio('Horizontal').check();
  await shell.locator('.strip').waitFor();
  check('choosing Horizontal there switches the shell', true);
  check('and the prefs manager saved it', prefsFile().tabLayout === 'horizontal', JSON.stringify(prefsFile()));
  check('the panel stays open, showing what Go now holds', (await panel.count()) === 1 && (await until(async () => await radio('Horizontal').isChecked())));
  await settle(shell);
  const gearBox = await gear.evaluate((e) => { const b = e.getBoundingClientRect(); return [b.right, b.bottom]; });
  const moved = await panelRect();
  check('with the tabs on top it still sits under the gear', moved.y > gearBox[1] && Math.abs(moved.x + moved.w - gearBox[0]) < 16, JSON.stringify([moved, gearBox]));
  r = await regions();
  check('and reports where it moved to', r.regions.some((g) => g.x === moved.x && g.y === moved.y && g.w === moved.w && g.h === moved.h), JSON.stringify(r.regions));
  await shell.screenshot({ path: path.join(out, 'settings-panel-horizontal.png') });
  await shell.getByRole('button', { name: 'Tabs on the side' }).click();
  await shell.locator('nav').waitFor();
  check('a change from the toolbar reaches the panel', await until(async () => await radio('Vertical').isChecked()));

  // Colours. The default is the dusty teal, marked; a preset goes to Go, which saves it,
  // paints the window and tells the shell, whose tones follow.
  const root = (name) => shell.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);
  const swatchOf = (name) => panel.getByRole('radio', { name });
  check('the frame colour starts as the dusty teal, marked', (await swatchOf('Dusty teal').getAttribute('aria-checked')) === 'true' && (await root('--frame')) === '#4f9e9e' && prefsFile().frameColor === '#4f9e9e', JSON.stringify(prefsFile()));
  check('six presets and a custom colour', (await panel.locator('.swatch[role=radio]').count()) === 6 && (await panel.getByLabel('Custom colour').count()) === 1);
  const barBefore = await root('--bar');
  for (const [name, color, file] of [['Blue', '#3470d0', 'blue'], ['Amber', '#faa804', 'amber']]) {
    await swatchOf(name).click();
    const ok = await until(async () => (await root('--frame')) === color && (await regions()).windowColor === color && prefsFile().frameColor === color);
    check(`${name} recolours the window and the shell, and is saved`, ok, `${await root('--frame')} ${(await regions()).windowColor} ${prefsFile().frameColor}`);
    check(`${name} is the one marked`, (await swatchOf(name).getAttribute('aria-checked')) === 'true' && (await panel.locator('.swatch[aria-checked=true]').count()) === 1);
    await settle(shell);
    await shell.screenshot({ path: path.join(out, `settings-colour-${file}.png`) });
  }
  check('the bar tint follows the frame colour', (await root('--bar')) !== barBefore, await root('--bar'));
  // A custom colour from the picker goes the same way, and no preset is marked.
  await panel.getByLabel('Custom colour').evaluate((e) => { e.value = '#7a5c99'; e.dispatchEvent(new Event('input', { bubbles: true })); });
  check('a custom colour reaches Go and the shell', await until(async () => (await root('--frame')) === '#7a5c99' && (await regions()).windowColor === '#7a5c99' && prefsFile().frameColor === '#7a5c99'), await root('--frame'));
  check('and no preset is marked', (await panel.locator('.swatch[aria-checked=true]').count()) === 0);
  await swatchOf('Dusty teal').click();
  await until(async () => (await root('--frame')) === '#4f9e9e');

  // Updates. A check asked for goes to the update manager, which says what it found. The
  // manager's own check two seconds after launch comes first, so it isn't counted here.
  const status = panel.locator('.status');
  await until(() => manifests > 0);
  const counted = manifests;
  await panel.getByRole('button', { name: 'Check for updates' }).click();
  check('Check asks the update manager, which says it is up to date', await until(async () => (await status.textContent()) === 'Up to date'), await status.textContent());
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);

  // Automatic updates off, from the keyboard: saved, and the manager's timer stops. It stays
  // off after a restart; a check asked for still runs; on again, the timer starts as at launch.
  await panel.getByRole('button', { name: 'Check for updates' }).focus();
  await shell.keyboard.press('Tab');
  const ring = await autoUpdate.evaluate((e) => [e === document.activeElement, e.matches(':focus-visible'), getComputedStyle(e).boxShadow !== 'none'].join(' '));
  check('Tab reaches the switch and shows a focus ring', ring === 'true true true', ring);
  await shell.keyboard.press('Space');
  check('Space turns automatic updates off and saves it', await until(() => prefsFile().autoUpdate === false), JSON.stringify(prefsFile()));
  check('and the switch says it is off', await until(async () => (await autoUpdate.getAttribute('aria-checked')) === 'false' && !(await autoUpdate.isChecked())));
  await stopServer();
  await startServer();
  const atStart = manifests;
  await shell.goto(base + '/');
  await mock.getByRole('button', { name: 'Navigate' }).waitFor();
  errors.length = 0; // Stopping the server broke the old page's event stream, as it should.
  check('the panel starts closed', (await panel.count()) === 0);
  check('the frame colour survived the restart', (await (await regions()).windowColor) === '#4f9e9e' && (await root('--frame')) === '#4f9e9e');
  await gear.click();
  await panel.waitFor();
  check('after a restart the switch is still off', !(await autoUpdate.isChecked()) && (await autoUpdate.getAttribute('aria-checked')) === 'false');
  await sleep(3500);
  check('and the update manager made no check of its own', manifests === atStart, `${atStart} -> ${manifests}`);
  await panel.getByRole('button', { name: 'Check for updates' }).click();
  check('a check asked for still runs', await until(async () => manifests === atStart + 1 && (await status.textContent()) === 'Up to date'), `${manifests} ${await status.textContent()}`);
  await autoUpdate.check();
  check('turning it on again with a click saves it', await until(() => prefsFile().autoUpdate === true));
  check('and the switch says it is on', await until(async () => (await autoUpdate.getAttribute('aria-checked')) === 'true'));
  check('and the timer checks again, as at launch', await until(() => manifests === atStart + 2, 4000), `${atStart} -> ${manifests}`);

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
