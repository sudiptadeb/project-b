// The standard shell's tab groups, on the mock engine: Go's groups drawn in the sidebar and
// in the strip on top, a header that folds its group, the group's menu that names it,
// colours it, opens a tab in it, ungroups it and closes it, and the tab's menu that starts
// a group or adds to one. Light and dark, both layouts, shot for the eye.
// Run: node shell/e2e/shells/standard-groups.cjs   (from the repo root, after `npm run build`
// in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, settle, until } = lib;

const send = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });
const sites = ['docs.example.com', 'mail.example.com', 'news.example.org', 'shop.example.net', 'maps.example.com'];

(async () => {
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1000, height: 600 } });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === 1, 10000);
  await send({ t: 'navigate', text: `https://${sites[0]}/` });
  for (const s of sites.slice(1)) {
    await send({ t: 'newTab' });
    await send({ t: 'navigate', text: `https://${s}/` });
  }
  await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === sites.length);
  const ids = await w.shell().evaluate(() => [...document.querySelectorAll('.sidebar [role=tab]')].map((e) => +e.dataset.tab));
  const header = (i = 0) => ui.locator('.sidebar .group').nth(i);
  const tabRow = (id) => ui.locator(`.sidebar [role=tab][data-tab="${id}"]`);
  const menu = ui.locator('.ui-menu.float');
  // The tab's own menu, the shared context menu, and its submenu.
  const tabMenu = ui.locator('.ui-context-menu').first();
  const subMenu = ui.locator('.ui-context-menu').nth(1);
  const toGroup = async (id, name) => {
    await tabRow(id).click({ button: 'right' });
    await tabMenu.getByRole('menuitem', { name: 'Add Tab to Group' }).click();
    await subMenu.getByRole('menuitem', { name, exact: true }).click();
  };

  // A group from the tab's menu: Add Tab to Group, New Group.
  await tabRow(ids[1]).click({ button: 'right' });
  check('a right-click on a tab opens its menu', await tabMenu.isVisible());
  check('it holds the tab\'s actions', JSON.stringify(await tabMenu.getByRole('menuitem').allInnerTexts()).includes('Duplicate Tab') &&
    (await tabMenu.getByRole('menuitem', { name: 'Close Other Tabs' }).count()) === 1, JSON.stringify(await tabMenu.getByRole('menuitem').allInnerTexts()));
  const box = await tabMenu.boundingBox();
  check('and over the page the menu is the shell\'s while open', await until(async () => (await lib.regions({ x: Math.round(box.x + box.width / 2), y: Math.round(box.y + box.height - 8) })).hit === 'shell'), JSON.stringify(box));
  await tabMenu.getByRole('menuitem', { name: 'Add Tab to Group' }).click();
  await subMenu.getByRole('menuitem', { name: 'New Group', exact: true }).click();
  check('New Group draws a group with a header, the tab under it', await until(async () => (await header().count()) === 1 && (await tabRow(ids[1]).evaluate((e) => e.classList.contains('grouped')))));
  check('the menu closes once it has run', (await ui.locator('.ui-context-menu').count()) === 0);

  // Another tab joins it from its menu.
  await toGroup(ids[2], '1 tab');
  check('and choosing the group puts the tab in it', await until(async () => (await tabRow(ids[2]).evaluate((e) => e.classList.contains('grouped')))));

  // The group's menu names and colours it.
  await header().click({ button: 'right' });
  check('a right-click on the header opens the group\'s menu', await menu.isVisible());
  await menu.getByLabel('Group name').fill('Research');
  await menu.getByRole('radio', { name: '#d03434' }).click();
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await menu.count()) === 0));
  check('the name and the colour are kept, and the header shows them', await until(async () => (await header().innerText()).includes('Research') &&
    (await header().locator('.swatch').evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(208, 52, 52)'));

  // A press on the header folds the group; the active tab stays when it's in it.
  await ui.locator(`.sidebar [role=tab][data-tab="${ids[0]}"]`).click();
  await header().click();
  check('a press on the header folds the group: its tabs go', await until(async () => (await tabRow(ids[1]).count()) === 0 && (await tabRow(ids[2]).count()) === 0));
  check('and the header says it is folded', (await header().getAttribute('aria-expanded')) === 'false');
  await header().click();
  check('another press opens it again', await until(async () => (await tabRow(ids[1]).count()) === 1));

  // New tab in group from the group's menu.
  await header().click({ button: 'right' });
  await menu.getByRole('menuitem', { name: 'New tab in group' }).click();
  check('New tab in group opens a tab in the group', await until(async () => (await ui.locator('.sidebar [role=tab].grouped').count()) === 3));

  // Tabs on top: the group as a chip, its tabs ruled in its colour.
  await fire('menu', 'view.tabsTop');
  await until(async () => (await ui.locator('.strip').count()) === 1);
  check('on top, the group is a chip with its name', await until(async () => (await ui.locator('.strip .chip').innerText()).includes('Research')));
  check('and its tabs carry its rule', (await ui.locator('.strip [role=tab].grouped').count()) === 3);
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `standard-groups-top-${scheme}.png`) });
  }
  await ui.locator('.strip .chip').click({ button: 'right' });
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-groups-menu-top.png') });
  await page.keyboard.press('Escape');
  await fire('menu', 'view.tabsSide');
  await until(async () => (await header().count()) === 1);
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `standard-groups-side-${scheme}.png`) });
  }

  // Ungroup keeps the tabs; Close group closes them.
  await header().click({ button: 'right' });
  await menu.getByRole('menuitem', { name: 'Ungroup' }).click();
  check('Ungroup ends the group and keeps its tabs', await until(async () => (await header().count()) === 0 && (await ui.locator('.sidebar [role=tab]').count()) === sites.length + 1));
  await toGroup(ids[3], 'New Group');
  await until(async () => (await header().count()) === 1);
  await toGroup(ids[4], '1 tab');
  await until(async () => (await ui.locator('.sidebar [role=tab].grouped').count()) === 2);
  await header().click({ button: 'right' });
  await menu.getByRole('menuitem', { name: 'Close group' }).click();
  check('Close group closes its tabs', await until(async () => (await header().count()) === 0 && (await ui.locator('.sidebar [role=tab]').count()) === sites.length - 1));

  // The keyboard: Shift+F10 on a tab opens its menu with the first item focused.
  await tabRow(ids[0]).focus();
  await page.keyboard.press('Shift+F10');
  check('Shift+F10 on a tab opens its menu, the first item focused', await until(async () => (await tabMenu.isVisible()) && (await tabMenu.getByRole('menuitem').first().evaluate((e) => e === document.activeElement))));
  await page.keyboard.press('ArrowDown');
  check('the arrow keys move through it', !(await tabMenu.getByRole('menuitem').first().evaluate((e) => e === document.activeElement)));
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await ui.locator('.ui-context-menu').count()) === 0));

  // Close Other Tabs closes the window's other tabs; Duplicate opens a copy.
  const before = await ui.locator('.sidebar [role=tab]').count();
  await tabRow(ids[0]).click({ button: 'right' });
  await tabMenu.getByRole('menuitem', { name: 'Duplicate Tab' }).click();
  check('Duplicate Tab opens a copy of the tab', await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === before + 1));
  await tabRow(ids[0]).click({ button: 'right' });
  await tabMenu.getByRole('menuitem', { name: 'Close Other Tabs' }).click();
  check('Close Other Tabs leaves only that tab', await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === 1 && (await tabRow(ids[0]).count()) === 1));

  check('no errors in the pages', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
