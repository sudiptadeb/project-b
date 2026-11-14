// The standard shell's workspace switcher, on the mock engine: with two workspaces, a button
// at the sidebar's foot, or at the strip's start on top, chooses which workspace's tabs the
// window lists, or all of them; choosing one makes one of its tabs active, New Tab opens in
// it, and the window keeps the choice when its shell loads again. With one workspace there
// is nothing to choose and no button.
// Run: node shell/e2e/shells/standard-workspaces.cjs   (from the repo root, after
// `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, settle, until } = lib;

const send = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });
const WORK = '0a1b2c3d';

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: WORK, name: 'Work', color: '#d03434' }] }));
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1000, height: 600 } });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  const tabs = ui.locator('.sidebar [role=tab]');
  await until(async () => (await tabs.count()) === 1, 10000);
  // Two tabs in the first workspace, Main, as New Tab opens beside the active one; then two
  // in Work.
  await send({ t: 'navigate', text: 'https://docs.example.com/' });
  await send({ t: 'newTab', text: 'https://news.example.org/' });
  await send({ t: 'newTab', workspace: WORK, text: 'https://mail.example.com/' });
  await send({ t: 'newTab', workspace: WORK, text: 'https://tickets.example.com/' });
  await until(async () => (await tabs.count()) === 4);
  const button = ui.locator('.sidebar .ws-button');
  const menu = ui.locator('.ui-menu.float');

  check('with two workspaces, the sidebar\'s foot has the switcher, showing all', await until(async () => (await button.isVisible()) && (await button.innerText()).includes('All workspaces')));
  await button.click();
  const names = await menu.getByRole('menuitemradio').allInnerTexts();
  check('it opens a menu of the workspaces, All ticked', (await menu.getByRole('menuitemradio', { name: 'All workspaces', exact: true }).getAttribute('aria-checked')) === 'true' &&
    names.length === 3, JSON.stringify(names));
  await menu.getByRole('menuitemradio', { name: 'Work', exact: true }).click();
  check('choosing Work lists only its tabs', await until(async () => (await tabs.count()) === 2));
  check('and makes one of them active, as the active tab was elsewhere', await until(async () => {
    const r = await lib.regions();
    return (await ui.locator('.sidebar [role=tab][aria-selected=true]').count()) === 1;
  }));
  check('the button says Work', (await button.innerText()).includes('Work'));
  await ui.locator('.sidebar .new-tab').click();
  check('New Tab opens in Work, and the list shows it', await until(async () => (await tabs.count()) === 3));
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await button.click();
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `standard-workspaces-side-${scheme}.png`) });
    await page.keyboard.press('Escape');
  }

  // The window keeps the choice when its shell loads again.
  await w.shell().evaluate(() => location.reload());
  await until(async () => (await button.count()) === 1, 10000);
  check('after the shell loads again, the window still shows Work', await until(async () => (await button.innerText()).includes('Work') && (await tabs.count()) === 3, 10000));

  // On top, the switcher starts the strip.
  await fire('menu', 'view.tabsTop');
  const top = ui.locator('.strip .ws-button');
  check('on top, the switcher starts the strip, and the strip lists only Work', await until(async () => (await top.isVisible()) && (await ui.locator('.strip [role=tab]').count()) === 3));
  await top.click();
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-workspaces-top.png') });
  await menu.getByRole('menuitemradio', { name: 'All workspaces', exact: true }).click();
  check('choosing All lists every tab again', await until(async () => (await ui.locator('.strip [role=tab]').count()) === 5));
  await fire('menu', 'view.tabsSide');

  // A tab's workspace is fixed: its menu opens the same address in another workspace.
  await until(async () => (await tabs.count()) === 5);
  await tabs.first().click({ button: 'right' });
  const tabMenu = ui.locator('.ui-context-menu').first();
  await tabMenu.getByRole('menuitem', { name: 'Open in Workspace' }).click();
  await ui.locator('.ui-context-menu').nth(1).getByRole('menuitem', { name: 'Work', exact: true }).click();
  check('Open in Workspace opens the address in a tab of that workspace', await until(async () => (await tabs.count()) === 6));
  await page.screenshot({ path: path.join(lib.out, 'standard-tab-menu.png') });

  // Keyboard: Shift+F10 opens it with the first item focused; Escape closes it.
  await until(async () => (await button.count()) === 1);
  await button.focus();
  await page.keyboard.press('Enter');
  check('Enter opens it', await until(async () => (await menu.count()) === 1));
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await menu.count()) === 0));

  // With one workspace there is nothing to choose.
  await fetch(`${lib.base}/settings/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'deleteWorkspace', id: WORK }) });
  check('with one workspace left, the switcher goes, and every tab is listed', await until(async () => (await button.count()) === 0, 10000));

  check('no errors in the pages', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
