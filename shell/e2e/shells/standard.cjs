// The standard shell's own tests, past what first-app.cjs drives: the right-click menu over
// the page (docs/daily-driver.md), on a link, an image, a selection, a field and the page
// itself, chosen by the mouse and by the keys, its workspace submenu, and closing it. It
// leaves light and dark screenshots in build/e2e/standard-menu-*.png.
// Run: node shell/e2e/shells/standard.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ version: 2, workspaces: [{ id: '', name: 'Main', color: '#2e9d5b' }, { id: '0a1b2c3d', name: 'Work', color: '#3470d0' }] }));
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const menu = ui.getByRole('menu').first();
  const item = (name) => ui.getByRole('menuitem', { name: new RegExp(`^${name}`) });
  const rightClick = async (on) => {
    await fire('contextMenu', JSON.stringify(on));
    await menu.waitFor();
    await settle(page);
  };
  const gone = () => until(async () => (await menu.count()) === 0);
  const shoot = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `standard-menu-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  check('comes up', await until(async () => (await regions()).shell === 'standard' && (await regions()).visible.length === 1, 10000));
  await fire('navigate', 'https://a.test/');
  await until(async () => (await regions()).tabs[0]?.title !== '');
  const first = (await regions()).active;

  // A link: Go's actions, in a menu at the press, over the page and the shell's.
  await rightClick({ x: 120, y: 80, link: 'https://b.test/', linkText: 'b' });
  const names = await ui.getByRole('menuitem').allInnerTexts();
  check('a link\'s menu has Go\'s actions', ['Open Link in New Tab', 'Open Link in New Window', 'Open Link in Workspace', 'Copy Link'].every((n) => names.some((x) => x.startsWith(n))), JSON.stringify(names));
  check('it is named for what was pressed', (await menu.getAttribute('aria-label')) === 'Link');
  const box = await menu.boundingBox();
  const at = (await regions()).visible[0].rect;
  check('it opens at the press, in the page', Math.abs(box.x - (at.x + 120)) < 2 && Math.abs(box.y - (at.y + 80)) < 2, JSON.stringify({ box, at }));
  const cover = await regions({ x: at.x + at.w - 10, y: at.y + at.h - 10 });
  check('while open, a press anywhere is the shell\'s', cover.hit === 'shell', JSON.stringify(cover.hit));
  check('the link\'s copy actions come after a line', JSON.stringify(await menu.evaluate((m) => [...m.children].map((c) => c.getAttribute('role') === 'separator' ? '-' : c.textContent.trim()))) === JSON.stringify(['Open Link in New Tab', 'Open Link in Workspace', 'Open Link in New Window', '-', 'Copy Link', 'Save Link As…']));
  const row = menu.getByRole('menuitem', { name: /^Open Link in Workspace/ });
  const chevron = await row.locator('.ui-menu-chevron').boundingBox();
  const rowBox = await row.boundingBox();
  const menuBox = await menu.boundingBox();
  check('the submenu\'s chevron sits inside its row, 12px from the end, and the row inside the menu', chevron && Math.abs(rowBox.x + rowBox.width - 12 - (chevron.x + chevron.width)) < 1.5 && rowBox.x + rowBox.width <= menuBox.x + menuBox.width, JSON.stringify({ chevron, rowBox, menuBox }));
  await shoot('link');
  await item('Copy Link').click();
  check('Copy Link copies it, and the menu goes', (await gone()) && (await until(async () => (await regions()).clipboard === 'https://b.test/')), JSON.stringify((await regions()).clipboard));
  check('and the page has the keyboard again', await until(async () => (await regions()).focused === first));

  // The workspace submenu, by the keys.
  await rightClick({ link: 'https://work.test/' });
  await menu.focus();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowRight');
  const sub = ui.getByRole('menu', { name: 'Open Link in Workspace' });
  check('→ opens the workspace submenu, with the other workspace', await until(async () => (await sub.isVisible()) && (await sub.getByRole('menuitem').allInnerTexts()).join() === 'Work'));
  await shoot('workspace');
  await page.keyboard.press('Enter');
  check('Enter opens the link in that workspace', await until(async () => (await regions()).tabs.some((t) => t.url === 'https://work.test/' && t.workspace === '0a1b2c3d')));

  // Escape closes, and nothing runs.
  await rightClick({ link: 'https://c.test/' });
  await menu.focus();
  await page.keyboard.press('Escape');
  check('Escape closes it', await gone());
  await rightClick({ link: 'https://c.test/' });
  const r = (await regions()).visible[0].rect;
  await page.mouse.click(r.x + r.w - 20, r.y + r.h - 20);
  check('a press outside closes it', await gone());
  check('and nothing ran', !(await regions()).tabs.some((t) => t.url === 'https://c.test/') && (await regions()).clipboard === 'https://b.test/');

  // A link opened in a new tab opens behind.
  await rightClick({ link: 'https://d.test/' });
  await item('Open Link in New Tab').click();
  check('Open Link in New Tab opens it behind', await until(async () => {
    const g = await regions();
    return g.tabs.some((t) => t.url === 'https://d.test/') && g.tabs.find((t) => t.id === g.active)?.url === 'https://a.test/';
  }));

  // A selection, a field and the page.
  await rightClick({ selection: 'set up your workspace and open the dashboard' });
  check('a selection offers Copy and a search for it', (await item('Copy').count()) === 1 && (await item('Search for “set up your workspace an…”').count()) === 1);
  await shoot('selection');
  await item('Copy').click();
  check('Copy copies the selection', await until(async () => (await regions()).clipboard === 'set up your workspace and open the dashboard'));

  await rightClick({ editable: true });
  check('a field offers Cut disabled with nothing selected', (await item('Cut').getAttribute('aria-disabled')) === 'true');
  await item('Paste').click();
  check('Paste pastes into the page', await until(async () => JSON.stringify((await regions()).edits) === '[5]'), JSON.stringify((await regions()).edits));

  await rightClick({});
  check('the page offers Back, Forward disabled with nowhere to go, and Reload', (await item('Back').getAttribute('aria-disabled')) === null && (await item('Forward').getAttribute('aria-disabled')) === 'true' && (await item('Reload').count()) === 1);
  await shoot('page');
  await page.keyboard.press('Escape');
  await gone();

  // Near the window's corner it flips to stay inside.
  const v = (await regions()).visible[0].rect;
  await rightClick({ x: v.w - 10, y: v.h - 10, link: 'https://e.test/' });
  const b = await menu.boundingBox();
  const vp = page.viewportSize();
  check('near the corner it stays inside the window', b.x + b.width <= vp.width - 4 && b.y + b.height <= vp.height - 4, JSON.stringify({ b, vp }));
  await page.keyboard.press('Escape');

  // Downloads: the control shows once there is one, and its list acts on each.
  const control = ui.getByRole('button', { name: 'Downloads' });
  check('no Downloads control before any download', (await control.count()) === 0);
  await fire('download', 'https://a.test/files/report.pdf');
  check('a download brings the Downloads control', await until(() => control.isVisible()));
  await control.click();
  const list = ui.getByRole('dialog', { name: 'Downloads' });
  const entry = list.getByRole('listitem', { name: 'report.pdf' });
  check('its list shows the download by its file name', await until(() => entry.isVisible()));
  check('done, it offers Open and Show in Finder', await until(async () => (await entry.getByRole('button', { name: 'Open' }).count()) === 1 && (await entry.getByRole('button', { name: 'Show in Finder' }).count()) === 1));
  await shoot('downloads');
  await entry.getByRole('button', { name: 'Show in Finder' }).click();
  check('Show in Finder asks Go to show the file', await until(async () => (await regions()).shown?.length === 1));
  await entry.getByRole('button', { name: 'Open' }).click();
  check('Open asks Go to open it', await until(async () => (await regions()).openedFiles?.length === 1));
  await page.keyboard.press('Escape');
  check('Escape closes the list', await until(async () => (await list.count()) === 0));
  await control.click();
  await list.getByRole('button', { name: 'Clear' }).click();
  check('Clear empties the list, and the control goes', await until(async () => (await control.count()) === 0));

  // A page's dialogs: drawn over the window while its tab shows, answered back to Go.
  const ask = ui.getByRole('alertdialog');
  await fire('prompt', 'Your name?');
  check('a prompt shows as an alertdialog with its words and a field', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('Your name?') && (await ask.getByRole('textbox').count()) === 1));
  check('its field starts with the page\'s text', (await ask.getByRole('textbox').inputValue()) === 'mock');
  await shoot('prompt');
  const t1 = (await regions()).active;
  await ui.getByRole('button', { name: 'New Tab' }).first().click();
  check('another tab shown hides it', await until(async () => (await ask.count()) === 0 && (await regions()).active !== t1));
  check('and its tab\'s row says it waits', await until(async () => (await ui.locator(`[role=tab][data-tab="${t1}"] [aria-label="Waiting for an answer"]`).count()) === 1));
  await ui.locator(`[role=tab][data-tab="${t1}"]`).click();
  check('and showing its tab brings it back', await until(() => ask.isVisible()));
  await ask.getByRole('textbox').focus();
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  check('Tab goes round the card\'s controls', await ask.getByRole('textbox').evaluate((el) => el === document.activeElement));
  await ask.getByRole('textbox').fill('Ada');
  await ask.getByRole('button', { name: 'OK' }).click();
  check('OK answers the page with the text', await until(async () => JSON.stringify((await regions()).dialogAnswers) === JSON.stringify([{ id: 1, ok: true, text: 'Ada' }])), JSON.stringify((await regions()).dialogAnswers));
  check('and the dialog goes', await until(async () => (await ask.count()) === 0));
  await fire('alert', 'Saved.');
  await until(() => ask.isVisible());
  check('an alert has only OK', (await ask.getByRole('button').count()) === 1);
  await page.keyboard.press('Escape');
  check('Escape dismisses it', await until(async () => (await ask.count()) === 0 && (await regions()).dialogAnswers?.length === 2));
  await fire('beforeunload', '');
  await until(() => ask.isVisible());
  check('leaving asks with Leave and Cancel', (await ask.getByRole('button', { name: 'Leave' }).count()) === 1 && (await ask.getByRole('button', { name: 'Cancel' }).count()) === 1);
  await ask.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel answers no', await until(async () => (await regions()).dialogAnswers?.[2]?.ok === false));

  // Find in page: the find key opens the bar with the keyboard in it.
  const finder = ui.getByRole('textbox', { name: 'Find in page' });
  await fire('menu', 'find.open');
  check('the find key opens a field named Find in page, with the keyboard in it', await until(async () => (await finder.isVisible()) && (await finder.evaluate((el) => el === document.activeElement))));
  await finder.pressSequentially('mock');
  const bar = ui.getByRole('search', { name: 'Find in page' });
  check('typing searches, and the count shows', await until(async () => (await bar.innerText()).includes('1 of 3')));
  await shoot('find');
  await finder.press('Enter');
  check('Enter shows the next match', await until(async () => (await bar.innerText()).includes('2 of 3')));
  await finder.press('Shift+Enter');
  await finder.press('Shift+Enter');
  check('Shift+Enter the one before', await until(async () => (await bar.innerText()).includes('3 of 3')));
  await finder.fill('nothing');
  await finder.dispatchEvent('input');
  check('a search with no match says so', await until(async () => (await bar.innerText()).includes('No results')));
  await finder.press('Escape');
  check('Escape closes it, and Go ends the search', await until(async () => (await finder.count()) === 0 && (await regions()).findsStopped?.length > 0));

  // A page filling the screen has the whole window, and Escape gives it back.
  const before = (await regions()).visible[0].rect;
  await fire('pageFullscreen', '');
  const screen = page.viewportSize();
  const covers = async () => {
    const r = (await regions()).visible;
    return r.length === 1 && r[0].rect.x === 0 && r[0].rect.y === 0 && Math.round(r[0].rect.w) === screen.width && Math.round(r[0].rect.h) === screen.height;
  };
  check('a page in full screen covers the whole window', await until(covers), JSON.stringify((await regions()).visible));
  check('with no control of the shell on it', (await ui.getByRole('button', { name: 'New Tab' }).count()) === 0);
  await shoot('page-full-screen');
  await fire('key', 'escape');
  check('Escape gives the window back as it was', await until(async () => JSON.stringify((await regions()).visible[0].rect) === JSON.stringify(before)), JSON.stringify({ before, now: (await regions()).visible[0].rect }));
  check('and the address bar still shows the page\'s address', await until(async () => (await ui.getByRole('combobox', { name: 'Address' }).inputValue()).includes('a.test')), await ui.getByRole('combobox', { name: 'Address' }).inputValue().catch((e) => e.message));

  check('no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})().catch(async (e) => {
  check('the checks ran to their end', false, e.message.split('\n').slice(0, 12).join(' / '));
  await lib.done();
});
