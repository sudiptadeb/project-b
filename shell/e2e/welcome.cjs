// The welcome on the mock engine (docs/onboarding.md): with the flag Show the welcome at each
// start on, a start opens it over the whole window, Standard's bars hidden but for a strip at
// the top to drag the window by. A colour picked there is the profile's at once: the window,
// the page and the shell take it. Start browsing gives the window back to the shell and keeps
// that it was finished; Run setup again, in Settings, closes the app and the next
// start opens it over the whole window.
// Run: node shell/e2e/welcome.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

const flags = (value) => fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify(value));
const welcomeTab = (r) => (r.tabs || []).find((t) => t.page === 'welcome');
const done = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, 'welcome.json'), 'utf8')).done;
  } catch {
    return undefined;
  }
};

// What a window's shell shows of itself: a tab, or a field to type in, as Standard's empty
// window has.
const bars = async (w) => (await w.ui.getByRole('tab').first().isVisible()) || (await w.ui.getByRole('combobox').first().isVisible());

(async () => {
  const browser = await chromium.launch();

  flags({ showWelcome: 'on' });
  await lib.start(['-use=standard']);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const welcome = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  check('a start with the flag on opens the welcome, its tab the active one', await until(async () => {
    const r = await regions();
    return welcomeTab(r) && r.active === welcomeTab(r).id && r.visible.some((v) => v.tab === r.active);
  }));
  check('the shell draws no bars around it', await until(async () => !(await bars(w))));
  let r = await regions();
  check('a strip across the top drags the window', r.drag.length === 1 && r.drag[0].y === 0 && r.drag[0].h === 40 && (await regions({ x: 600, y: 20 })).hit === 'drag', JSON.stringify(r.drag));
  check('and the page has the rest of it', (await regions({ x: 600, y: 400 })).hit === 'page');
  check('welcome.json says it is not finished', done() === false, String(done()));
  await welcome.getByRole('button', { name: 'Get started' }).waitFor();
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-hello.png') });

  await welcome.getByRole('button', { name: 'Get started' }).click();
  // Bring your things over, the import wizard (shell/e2e/imports.cjs), then the style.
  await welcome.getByRole('heading', { name: 'Bring your things over' }).waitFor();
  await welcome.getByRole('button', { name: 'Continue' }).click();
  await welcome.getByRole('radio', { name: 'Blue' }).click();
  const rootOf = (name) => welcome.locator(':root').evaluate((el, n) => getComputedStyle(el).getPropertyValue(n).trim(), name);
  check('a colour picked is the window\'s and the page\'s at once', await until(async () => (await regions()).windowColor === '#5b7fd6' && (await rootOf('--frame')) === '#5b7fd6'));
  await welcome.getByRole('radio', { name: 'Tabs on top' }).click();
  check('Tabs on top is the profile\'s tab layout', await until(() => JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8')).tabLayout === 'horizontal'));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-style.png') });
  await welcome.getByRole('button', { name: 'Continue' }).click();

  // How you work: Personal and Work ticked, one added; the keys, Firefox's, with a few
  // shown. Continue makes the workspaces: Main becomes Personal.
  check('Continue goes on to How you work', await until(() => welcome.getByRole('heading', { name: 'How you work' }).isVisible()));
  const ticked = async (name) => welcome.getByRole('checkbox', { name, exact: true }).isChecked();
  check('Personal and Work are ticked, Reading isn\'t', (await ticked('Personal')) && (await ticked('Work')) && !(await ticked('Reading')));
  await welcome.getByRole('textbox', { name: 'Add a workspace' }).fill('Travel');
  await welcome.getByRole('textbox', { name: 'Add a workspace' }).press('Enter');
  check('one added is ticked', await until(() => ticked('Travel')));
  const keys = welcome.getByRole('list', { name: 'Keys' });
  check('Project B\'s keys are shown, Downloads on ⇧⌘J', await until(async () => (await keys.innerText()).includes('⇧⌘J')));
  await welcome.getByRole('radio', { name: 'Like Firefox' }).click();
  const keymap = () => { try { return JSON.parse(fs.readFileSync(path.join(lib.profile, 'keymap.json'), 'utf8')); } catch { return {}; } };
  check('Like Firefox makes the keymap Firefox\'s', await until(() => keymap().follows === 'firefox'), JSON.stringify(keymap()));
  check('and shows its keys: Downloads on ⌘J, page source on ⌘U', await until(async () => {
    const t = await keys.innerText();
    return t.includes('⌘J') && !t.includes('⇧⌘J') && t.includes('⌘U');
  }), await keys.innerText());
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-work.png') });
  await welcome.getByRole('button', { name: 'Continue' }).click();
  const spaces = () => { try { return JSON.parse(fs.readFileSync(path.join(lib.profile, 'workspaces.json'), 'utf8')).workspaces; } catch { return []; } };
  check('Continue makes the workspaces ticked, Main as Personal', await until(() => JSON.stringify(spaces().map((x) => x.name)) === JSON.stringify(['Personal', 'Work', 'Travel'])) && spaces()[0].id === '', JSON.stringify(spaces()));
  await welcome.getByRole('button', { name: 'Back' }).click();
  await welcome.getByRole('checkbox', { name: 'Travel', exact: true }).uncheck();
  await welcome.getByRole('button', { name: 'Continue' }).click();
  check('taken again, one no longer ticked goes', await until(() => JSON.stringify(spaces().map((x) => x.name)) === JSON.stringify(['Personal', 'Work'])), JSON.stringify(spaces()));
  await welcome.getByRole('button', { name: 'Continue' }).click();
  // Make default browser asks the system, which the mock answers yes for.
  await welcome.getByRole('button', { name: 'Make default browser' }).click();
  check('Make default browser asks, and the page says it is the default', await until(() => welcome.getByText('This is your default browser').isVisible()));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-done.png') });
  await welcome.getByRole('button', { name: 'Start browsing' }).click();
  check('Start browsing closes it and the shell comes back', await until(async () => !welcomeTab(await regions()) && (await bars(w))));
  check('and welcome.json says it was finished', done() === true, String(done()));
  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // Finished, and the flag off: no welcome. Show the welcome again closes the app, and the
  // next start opens it over the whole window.
  flags({});
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await until(async () => (await regions()).shell === 'standard');
  check('a start after it was finished opens none', !welcomeTab(await regions()));
  await lib.fire('menu', 'app.settings');
  await w.settings.getByRole('button', { name: 'Close the app' }).click();
  check('Run setup again, in Settings, closes the app', await until(async () => (await regions()).quit));
  check('and welcome.json says it is unfinished', done() === false, String(done()));
  await w.page.close();
  await lib.stop();
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  check('the next start opens it over the whole window', await until(async () => {
    const r = await regions();
    return welcomeTab(r) && r.active === welcomeTab(r).id && !(await bars(w));
  }));

  // The picture of the browser has the platform's window buttons: the Mac's lights at the
  // left, or Linux's own three at the right end of the top row; and New Tab's key as the
  // platform writes it.
  const picture = async (wn, platform) => {
    const at = wn.page.frameLocator('iframe.our-page[data-page="welcome"]');
    const win = at.locator('.window').first();
    await win.waitFor();
    const n = (sel) => win.locator(sel).count();
    const key = await win.locator('.vtab.new em').innerText();
    if (platform === 'mac') {
      check('on the Mac the picture has the lights and no window buttons', (await n('.lights')) === 1 && (await n('.window-buttons')) === 0);
      check('and New Tab reads ⌘T', key === '⌘T', key);
      return;
    }
    const bar = await win.locator('.bar').boundingBox();
    const buttons = win.locator('.window-buttons');
    const box = await buttons.boundingBox();
    check(`on ${platform} the picture has no lights`, (await n('.lights')) === 0);
    check('and three window buttons at the right end of the top row', (await buttons.locator('svg').count()) === 3 && box && box.x + box.width > bar.x + bar.width * 0.9, JSON.stringify({ bar, box }));
    check('and New Tab reads Ctrl+T', key === 'Ctrl+T', key);
  };
  await picture(w, 'mac');
  await w.page.close();
  await lib.stop();
  await lib.start(['-use=standard', '-platform=linux']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await until(async () => welcomeTab(await regions()));
  await picture(w, 'linux');
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-linux-hello.png') });
  const lw = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await lw.getByRole('button', { name: 'Get started' }).click();
  await lw.getByRole('heading', { name: 'Bring your things over' }).waitFor();
  await lw.getByRole('button', { name: 'Continue' }).click();
  for (const [name, shot] of [['Tabs on the side', 'side'], ['Tabs on top', 'top'], ['Glide', 'glide']]) {
    await lw.getByRole('radio', { name }).click();
    await settle(w.page);
    await w.page.screenshot({ path: path.join(lib.out, `welcome-linux-${shot}.png`) });
  }
  check('it logged no errors on Linux', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await browser.close();
  await lib.done();
})();
