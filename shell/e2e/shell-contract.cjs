// The check every shell passes: "What every shell does" in docs/shells.md, rule by rule, on
// the mock engine in a real browser. It knows nothing of a shell but what the rules say: the
// tabs are role=tab with data-tab, pages show where the layout puts them, and Go's state
// comes through the mock (/mock/regions, /mock/fire).
// Run: node shell/e2e/shell-contract.cjs [id]   (from the repo root, after `npm run build`
// in shell/; with no id, every shell in core/shells/registry.json)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, fire, settle, until } = lib;

const registry = JSON.parse(fs.readFileSync(path.join(lib.root, 'core/shells/registry.json'), 'utf8'));
const ids = process.argv[2] ? [process.argv[2]] : registry.map((s) => s.id);

const rules = [
  'It says ready, draws the snapshot, and says shown',
  "The active tab's page shows, after every change, and no page is given the keyboard before it shows",
  "The pages it shows lie inside the window's frame, don't overlap, and are all tabs of this window",
  'Each tab is an element with role=tab and data-tab, the active one aria-selected; a click makes it active',
  'focusAddressBar puts the keyboard in a text field where an address and Enter take the active tab there',
  'Every button, field and tab it draws lies in a region it marked v-shell',
  'A press in the middle of a page it shows reaches that page',
  'Some empty space is marked v-drag, and no page lies under the native window buttons',
  'Reloaded, it comes back showing the same pages',
  'Switched to and away from, it starts from Go\'s data alone',
  'It logs no errors',
];

(async () => {
  const browser = await chromium.launch();
  const summary = [];
  for (const id of ids) {
    if (!registry.some((s) => s.id === id)) throw new Error(`no shell ${id} in core/shells/registry.json`);
    console.log(`\n=== the ${id} shell`);
    const broken = new Set();
    // rule records one check of rule n.
    const rule = (n, name, ok, detail) => {
      if (!lib.check(`${n}. ${name}`, ok, detail)) broken.add(n);
      return ok;
    };
    await lib.start([`-use=${id}`]);
    const w = await lib.openWindow(browser);
    const { page, ui } = w;
    page.setDefaultTimeout(5000);
    const tabEls = ui.locator('[role=tab]');

    // The window as Go and the mock have it now, and the tabs as the shell draws them.
    const drawn = () => tabEls.evaluateAll((els) => els.map((el) => ({ tab: Number(el.dataset.tab), selected: el.getAttribute('aria-selected') })));
    const shows = (r) => r.visible.some((v) => v.tab === r.active);
    // The shell is up when it loads this shell, draws a tab for each of Go's, and shows the
    // active one.
    const up = async () => {
      const r = await regions();
      return r.shell === id && (await tabEls.count()) === r.tabs.length && shows(r);
    };

    // 1. ready, the snapshot drawn, shown.
    let ready = await until(up, 10000);
    rule(1, 'it draws the snapshot: a tab for each of Go\'s', ready, JSON.stringify({ drawn: await tabEls.count(), go: (await regions()).tabs.length }));
    rule(1, 'and says shown, so Go marks the app ready', await until(() => /\bapp\.ready\b/.test(lib.log)));

    // 3, for the layout as it is now.
    const insideAndApart = async (when) => {
      const r = await regions();
      const f = r.frameWidth;
      const W = r.window.W;
      const H = r.window.H;
      const outside = r.visible.filter(({ rect: p }) => p.x < f || p.y < f || p.x + p.w > W - f || p.y + p.h > H - f);
      const overlap = r.visible.some((a, i) => r.visible.some((b, j) => j > i &&
        a.rect.x < b.rect.x + b.rect.w && b.rect.x < a.rect.x + a.rect.w && a.rect.y < b.rect.y + b.rect.h && b.rect.y < a.rect.y + a.rect.h));
      const strangers = r.visible.filter((v) => !r.tabs.some((t) => t.id === v.tab));
      rule(3, `${when}: the pages lie inside the frame, apart, and are this window's tabs`, outside.length === 0 && !overlap && strangers.length === 0,
        JSON.stringify({ visible: r.visible, frame: f, window: [W, H], tabs: r.tabs.map((t) => t.id) }));
    };
    // 2, after a change: Go's active tab is one the layout shows, and the dev window shows
    // a page for it.
    const activeShows = async (when) => {
      const ok = await until(async () => {
        const r = await regions();
        return shows(r) && (await page.locator(`iframe.mock-page[data-tab="${r.active}"]`).isVisible());
      });
      rule(2, `${when}: the active tab's page shows`, ok, JSON.stringify(await regions().then((r) => ({ active: r.active, visible: r.visible }))));
      // A page the window doesn't show can't take the keyboard, so the layout that shows it
      // goes first.
      const unshown = (await regions()).unshownFocus;
      rule(2, `${when}: the shell gave no page the keyboard before its layout showed it`, unshown.length === 0, JSON.stringify(unshown));
      await insideAndApart(when);
    };
    const tabCount = async (n) => until(async () => (await regions()).tabs.length === n && (await drawn()).length === n);

    // A check that throws fails the rule it was checking, and leaves the rest unchecked.
    let current = 2;
    if (!ready) {
      for (let n = 2; n <= 10; n++) rule(n, 'not checked: the shell never came up', false);
    } else {
      try {
        await activeShows('at the start');
        await fire('menu', 'tab.new');
        await tabCount(2);
        await activeShows('after a new tab');
        await fire('menu', 'tab.new');
        await tabCount(3);
        await activeShows('after another');
        await fire('menu', 'tab.select1');
        await until(async () => (await regions()).active === (await regions()).tabs[0].id);
        await activeShows('after another tab was made active');
        let r = await regions();
        await fire('open', 'https://opened.test/', { tab: r.active });
        await tabCount(4);
        await activeShows('after a page opened a tab');
        await fire('menu', 'tab.close');
        await tabCount(3);
        await activeShows('after the active tab closed');
        await page.setViewportSize({ width: 900, height: 600 });
        await until(async () => (await regions()).window.W === 900);
        await activeShows('after a resize');
        await page.setViewportSize({ width: 1100, height: 700 });
        await until(async () => (await regions()).window.W === 1100);
        await activeShows('after a resize back');

        // 4. The tabs as elements.
        current = 4;
        r = await regions();
        let d = await drawn();
        rule(4, 'each of Go\'s tabs is an element with role=tab and its data-tab, in order', JSON.stringify(d.map((t) => t.tab)) === JSON.stringify(r.tabs.map((t) => t.id)), JSON.stringify({ drawn: d, go: r.tabs.map((t) => t.id) }));
        rule(4, 'the active one, and only it, is aria-selected', d.every((t) => (t.selected === 'true') === (t.tab === r.active)), JSON.stringify({ drawn: d, active: r.active }));
        const other = r.tabs.find((t) => t.id !== r.active).id;
        await tabEls.nth(r.tabs.findIndex((t) => t.id === other)).click();
        rule(4, 'a click on a tab makes it active', await until(async () => (await regions()).active === other && (await drawn()).find((t) => t.tab === other)?.selected === 'true'), JSON.stringify(await regions().then((r) => r.active)));

        // 5. The address bar.
        current = 5;
        await fire('key', 'focusAddressBar', { tab: other });
        const inField = () => w.shell().evaluate(() => {
          const el = document.activeElement;
          return document.hasFocus() && !!el && (el.tagName === 'INPUT' && /^(text|url|search|)$/.test(el.type) || el.tagName === 'TEXTAREA' || el.isContentEditable);
        });
        rule(5, 'focusAddressBar puts the keyboard in a text field', await until(inField), await w.shell().evaluate(() => document.activeElement?.outerHTML.slice(0, 80)));
        await page.keyboard.press('ControlOrMeta+A');
        await page.keyboard.type('contract.test/typed');
        await page.keyboard.press('Enter');
        rule(5, 'an address and Enter take the active tab there', await until(async () => (await regions()).tabs.find((t) => t.id === other)?.url === 'https://contract.test/typed'), JSON.stringify((await regions()).tabs.find((t) => t.id === other)));
        await activeShows('after a navigation');

        // 6. What the shell draws to be pressed lies in its own regions.
        current = 6;
        await settle(page);
        const controls = await w.shell().evaluate(() => [...document.querySelectorAll('button, input, select, textarea, a[href], [role=tab], [tabindex]')]
          .filter((el) => el.checkVisibility({ visibilityProperty: true, opacityProperty: true }))
          .map((el) => ({ what: el.outerHTML.slice(0, 60), rect: el.getBoundingClientRect().toJSON() }))
          .filter(({ rect }) => rect.width > 0 && rect.height > 0 && rect.right > 0 && rect.bottom > 0 && rect.left < innerWidth && rect.top < innerHeight));
        const outsideShell = [];
        for (const c of controls) {
          const x = Math.min(Math.max(c.rect.x + c.rect.width / 2, 0), 1099);
          const y = Math.min(Math.max(c.rect.y + c.rect.height / 2, 0), 699);
          if ((await regions({ x, y })).hit !== 'shell') outsideShell.push(c.what);
        }
        rule(6, `every one of the ${controls.length} buttons, fields and tabs lies in a v-shell region`, controls.length > 0 && outsideShell.length === 0, outsideShell.join(' | '));

        // 7. A press in the middle of each page it shows.
        current = 7;
        r = await regions();
        for (const v of r.visible) {
          const x = v.rect.x + v.rect.w / 2;
          const y = v.rect.y + v.rect.h / 2;
          // The keyboard goes elsewhere first, as a shell can send it.
          await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'focusTab', id: 0 }) });
          await until(async () => (await regions()).focused === 0);
          // With the right button: the middle of a mock page may hold one of its buttons,
          // which a left press would set off.
          await page.mouse.move(x - 1, y);
          await page.mouse.click(x, y, { button: 'right' });
          rule(7, `a press in the middle of tab ${v.tab}'s page reaches it`, (await regions({ x, y })).hit === 'page' && await until(async () => (await regions()).focused === v.tab), JSON.stringify(await regions({ x, y }).then((h) => ({ hit: h.hit, focused: h.focused }))));
        }
        await activeShows('after a press in a page');

        // 8. Drag space, and the native buttons over no page.
        current = 8;
        r = await regions();
        rule(8, 'some empty space is marked v-drag', r.drag.length > 0, JSON.stringify(r.drag));
        const c = r.window.Controls;
        const under = r.visible.filter(({ rect: p }) => c.w > 0 && p.x < c.x + c.w && c.x < p.x + p.w && p.y < c.y + c.h && c.y < p.y + p.h);
        rule(8, 'no page lies under the native window buttons', under.length === 0, JSON.stringify({ controls: c, under }));

        // 9. Reloaded, the same pages.
        current = 9;
        const shownSet = (r) => JSON.stringify(r.visible.map((v) => v.tab).sort((a, b) => a - b));
        const before = shownSet(await regions());
        await w.shell().evaluate(() => { window.beforeReload = true; location.reload(); });
        const reloaded = await until(async () => (await w.shell()?.evaluate(() => !window.beforeReload).catch(() => false)) && up(), 10000);
        await settle(page);
        const after = shownSet(await regions());
        rule(9, 'reloaded, it comes back showing the same pages', reloaded && before === after, JSON.stringify({ before, after }));

        // 10. Away to another shell and back, through the pref, as View, Shell does.
        current = 10;
        const away = registry.find((s) => s.id !== id).id;
        await fire('menu', `shells:${away}`);
        rule(10, `switched away, the window loads ${away}`, await until(async () => (await regions()).shell === away && shows(await regions()), 10000), JSON.stringify((await regions()).shell));
        await fire('menu', `shells:${id}`);
        ready = await until(up, 10000);
        rule(10, 'switched back, it comes up from Go\'s data: its tabs, and the active one shown', ready, JSON.stringify(await drawn().catch((e) => e.message)));
        await insideAndApart('after switching back');
      } catch (e) {
        rule(current, 'its checks ran to their end', false, e.message.split('\n')[0]);
        for (let n = current + 1; n <= 10; n++) rule(n, 'not checked: an earlier check ended the run', false);
      }
    }

    // 11.
    rule(11, 'it logged no errors', w.errors.length === 0, w.errors.join(' | '));

    await w.page.close();
    await lib.stop();
    rules.forEach((name, i) => summary.push(`${broken.has(i + 1) ? 'FAIL' : 'ok  '} ${id} ${i + 1}. ${name}`));
  }
  await browser.close();
  console.log('\n' + summary.join('\n'));
  await lib.done();
})();
