// The mobile shell's own tests, on the mock engine in a phone-sized window with a touch
// screen: the one bar at the foot, the address pill and its editing with the suggestions
// over the field, a new tab in edit mode, the switcher (open, switch, close a card, the
// last tab), the menu sheet and Escape, a swipe along the bar, the workspaces, light and
// dark. The check covers what every shell does.
// Run: node shell/e2e/shells/mobile.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

const id = 'mobile';
const W = 412;
const H = 915;

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#3470d0' }] }));
  await lib.start([`-use=${id}`]);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: W, height: H }, hasTouch: true });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const field = ui.getByRole('textbox', { name: 'Address' });
  const tabsButton = ui.getByRole('button', { name: 'Tabs', exact: true });
  const menuButton = ui.getByRole('button', { name: 'Menu' });
  const switcher = ui.getByRole('dialog', { name: 'Tabs' });
  const menu = ui.getByRole('dialog', { name: 'Menu' });
  const cards = switcher.getByRole('tab');
  const suggestions = ui.getByRole('list', { name: 'Suggestions' }).getByRole('listitem');
  const box = (selector) => w.shell().evaluate((s) => document.querySelector(s)?.getBoundingClientRect().toJSON(), selector);
  const active = async () => {
    const r = await regions();
    return r.tabs.find((t) => t.id === r.active);
  };
  const tabCount = (n) => until(async () => (await regions()).tabs.length === n);
  const inField = () => w.shell().evaluate(() => document.hasFocus() && document.activeElement?.getAttribute('aria-label') === 'Address');
  const visit = async (text, url) => {
    await field.tap();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type(text);
    await page.keyboard.press('Enter');
    return until(async () => {
      const t = await active();
      return t.url === url && !t.loading;
    });
  };
  const shots = async (what) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `${id}-${what}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  check('the window loads the mobile shell with its one tab', await until(async () => (await regions()).shell === id && (await regions()).visible.length === 1, 10000));
  let r = await regions();
  check('it asks for no band', r.frameWidth === 0, String(r.frameWidth));

  // The one bar, at the foot: 48px, the page above it.
  const bar = await box('.toolbar');
  check('the toolbar is the window\'s foot, 48px', bar.bottom === H && bar.height === 48, JSON.stringify(bar));
  r = await regions();
  const shown = r.visible[0].rect;
  check('the page fills the window down to the toolbar', shown.x === 0 && shown.w === W && shown.y + shown.h === H - 48, JSON.stringify(shown));
  const pill = await field.boundingBox();
  check('the address pill fills the bar beside the count and ⋯', pill.width > W - 140, JSON.stringify(pill));

  // A first tab with nothing in it starts with the keyboard in the field.
  check('the first, empty tab starts with the keyboard in the address field', await until(inField));
  await visit('example.com/docs/release', 'https://example.com/docs/release');
  check('an address and Enter take the tab there', true);
  await settle(page);
  check('at rest the pill shows the host alone', await until(async () => (await field.inputValue()) === 'example.com'), await field.inputValue());
  check('with a lock, the page being https', (await ui.locator('.address .mark').count()) === 1);
  await visit('example.com/docs/rollout', 'https://example.com/docs/rollout');
  await visit('example.org/release-notes', 'https://example.org/release-notes');
  await shots('page');

  // Editing: a tap selects the whole address; typing lists suggestions over the field,
  // the closest nearest it.
  await field.tap();
  check('a tap on the pill puts the whole address in the field, selected', await until(async () => (await field.inputValue()) === 'https://example.org/release-notes' &&
    await w.shell().evaluate(() => { const f = document.activeElement; return f.selectionStart === 0 && f.selectionEnd === f.value.length; })));
  await page.keyboard.type('example.com/docs');
  check('typing lists suggestions', await until(async () => (await suggestions.count()) >= 2), String(await suggestions.count()));
  const list = await box('.suggestions');
  const fieldBox = await field.boundingBox();
  check('they lie over the field, the toolbar under them', list.bottom <= fieldBox.y, JSON.stringify({ list, fieldBox }));
  const rows = await suggestions.evaluateAll((els) => els.map((e) => e.getBoundingClientRect().height));
  check('each row is 48px', rows.every((h) => Math.round(h) === 48 || Math.round(h) === 49), JSON.stringify(rows));
  check('a press on them reaches the shell, over the page', await until(async () => (await regions({ x: 100, y: list.bottom - 10 })).hit === 'shell'));
  await shots('suggestions');
  const nearest = suggestions.last();
  const target = await nearest.locator('.url').innerText();
  await nearest.getByRole('button', { name: /^Edit / }).tap();
  check('↖ puts its address in the field and keeps editing', await until(async () => (await field.inputValue()).endsWith(target) && await inField()), await field.inputValue());
  await page.keyboard.press('Escape');
  check('Escape puts the address back and gives the page the keyboard', await until(async () => (await field.inputValue()) === 'example.org' && (await regions()).focused === (await regions()).active));
  await field.tap();
  await page.keyboard.type('example.com/docs/re');
  await until(async () => (await suggestions.count()) >= 1);
  const go = await suggestions.last().locator('.url').innerText();
  await suggestions.last().getByRole('button').first().tap();
  check('a tap on a suggestion goes there', await until(async () => (await active()).url.endsWith(go) && (await suggestions.count()) === 0), JSON.stringify({ go, now: (await active()).url }));

  // A new tab from the menu starts in edit mode.
  await menuButton.tap();
  check('⋯ opens the menu sheet', await until(() => menu.isVisible()));
  check('with Back, as the tab has somewhere to go back to', await menu.getByRole('button', { name: 'Back' }).isVisible());
  await shots('menu');
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => !(await menu.isVisible())));
  await menuButton.tap();
  await menu.getByRole('button', { name: 'Back' }).tap();
  check('Back in the menu goes back, and closes it', await until(async () => (await active()).url === 'https://example.org/release-notes' && !(await menu.isVisible())));
  await menuButton.tap();
  await menu.getByRole('button', { name: 'New Tab' }).tap();
  await tabCount(2);
  check('New tab opens an empty tab with the keyboard in the field', await until(async () => (await active()).url === '' && await inField()));
  check('the count says 2', (await tabsButton.innerText()) === '2');
  await visit('example.net/mail', 'https://example.net/mail');

  // The switcher.
  await tabsButton.tap();
  // Once it has slid in.
  check('the count opens the switcher over the page, the toolbar still under it', await until(async () => await switcher.isVisible() && (await box('.switcher')).bottom === H - 48), JSON.stringify(await box('.switcher')));
  check('a card per tab, the active one selected', (await cards.count()) === 2 && (await cards.nth(1).getAttribute('aria-selected')) === 'true');
  const card = await cards.first().boundingBox();
  check('two columns', card.width > 150 && card.width < W / 2, JSON.stringify(card));
  await shots('switcher');
  const first = (await regions()).tabs[0].id;
  await cards.first().tap();
  check('a tap on a card shows its tab and closes the switcher', await until(async () => (await regions()).active === first && !(await switcher.isVisible())));
  await tabsButton.tap();
  await cards.nth(1).getByRole('button', { name: /^Close / }).tap();
  check('its × closes that tab and keeps the switcher open', await tabCount(1) && await switcher.isVisible());
  await cards.first().getByRole('button', { name: /^Close / }).tap();
  check('closing the last tab leaves one new, empty tab in edit mode', await until(async () => (await regions()).tabs.length === 1 && (await active()).url === '' && await inField()));
  check('and the switcher shut', !(await switcher.isVisible()));

  // A swipe along the bar goes to the tab after or before.
  await visit('example.com/one', 'https://example.com/one');
  await fire('menu', 'tab.new');
  await tabCount(2);
  await visit('example.com/two', 'https://example.com/two');
  const [one, two] = (await regions()).tabs.map((t) => t.id);
  const swipe = async (from, to) => {
    const y = H - 24;
    await page.mouse.move(from, y);
    await page.mouse.down();
    await page.mouse.move((from + to) / 2, y);
    await page.mouse.move(to, y);
    await page.mouse.up();
  };
  await swipe(150, 330);
  check('a swipe right along the bar goes to the tab before', await until(async () => (await regions()).active === one));
  await swipe(330, 150);
  check('a swipe left goes to the tab after', await until(async () => (await regions()).active === two));
  check('and leaves the field alone', !(await inField()));

  // Workspaces: chips in the switcher, and where new tabs open.
  await tabsButton.tap();
  const chips = switcher.getByRole('radio');
  check('the switcher has a chip per workspace, Default first', await until(async () => (await chips.allInnerTexts()).join() === 'Default,Work'), await switcher.locator('.head').innerHTML());
  await chips.nth(1).tap();
  check('Work shows none of the default\'s cards', await until(async () => (await cards.filter({ visible: true }).count()) === 0));
  await switcher.getByRole('button', { name: 'New Tab' }).tap();
  check('+ there opens a tab in Work', await until(async () => (await regions()).tabs.length === 3 && (await active()).workspace === '0a1b2c3d'), JSON.stringify(await active()));
  await visit('example.com/work', 'https://example.com/work');
  await tabsButton.tap();
  check('the switcher opens on the active tab\'s workspace', await until(async () => (await chips.nth(1).getAttribute('aria-checked')) === 'true' && (await cards.filter({ visible: true }).count()) === 1));
  await page.keyboard.press('Escape');

  // Groups. A long press on a card opens its menu; New group makes a group of its tab and a
  // new one, which starts in edit mode.
  const strip = ui.getByRole('group', { name: /^Group / });
  const stripTabs = strip.locator('button[data-tab]');
  const groupSheet = ui.getByRole('dialog', { name: 'Group' });
  const cardMenu = ui.getByRole('dialog', { name: 'Tab' });
  await tabsButton.tap();
  await chips.first().tap();
  await until(async () => (await cards.filter({ visible: true }).count()) === 2);
  const [g1] = (await regions()).tabs.map((t) => t.id);
  await cards.filter({ visible: true }).first().click({ button: 'right' });
  check('a long press on a card opens its menu', await until(() => cardMenu.isVisible()));
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  check('New group opens a new tab in edit mode', await until(async () => (await regions()).tabs.length === 4 && (await active()).url === '' && await inField()));
  check('and the strip shows the group, the tab and the new one', await until(async () => (await strip.isVisible()) && (await stripTabs.count()) === 2));
  const stripBox = await box('[role=group][aria-label^="Group "]');
  check('the strip is 44px, just over the toolbar', Math.round(stripBox.height) === 44 && Math.round(stripBox.bottom) === H - 48, JSON.stringify(stripBox));
  check('the page ends over the strip', await until(async () => {
    const v = (await regions()).visible[0].rect;
    return v.y + v.h === H - 48 - 44;
  }), JSON.stringify((await regions()).visible));
  await visit('example.com/group/two', 'https://example.com/group/two');
  const g2 = (await regions()).active;
  await strip.getByRole('button', { name: 'New tab in group' }).tap();
  check('+ in the strip opens a third tab in the group', await until(async () => (await stripTabs.count()) === 3 && (await active()).url === ''));
  await visit('example.com/group/three', 'https://example.com/group/three');
  const g3 = (await regions()).active;
  check('the strip lists the group\'s tabs in order', JSON.stringify(await stripTabs.evaluateAll((els) => els.map((e) => Number(e.dataset.tab)))) === JSON.stringify([g1, g2, g3]));
  await shots('strip');
  await stripTabs.first().tap();
  check('a tap on a favicon in the strip goes to its tab', await until(async () => (await regions()).active === g1));
  await swipe(330, 150);
  check('a swipe along the bar stays in the group', await until(async () => (await regions()).active === g2));

  // The group sheet: rename, colour.
  await strip.getByRole('button', { name: /^Group / }).tap();
  check('the group\'s dot opens its sheet', await until(() => groupSheet.isVisible()));
  const nameField = groupSheet.getByRole('textbox', { name: 'Group name' });
  await nameField.tap();
  await page.keyboard.type('Release');
  await page.keyboard.press('Enter');
  check('a name and Enter rename the group', await until(async () => (await strip.getAttribute('aria-label')) === 'Group Release'));
  await groupSheet.getByRole('radio', { name: '#8a4fd0' }).tap();
  check('a swatch gives it that colour', await until(async () => (await groupSheet.getByRole('radio', { name: '#8a4fd0' }).getAttribute('aria-checked')) === 'true'));
  await shots('group');
  await page.keyboard.press('Escape');

  // The switcher shows the group as one card.
  await tabsButton.tap();
  const groupCard = switcher.locator('[data-group]');
  check('the switcher shows the group as one card, its tabs\' favicons in it', await until(async () => (await groupCard.count()) === 1 && (await groupCard.getByRole('tab').count()) === 3));
  check('named, and the ungrouped tab beside it', (await groupCard.locator('.title').innerText()) === 'Release' && (await switcher.locator('.card:not([data-group]):visible').count()) === 1);
  await shots('switcher-group');
  await groupCard.locator('.title').tap();
  check('a tap on the group card opens its sheet', await until(() => groupSheet.isVisible()));
  await page.keyboard.press('Escape');

  // Reloaded, the shell draws the groups Go keeps.
  await w.shell().evaluate(() => location.reload());
  check('reloaded, the group comes back from Go', await until(async () => (await strip.getAttribute('aria-label').catch(() => '')) === 'Group Release' && (await stripTabs.count()) === 3, 10000));

  // The strip's × closes the active tab and goes to its neighbour in the group.
  await until(async () => (await regions()).active === g2);
  await strip.getByRole('button', { name: /^Close / }).tap();
  check('the strip\'s × closes the active tab and goes to the next in the group', await until(async () => (await regions()).active === g3 && (await stripTabs.count()) === 2));
  await stripTabs.last().click({ button: 'right' });
  check('a long press on a favicon closes its tab, and a group of one ends', await until(async () => (await regions()).tabs.length === 3 && !(await strip.isVisible())));

  // Add to group, Remove from group, and Close group with its confirm.
  await tabsButton.tap();
  await until(async () => (await cards.filter({ visible: true }).count()) === 2);
  await cards.filter({ visible: true }).nth(1).click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  await until(async () => (await regions()).tabs.length === 3);
  await visit('example.com/again', 'https://example.com/again');
  await tabsButton.tap();
  const loose = switcher.locator('.card:not([data-group])').filter({ visible: true });
  await until(async () => (await loose.count()) === 1);
  await loose.first().click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: '2 tabs' }).tap();
  check('Add to group puts a card\'s tab in that group', await until(async () => (await groupCard.getByRole('tab').count()) === 3 && (await loose.count()) === 0));
  await groupCard.getByRole('tab').first().click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Remove from group' }).tap();
  check('Remove from group takes it out', await until(async () => (await groupCard.getByRole('tab').count()) === 2 && (await loose.count()) === 1));
  await groupCard.getByRole('button', { name: /^Close group/ }).tap();
  const confirm = ui.getByRole('dialog', { name: 'Close group' });
  check('Close group asks first: Close 2 tabs?', await until(async () => (await confirm.isVisible()) && (await confirm.locator('h2').innerText()) === 'Close 2 tabs?'));
  const before = (await regions()).tabs.length;
  await confirm.getByRole('button', { name: 'Close', exact: true }).tap();
  check('and Close closes them', await until(async () => (await regions()).tabs.length === before - 2), String((await regions()).tabs.length));

  // Settings, from the menu.
  await menuButton.tap();
  await menu.getByRole('menuitem', { name: 'Settings' }).tap();
  check('Settings in the menu opens the settings page', await until(async () => (await regions()).settingsOpen === true));

  // The address bar on top, from settings on a phone: the toolbar moves under the status
  // bar, the strip stays at the foot, the suggestions grow down.
  const settings = page.frameLocator('iframe.mock-settings');
  await settings.getByRole('radiogroup', { name: 'Address bar' }).getByRole('radio', { name: 'Top' }).tap();
  await settings.getByRole('button', { name: 'Close settings' }).tap();
  await until(async () => (await regions()).settingsOpen === false);
  const clear = (await regions()).window.Controls;
  const topY = clear.h > 0 ? clear.y + clear.h : 0;
  check('Top in settings moves the toolbar to the top', await until(async () => {
    const b = await box('.toolbar');
    return b && Math.round(b.y) === topY && b.height === 48;
  }), JSON.stringify(await box('.toolbar')));
  check('the page lies under it, down to the foot', await until(async () => {
    const v = (await regions()).visible[0].rect;
    return v.y === topY + 48 && v.y + v.h === H;
  }), JSON.stringify((await regions()).visible));
  await fire('menu', 'tab.new');
  await until(inField);
  await page.keyboard.type('example.com/docs');
  await until(async () => (await suggestions.count()) >= 2);
  const below = await box('.suggestions');
  check('the suggestions grow down from the field', below.y >= topY + 48 - 4 && below.y < topY + 60, JSON.stringify(below));
  await shots('top-suggestions');
  await page.keyboard.press('Enter');
  await until(async () => !(await inField()));
  await tabsButton.tap();
  check('the switcher opens under the toolbar', await until(async () => {
    const b = await box('.switcher');
    return (await switcher.isVisible()) && Math.round(b.y) === topY + 48 && b.bottom === H;
  }), JSON.stringify(await box('.switcher')));
  await page.keyboard.press('Escape');
  await fire('menu', 'tab.select1');
  const firstTab = (await regions()).tabs[0].id;
  await until(async () => (await regions()).active === firstTab);
  await menuButton.tap();
  await menu.getByRole('button', { name: 'New Tab' }).tap();
  await until(inField);
  await page.keyboard.press('Escape');
  await tabsButton.tap();
  const lastCard = cards.filter({ visible: true }).first();
  await lastCard.click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  check('with the toolbar on top, the strip is at the foot', await until(async () => {
    const b = await box('[role=group][aria-label^="Group "]');
    return b && Math.round(b.bottom) === H;
  }), JSON.stringify(await box('[role=group][aria-label^="Group "]')));
  await page.keyboard.press('Escape');
  await shots('top-strip');

  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
