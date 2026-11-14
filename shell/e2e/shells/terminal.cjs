// The terminal shell's own tests, on the mock engine: its panes in groups, the splits, the
// dividers, the groups kept in its own data across a reload, its keys, and the gear. The contract
// every shell keeps is shell-contract.cjs's.
// Run: node shell/e2e/shells/terminal.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  await lib.start(['-use=terminal']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const chips = ui.locator('.chip');
  const title = (id) => ui.locator(`.title[data-pane="${id}"]`);
  const prompt = ui.getByRole('combobox', { name: 'Address' });
  // The pages shown, by tab, and each one's rectangle.
  const shown = async () => Object.fromEntries((await regions()).visible.map((v) => [v.tab, v.rect]));
  const showsOnly = async (ids) => {
    const s = await shown();
    return JSON.stringify(Object.keys(s).map(Number).sort((a, b) => a - b)) === JSON.stringify([...ids].sort((a, b) => a - b));
  };
  const near = (a, b) => Math.abs(a - b) <= 1;
  // Every page inside the window's frame, and no two over each other.
  const insideAndApart = async () => {
    const r = await regions();
    const f = r.frameWidth;
    const inside = r.visible.every(({ rect: p }) => p.x >= f && p.y >= f && p.x + p.w <= r.window.W - f && p.y + p.h <= r.window.H - f);
    const apart = r.visible.every((a, i) => r.visible.every((b, j) => j <= i ||
      a.rect.x + a.rect.w <= b.rect.x || b.rect.x + b.rect.w <= a.rect.x || a.rect.y + a.rect.h <= b.rect.y || b.rect.y + b.rect.h <= a.rect.y));
    return inside && apart;
  };
  const marked = (id) => title(id).evaluate((el) => el.classList.contains('active'));
  // Light and dark, of the window as it is now.
  const shoot = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `terminal-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };
  // A press in the middle of a tab's mock page, with the right button so it sets off none
  // of the page's buttons.
  const pressPage = async (id) => {
    const p = (await shown())[id];
    await page.mouse.move(p.x + p.w / 2 - 1, p.y + p.h / 2);
    await page.mouse.click(p.x + p.w / 2, p.y + p.h / 2, { button: 'right' });
  };

  // One group of one pane.
  check('comes up', await until(async () => (await regions()).shell === 'terminal' && (await regions()).visible.length === 1, 10000));
  const t1 = (await regions()).active;
  check('opens with one group of one pane', (await chips.count()) === 1 && (await ui.locator('[role=tab]').count()) === 1 && (await showsOnly([t1])));
  check('the pane\'s title bar is marked active', await marked(t1));
  await page.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await shoot('one');
  const one = (await shown())[t1];

  // Split right: two pages side by side, sharing the body's width.
  await title(t1).getByRole('button', { name: 'Split right' }).click();
  check('split right opens a tab beside it', await until(async () => (await regions()).tabs.length === 2 && (await regions()).visible.length === 2));
  const t2 = (await regions()).tabs.find((t) => t.id !== t1).id;
  let s = await shown();
  check('split right: side by side, on the same line, apart', near(s[t1].y, s[t2].y) && near(s[t1].h, s[t2].h) && s[t1].x + s[t1].w <= s[t2].x, JSON.stringify(s));
  check('split right: together they take the body\'s width', near(s[t1].x, one.x) && near(s[t2].x + s[t2].w, one.x + one.w) && near(s[t1].w, s[t2].w), JSON.stringify({ s, one }));
  check('the new pane is active, in the same group', (await regions()).active === t2 && (await chips.count()) === 1 && await marked(t2) && !(await marked(t1)));
  check('pages inside the frame and apart after a split', await insideAndApart());

  // A press in the other pane's page makes it active.
  await pressPage(t1);
  check('a press in the other pane\'s page makes it active', await until(async () => (await regions()).active === t1));
  check('and marks its title bar', await until(async () => (await marked(t1)) && !(await marked(t2))));

  // The prompt navigates the active pane only.
  await prompt.click();
  await prompt.fill('one.test');
  await prompt.press('Enter');
  check('the prompt takes the active pane to the address', await until(async () => (await regions()).tabs.find((t) => t.id === t1).url.startsWith('https://one.test')));
  check('and leaves the other pane where it was', (await regions()).tabs.find((t) => t.id === t2).url === '');

  // Split down: a third pane under the first, mixed directions in one group.
  await title(t1).getByRole('button', { name: 'Split down' }).click();
  check('split down opens a third tab', await until(async () => (await regions()).visible.length === 3));
  const t3 = (await regions()).tabs.find((t) => t.id !== t1 && t.id !== t2).id;
  s = await shown();
  check('split down: stacked, the new one under, in the same column', near(s[t1].x, s[t3].x) && near(s[t1].w, s[t3].w) && s[t1].y + s[t1].h <= s[t3].y, JSON.stringify(s));
  check('the column beside the other pane, which keeps the full height', near(s[t1].y, s[t2].y) && near(s[t3].y + s[t3].h, s[t2].y + s[t2].h) && s[t1].x + s[t1].w <= s[t2].x, JSON.stringify(s));
  check('three panes in one group', (await chips.count()) === 1 && (await ui.locator('.chip [role=tab]').count()) === 3);
  check('pages inside the frame and apart with three panes', await insideAndApart());
  await page.frameLocator(`iframe.mock-page[data-tab="${t3}"]`).getByRole('button', { name: 'Navigate' }).waitFor();
  await shoot('split');

  // Dragging the divider between the column and the other pane.
  const before = await shown();
  const grip = await ui.locator('[role=separator][aria-orientation=vertical]').boundingBox();
  const gx = grip.x + grip.width / 2;
  const gy = grip.y + grip.height / 2;
  await page.mouse.move(gx - 1, gy);
  await page.mouse.move(gx, gy);
  await page.mouse.down();
  await page.mouse.move(gx + 60, gy, { steps: 4 });
  await page.mouse.move(gx + 120, gy, { steps: 4 });
  await page.mouse.up();
  check('dragging the divider widens one side', await until(async () => near((await shown())[t1].w, before[t1].w + 120)), JSON.stringify({ before, after: await shown() }));
  s = await shown();
  check('and narrows the other as much', near(s[t2].w, before[t2].w - 120) && near(s[t3].w, s[t1].w), JSON.stringify(s));
  check('and they still fill the body', near(s[t1].x, one.x) && near(s[t2].x + s[t2].w, one.x + one.w) && await insideAndApart(), JSON.stringify(s));

  // Dragging far stops at the smallest size a pane may have.
  const grip2 = await ui.locator('[role=separator][aria-orientation=vertical]').boundingBox();
  await page.mouse.move(grip2.x + grip2.width / 2 - 1, gy);
  await page.mouse.move(grip2.x + grip2.width / 2, gy);
  await page.mouse.down();
  await page.mouse.move(2000, gy, { steps: 6 });
  await page.mouse.up();
  await settle(page);
  check('a drag past the edge leaves the pane its smallest size', near((await shown())[t2].w, 80), JSON.stringify(await shown()));
  // And back to where it was.
  const grip3 = await ui.locator('[role=separator][aria-orientation=vertical]').boundingBox();
  await page.mouse.move(grip3.x + grip3.width / 2 - 1, gy);
  await page.mouse.move(grip3.x + grip3.width / 2, gy);
  await page.mouse.down();
  await page.mouse.move(grip3.x + grip3.width / 2 - ((await shown())[t1].w - s[t1].w), gy, { steps: 6 });
  await page.mouse.up();
  check('dragged back, the sizes are as they were', await until(async () => near((await shown())[t1].w, s[t1].w)), JSON.stringify({ s, now: await shown() }));

  // Reloaded, the same groups, splits and sizes.
  const sized = JSON.stringify(await shown());
  await w.shell().evaluate(() => { window.beforeReload = true; location.reload(); });
  await until(async () => (await w.shell()?.evaluate(() => !window.beforeReload).catch(() => false)), 10000);
  check('reloaded, the same splits and sizes come back', await until(async () => JSON.stringify(await shown()) === sized), JSON.stringify({ before: sized, after: await shown() }));
  check('reloaded, one group of three', (await chips.count()) === 1 && (await ui.locator('.chip [role=tab]').count()) === 3);

  // A resize keeps every pane inside the frame.
  await page.setViewportSize({ width: 760, height: 480 });
  check('a smaller window keeps every pane inside the frame', await until(async () => (await regions()).window.W === 760 && (await insideAndApart()) && (await regions()).visible.length === 3), JSON.stringify(await regions().then((r) => ({ W: r.window.W, visible: r.visible }))));
  await page.setViewportSize({ width: 1100, height: 700 });
  check('and back', await until(async () => (await regions()).window.W === 1100 && (await insideAndApart()) && JSON.stringify(await shown()) === sized));

  // A new tab from Go is a group of its own.
  await fire('menu', 'tab.new');
  check('a new tab from Go makes a new group', await until(async () => (await regions()).tabs.length === 4 && (await chips.count()) === 2));
  const t4 = (await regions()).active;
  check('and shows it alone: the first group\'s panes are no longer shown', await until(() => showsOnly([t4])), JSON.stringify(await shown()));

  // A segment of the hidden group shows that group.
  await ui.locator(`[role=tab][data-tab="${t2}"]`).click();
  check('clicking a segment of a hidden group shows that group', await until(async () => (await regions()).active === t2 && (await showsOnly([t1, t2, t3]))));
  check('its chip is the one marked', await ui.locator('.chip.shown [role=tab]').count() === 3);
  await shoot('groups');

  // A tab a page opens is a group of its own.
  await fire('open', 'https://opened.test/', { tab: t2 });
  check('a tab a page opens becomes its own group', await until(async () => (await regions()).tabs.length === 5 && (await chips.count()) === 3));
  const t5 = (await regions()).active;
  check('and shows alone', await until(() => showsOnly([t5])));

  // Closing a group's last pane removes the group and shows another.
  await title(t5).getByRole('button', { name: /^Close/ }).click();
  check('closing a group\'s last pane removes the group', await until(async () => (await regions()).tabs.length === 4 && (await chips.count()) === 2));
  check('and shows another', await until(async () => {
    const r = await regions();
    return r.active !== t5 && r.visible.some((v) => v.tab === r.active);
  }));

  // Closing a pane gives its room to its sibling, and its sibling becomes active.
  await ui.locator(`[role=tab][data-tab="${t3}"]`).click();
  await until(async () => showsOnly([t1, t2, t3]));
  const three = await shown();
  await title(t3).getByRole('button', { name: /^Close/ }).click();
  check('closing a pane leaves its group shown', await until(() => showsOnly([t1, t2])), JSON.stringify(await shown()));
  s = await shown();
  check('its sibling takes its room', near(s[t1].y, three[t1].y) && near(s[t1].y + s[t1].h, three[t3].y + three[t3].h) && near(s[t1].w, three[t1].w), JSON.stringify({ three, s }));
  check('and becomes the active pane', (await regions()).active === t1 && await marked(t1));
  await title(t2).getByRole('button', { name: /^Close/ }).click();
  check('closing the other leaves one pane with the body to itself', await until(async () => JSON.stringify((await shown())[t1]) === JSON.stringify(one)), JSON.stringify({ one, now: await shown() }));

  // Its own keys, which Go sends it as actions while it is on: Cmd+D splits the active pane
  // right and Shift+Cmd+D down, as the buttons do, whose titles show the keys.
  const button = (name) => title(t1).getByRole('button', { name });
  check('the split buttons show their keys', (await button('Split right').getAttribute('title')) === 'Split right (⌘D)' && (await button('Split down').getAttribute('title')) === 'Split down (⇧⌘D)');
  await fire('key', 'splitRight');
  check('Cmd+D splits the active pane right', await until(async () => (await regions()).visible.length === 2));
  const t6 = (await regions()).tabs.find((t) => t.id !== t1 && t.id !== t4).id;
  s = await shown();
  check('Cmd+D: the new pane beside it, on the same line, and active', near(s[t1].y, s[t6].y) && near(s[t1].h, s[t6].h) && s[t1].x + s[t1].w <= s[t6].x && (await regions()).active === t6, JSON.stringify(s));
  await fire('key', 'splitDown');
  check('Shift+Cmd+D splits the active pane down', await until(async () => (await regions()).visible.length === 3));
  const t7 = (await regions()).tabs.find((t) => t.id !== t1 && t.id !== t4 && t.id !== t6).id;
  s = await shown();
  check('Shift+Cmd+D: the new pane under it, in the same column', near(s[t6].x, s[t7].x) && near(s[t6].w, s[t7].w) && s[t6].y + s[t6].h <= s[t7].y, JSON.stringify(s));
  check('the keys split within the shown group', (await chips.count()) === 2 && (await ui.locator('.chip.shown [role=tab]').count()) === 3 && await insideAndApart());

  // Cmd+W is Go's Close Tab, which activates the closed tab's neighbour in Go's list: here
  // t4, in the other group. The shell makes the pane that takes the room active instead, and
  // never shows the other group on the way.
  await ui.locator(`[role=tab][data-tab="${t1}"]`).click();
  await until(async () => (await regions()).active === t1);
  await w.shell().evaluate(() => {
    window.shownGroups = [];
    new MutationObserver(() => window.shownGroups.push([...document.querySelectorAll('.title[data-pane]')].map((el) => Number(el.dataset.pane))))
      .observe(document.body, { subtree: true, childList: true });
  });
  await fire('key', 'closeTab');
  check('Cmd+W in a split makes the pane that takes its room active', await until(async () => (await regions()).active === t6 && (await showsOnly([t6, t7]))), JSON.stringify(await regions().then((r) => ({ active: r.active, tabs: r.tabs.map((t) => t.id) }))));
  check('and, empty, it wants an address', await until(() => prompt.evaluate((el) => el === document.activeElement)));
  check('and the other group never showed', !(await w.shell().evaluate(() => window.shownGroups)).some((ids) => ids.includes(t4)), JSON.stringify(await w.shell().evaluate(() => window.shownGroups)));

  // The gear at the strip's far end opens the settings page.
  check('settings starts closed', (await regions()).settingsOpen === false);
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  check('the gear opens the settings page', await until(async () => (await regions()).settingsOpen === true));

  await fire('key', 'closeTab');
  check('Cmd+W closes it', await until(async () => (await regions()).settingsOpen === false));

  // With another shell on, Cmd+D is the page's: Go doesn't run the terminal's action, so
  // the shell hears nothing of it. A title set after it is the mark that it has heard all
  // Cmd+D could have sent.
  await fire('menu', 'shells:standard');
  check('View, Shell switches to standard', await until(async () => /\/shell\/standard\//.test(w.shell()?.url()) && (await ui.locator('[role=tab]').count()) === (await regions()).tabs.length, 10000));
  await w.shell().evaluate(() => {
    const own = window.__shellReceive;
    window.heard = [];
    window.__shellReceive = (json) => {
      window.heard.push(JSON.parse(json));
      own(json);
    };
  });
  const count = (await regions()).tabs.length;
  await fire('key', 'splitRight');
  await fire('title', 'After Cmd+D', { tab: (await regions()).active });
  const heard = () => w.shell().evaluate(() => window.heard.map((m) => (m.t === 'tab' ? m.tab.title : m.t)));
  check('with standard on, Cmd+D does nothing to the shell', await until(async () => (await heard()).includes('After Cmd+D')) && !(await heard()).includes('action') && (await regions()).tabs.length === count, JSON.stringify(await heard()));

  check('no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
