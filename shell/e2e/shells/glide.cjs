// Glide's own tests, on the mock engine: the sidebar of the Space shown, the Command Bar,
// Favorites and pinned tabs kept for the profile across a restart, folders on Go's tab
// groups, split view, Spaces as workspaces, and the sidebar hidden. The contract every
// shell keeps is shell-contract.cjs's. It leaves light and dark screenshots of the window
// in build/e2e/glide-*.png.
// Run: node shell/e2e/shells/glide.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  // A second Space: a workspace.
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ version: 2, workspaces: [{ id: '', name: 'Main', color: '#2e9d5b' }, { id: '0a1b2c3d', name: 'Work', color: '#3470d0' }] }));
  await lib.start(['-use=glide']);
  const browser = await chromium.launch();
  let w = await lib.openWindow(browser);
  let { page, ui } = w;
  page.setDefaultTimeout(5000);

  const tab = (id) => ui.locator(`[role=tab][data-tab="${id}"]`);
  const bar = ui.getByRole('dialog', { name: 'Command Bar' });
  const field = ui.getByRole('combobox', { name: 'Address' });
  const kept = () => JSON.parse(fs.readFileSync(path.join(lib.profile, 'shells', 'glide.json'), 'utf8'));
  const active = async () => {
    const r = await regions();
    return r.tabs.find((t) => t.id === r.active);
  };
  const at = (url) => until(async () => {
    const t = await active();
    return t?.url === url && !t.loading && t.title !== '';
  });
  const shown = async () => (await regions()).visible.map((v) => v.tab).sort((a, b) => a - b);
  const frame = () => w.shell().evaluate(() => getComputedStyle(document.querySelector('.glide')).getPropertyValue('--g-frame').trim());
  const menuItem = (name) => ui.getByRole('menuitem', { name: new RegExp(`^${name}`) });
  const shoot = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `glide-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };
  // The Command Bar: open it as the key does, type, and Enter.
  const command = async (text, { enter = true } = {}) => {
    await fire('menu', 'glide.commandBar');
    await field.waitFor();
    await field.fill(text);
    if (enter) await field.press('Enter');
  };

  check('comes up', await until(async () => (await regions()).shell === 'glide' && (await regions()).visible.length === 1, 10000));
  const t1 = (await regions()).active;
  check('a new window\'s empty tab opens the Command Bar on the address', await until(() => bar.isVisible()));
  await field.fill('mail.test');
  await field.press('Enter');
  check('an address and Enter take the tab there, and the bar closes', (await at('https://mail.test')) && (await until(async () => !(await bar.isVisible()))));
  check('the Space is Main, with both Spaces at the foot', (await ui.locator('.space-head').innerText()).trim() === 'Main' && (await ui.locator('.foot .space').count()) === 2);

  // Favorites, kept for the profile.
  await fire('menu', 'glide.favorite');
  check('Add to Favorites puts a tile at the top', await until(() => ui.getByRole('button', { name: 'Favorite Mock mail.test' }).isVisible()));
  check('and the profile keeps it', await until(() => kept().favorites?.['']?.[0]?.url === 'https://mail.test'));

  // Open Command Bar opens a new tab in the Space.
  await command('docs.test');
  check('Enter in the empty Command Bar opens a new tab there', await until(async () => (await regions()).tabs.length === 2) && (await at('https://docs.test')));
  const t2 = (await regions()).active;
  // Words look among the tabs first.
  await command('mail', { enter: false });
  check('the bar lists the open tab that matches', await until(() => ui.getByRole('option', { name: /Mock mail\.test/ }).isVisible()));
  await page.keyboard.press('Enter');
  check('and Enter switches to it', await until(async () => (await regions()).active === t1));
  await command('/zoom', { enter: false });
  check('a leading / lists actions by name, with their keys', await until(() => ui.getByRole('option', { name: /Zoom In/ }).isVisible()));
  await page.keyboard.press('Escape');
  check('Escape closes the bar', await until(async () => !(await bar.isVisible())));
  await shoot('command-bar-closed');

  // A favourite whose tab is open switches to it.
  await tab(t2).click();
  await ui.getByRole('button', { name: 'Favorite Mock mail.test' }).click();
  check('a Favorite switches to its open tab', await until(async () => (await regions()).active === t1 && (await regions()).tabs.length === 2));

  // An empty tab the page takes somewhere closes the bar it opened.
  await ui.getByRole('button', { name: 'New Tab' }).click();
  check('New Tab opens an empty tab with the Command Bar', await until(async () => (await regions()).tabs.length === 3 && (await bar.isVisible())));
  const empty = (await regions()).active;
  await fire('navigate', 'https://elsewhere.test/', { tab: empty });
  check('a page that goes somewhere by itself closes the bar', await until(async () => !(await bar.isVisible())));
  await ui.getByRole('button', { name: /^Close Mock elsewhere/ }).click();
  await until(async () => (await regions()).tabs.length === 2);

  // Pinning keeps a tab at the top of its Space, for the profile.
  await tab(t2).click();
  await until(async () => (await tab(t2).getAttribute('aria-selected')) === 'true');
  await fire('menu', 'glide.pin');
  check('Pin Tab moves it above New Tab', await until(async () => (await ui.locator('.pinned [role=tab]').count()) === 1 && (await ui.locator('.pinned [role=tab]').getAttribute('data-tab')) === String(t2)));
  check('and the profile keeps its address', kept().pinned?.['']?.[0]?.url === 'https://docs.test');

  // Folders are Go's tab groups.
  await command('one.test');
  await at('https://one.test');
  const t3 = (await regions()).active;
  await tab(t3).click({ button: 'right' });
  await menuItem('Add to New Folder').click();
  check('Add to New Folder makes a folder of the tab', await until(() => ui.locator('.folder').isVisible()));
  await tab(t3).click({ button: 'right' });
  await menuItem('New Tab in Folder').click();
  check('New Tab in Folder opens a tab in it', await until(async () => (await regions()).tabs.length === 4));
  const t4 = (await regions()).active;
  await field.fill('two.test');
  await field.press('Enter');
  await at('https://two.test');
  check('both tabs are in the folder', await until(async () => (await ui.locator('.folder ~ [role=tab]').count()) >= 2));
  await ui.locator('.folder').click();
  check('a closed folder still shows its active tab', await until(async () => (await tab(t4).isVisible()) && !(await tab(t3).isVisible())));
  await ui.locator('.folder').click();
  check('and opens again', await until(() => tab(t3).isVisible()));

  // Split view.
  await fire('menu', 'glide.splitAdd');
  check('Add to Split View shows two pages side by side', await until(async () => (await shown()).length === 2));
  let r = await regions();
  const [a, b] = r.visible.map((v) => v.rect).sort((x, y) => x.x - y.x);
  check('apart, on one line, the same height', a.x + a.w + 8 <= b.x + 1 && Math.abs(a.y - b.y) <= 1 && Math.abs(a.h - b.h) <= 1, JSON.stringify(r.visible));
  check('the split is one row in the sidebar', await until(async () => (await ui.locator('.split [role=tab]').count()) === 2));
  await page.frameLocator(`iframe.mock-page[data-tab="${r.active}"]`).getByRole('button', { name: 'Navigate' }).waitFor();
  await shoot('split');
  await fire('menu', 'glide.splitRemove');
  check('Separate Split View shows one page again', await until(async () => (await shown()).length === 1));
  await tab(t1).click();
  await at('https://mail.test');
  await page.frameLocator(`iframe.mock-page[data-tab="${t1}"]`).getByRole('button', { name: 'Navigate' }).waitFor();
  await shoot('home');

  // Spaces.
  const home = await frame();
  await ui.getByRole('button', { name: 'Space Work' }).click();
  check('an empty Space opens a tab in it, with the Command Bar', await until(async () => (await active())?.workspace === '0a1b2c3d' && (await bar.isVisible())));
  check('the frame takes the Space\'s colour', await until(async () => (await frame()) !== home), await frame());
  await field.fill('work.test');
  await shoot('command-bar');
  await field.press('Enter');
  await at('https://work.test');
  check('the sidebar lists only the Space\'s tabs', (await ui.locator('.list [role=tab]').count()) === 1 && (await ui.locator('.space-head').innerText()).trim() === 'Work');
  check('the Favorites are the Space\'s own', (await ui.locator('.fav').count()) === 0);
  await page.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await shoot('work');
  await fire('menu', 'glide.space1');
  check('Space 1 goes back to Main, to the tab it showed last', await until(async () => (await regions()).active === t1));
  await fire('menu', 'glide.spaceNext');
  check('Next Space goes to Work', await until(async () => (await active())?.workspace === '0a1b2c3d'));
  await fire('menu', 'glide.spacePrevious');
  check('Previous Space comes back to Main', await until(async () => !(await active())?.workspace));

  // New Space.
  await ui.getByRole('button', { name: 'New Space' }).click();
  await ui.getByRole('textbox', { name: 'Name' }).fill('Play');
  await ui.getByRole('button', { name: 'Create' }).click();
  check('New Space makes a workspace and shows it', await until(async () => {
    const t = await active();
    return t && t.workspace && t.workspace !== '0a1b2c3d' && (await ui.locator('.space-head').innerText()).trim() === 'Play';
  }));
  await page.keyboard.press('Escape');
  await fire('menu', 'glide.space1');
  await until(async () => (await regions()).active === t1);

  // The sidebar hidden: a slim bar keeps the window's buttons off the page.
  await fire('menu', 'glide.sidebar');
  check('Show or Hide Sidebar hides it', await until(async () => (await ui.locator('.sidebar').count()) === 0 && (await ui.getByRole('button', { name: 'Show Sidebar' }).isVisible())));
  check('the page widens, and stays clear of the window\'s buttons', await until(async () => {
    r = await regions();
    return r.visible.length === 1 && r.visible[0].rect.x <= 8 && r.visible[0].rect.y >= 36;
  }), JSON.stringify(r.visible));
  await ui.getByRole('button', { name: 'Show Sidebar' }).click();
  check('Show Sidebar brings it back', await until(() => ui.locator('.sidebar').isVisible()));

  // Closing a pinned tab closes it, and unpins it.
  await tab(t2).click();
  await ui.getByRole('button', { name: 'Close Mock docs.test' }).click();
  check('closing a pinned tab closes it and unpins it', await until(async () => !(await regions()).tabs.some((t) => t.id === t2) && (kept().pinned?.[''] || []).length === 0));

  // Kept across a restart: the Favorites and a pinned tab, which then opens on a press.
  await tab(t3).click();
  await until(async () => (await tab(t3).getAttribute('aria-selected')) === 'true');
  await fire('menu', 'glide.pin');
  await until(() => (kept().pinned?.[''] || []).length === 1);
  await page.close();
  await lib.stop();
  await lib.start(['-use=glide']);
  w = await lib.openWindow(browser);
  ({ page, ui } = w);
  page.setDefaultTimeout(5000);
  check('after a restart, the Favorite is there', await until(() => ui.getByRole('button', { name: 'Favorite Mock mail.test' }).isVisible(), 10000));
  const pinRow = ui.locator('.unloaded');
  check('and the pinned tab, waiting to be opened', await until(() => pinRow.isVisible()));
  await page.keyboard.press('Escape');
  await pinRow.click();
  check('a press opens it at its address, pinned', await until(async () => (await active())?.url === 'https://one.test' && (await ui.locator('.pinned [role=tab]').count()) === 1));

  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
