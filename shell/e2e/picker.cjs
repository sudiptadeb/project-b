// The picker page on the mock engine, in a real browser: a copy of the dev server started
// to pick (-picker) shows the profiles in the list with their colours, and New Profile
// makes one from a template and opens it, in a copy of its own, as choosing a profile
// does, and this copy quits. The mock starts no copies: it names the profile it opened.
// Run: node shell/e2e/picker.cjs   (from the repo root, after `npm run build` in shell/)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
const port = 8794;
const base = `http://127.0.0.1:${port}`;

fs.mkdirSync(out, { recursive: true });
const profiles = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-picker-'));
fs.writeFileSync(path.join(profiles, 'profiles.json'), JSON.stringify({
  profiles: [
    { id: 'default', name: 'Personal', color: '#4f9e9e', template: 'Personal' },
    { id: 'a1b2c3d4', name: 'Work', color: '#5b7fd6', template: 'Work' },
  ],
}));
const listed = () => JSON.parse(fs.readFileSync(path.join(profiles, 'profiles.json'), 'utf8')).profiles;
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

let server;
async function startServer() {
  // A server left over from another run would answer in ours' place.
  if (await fetch(base + '/mock/regions').then(() => true, () => false)) {
    throw new Error(`port ${port} already has a server: stop it and run again`);
  }
  server = spawn(bin, [`-addr=127.0.0.1:${port}`, `-shell=${path.join(root, 'shell/dist')}`, `-root=${profiles}`, '-picker'], { stdio: 'ignore' });
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
const regions = async () => (await fetch(`${base}/mock/regions`)).json();

(async () => {
  await startServer();
  const browser = await chromium.launch();
  const errors = [];
  const page = await browser.newPage({ viewport: { width: 900, height: 600 } });
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  await page.goto(base + '/picker/');

  const rows = page.getByRole('list', { name: 'Profiles' }).getByRole('button');
  check('the picker lists the profiles', await until(async () => (await rows.allTextContents()).map((t) => t.trim()).join(',') === 'Personal,Work'));
  const dots = await page.locator('.dot').evaluateAll((els) => els.map((e) => getComputedStyle(e).backgroundColor));
  check('each in its colour', dots.join(' ') === 'rgb(79, 158, 158) rgb(91, 127, 214)', dots.join(' '));
  let r = await regions();
  check('in a window of its own, the picker page covering it', r.page === 'picker' && r.regions.length === 1 && r.drag.length === 1, JSON.stringify([r.page, r.regions, r.drag]));
  check('and the Dock icon is the plain one', r.dock === '', r.dock);
  await page.screenshot({ path: path.join(out, 'picker.png') });

  await page.getByRole('button', { name: 'New Profile' }).click();
  const templates = await page.getByLabel('Template').locator('option').allTextContents();
  check('New Profile offers the templates', templates.join(',') === 'Personal,Work', templates.join(','));
  check('and keeps Create off until there is a name', await page.getByRole('button', { name: 'Create' }).isDisabled());
  await page.getByLabel('Name').fill('Office');
  await page.getByLabel('Template').selectOption('Work');
  await page.screenshot({ path: path.join(out, 'picker-new.png') });
  // Create's words read on its fill, light and dark, as the system has it.
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await until(async () => (await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)) === scheme);
    const [fg, bg] = await page.getByRole('button', { name: 'Create' }).evaluate((b) => [getComputedStyle(b).color, getComputedStyle(b).backgroundColor]);
    const ratio = contrast(fg, bg);
    check(`Create reads in ${scheme}: ${ratio.toFixed(2)}`, ratio >= 4.5, `${fg} on ${bg}`);
  }
  await page.getByRole('button', { name: 'Create' }).click();
  check('Create adds the profile to the list, in its template\'s colour', await until(() => listed().some((p) => p.name === 'Office' && p.color === '#5b7fd6' && p.template === 'Work')), JSON.stringify(listed()));
  const made = listed().find((p) => p.name === 'Office');
  check('with its prefs from the template', made && JSON.parse(fs.readFileSync(path.join(profiles, made.id, 'prefs.json'), 'utf8')).tabLayout === 'horizontal');
  check('and opens it in a copy of its own, and this copy quits', await until(async () => {
    r = await regions();
    return r.opened === made.id && r.quit;
  }), JSON.stringify([r.opened, r.quit]));

  check('no errors in the page', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})();
