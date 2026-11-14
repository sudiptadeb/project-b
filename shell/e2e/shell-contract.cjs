// The check every shell passes: "What every shell does" in docs/shells.md, rule by rule, on
// the mock engine in a real browser, with the Extensions flag on. It knows nothing of a
// shell but what the rules say: the tabs are role=tab with data-tab, the basic controls
// are found by their role and name, pages show where the layout puts them, and Go's state
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

const rules = {
  1: 'It says ready, draws the snapshot, and says shown',
  2: "The active tab's page shows, after every change, and no page is given the keyboard before it shows",
  3: "The pages it shows lie inside the band it asked for, don't overlap, and are all tabs of this window",
  4: 'Each tab is an element with role=tab and data-tab, the active one aria-selected; a click makes it active, on the shell or in a sheet one of its controls opens',
  5: 'focusAddressBar puts the keyboard in the field named Address, where an address and Enter take the active tab there',
  6: 'Every control it draws is the shell\'s: a press on it reaches the shell',
  7: 'A press in the middle of a page it shows reaches that page',
  8: 'Some space is marked drag, and no page lies under the native window buttons',
  9: 'Reloaded, it comes back showing the same pages',
  10: 'Switched to and away from, it starts from Go\'s data alone',
  11: 'Settings is one press away, or two: a control named Settings, on the shell or in a menu one of its controls opens, opens the settings page',
  12: 'The basic controls are somewhere in the shell, found by role and name, and each does its job',
  13: 'It logs no errors',
  15: 'A page\'s alert, confirm and prompt show as an alertdialog with Go\'s buttons by name, a prompt with a field; the answer reaches the page; another tab shown hides it, and back shows it',
  16: 'On a desktop, a right click in a page opens a menu of Go\'s actions there; choosing one makes Go do it; Escape, or a press outside, closes it',
  17: 'On a desktop, a control named Downloads shows once there is a download, and opens a list of them by file name, a finished one with Open and Show in Finder',
  20: 'On a desktop, the find key opens a text box named Find in page with the keyboard in it; typing searches and shows the count; Enter shows the next match; Escape closes it',
};

// The desktop shells that don't draw the right-click menu yet, and who brings it to each
// (docs/daily-driver.md). Rule 16 holds every other desktop shell to it.
const noMenuYet = {
  glide: 'r6 brings it',
  terminal: 'r4 brings it',
  fox: 'r4 brings it',
  'minimal-svelte': 'r4 brings it',
  'minimal-js': 'r4 brings it',
  'minimal-vue': 'r4 brings it',
  example: 'r4 brings it',
};

// The shells that don't draw a page's dialogs yet, and who brings each. Rule 15 holds
// every other shell to them.
const noDialogsYet = {
  glide: 'r6 brings it',
  terminal: 'r4 brings it',
  fox: 'r4 brings it',
  'minimal-svelte': 'r4 brings it',
  'minimal-js': 'r4 brings it',
  'minimal-vue': 'r4 brings it',
  example: 'r4 brings it',
  mobile: 'r3 brings it, with the dialogs on Android',
};

// The desktop shells that don't draw a find bar yet, and who brings each. Rule 20 holds
// every other desktop shell to it.
const noFindYet = {
  glide: 'r6 brings it',
  terminal: 'r4 brings it',
  fox: 'r4 brings it',
  'minimal-svelte': 'r4 brings it',
  'minimal-js': 'r4 brings it',
  'minimal-vue': 'r4 brings it',
  example: 'r4 brings it',
};

// The desktop shells that don't draw the downloads list yet, and who brings it to each.
// Rule 17 holds every other desktop shell to it.
const noDownloadsYet = {
  glide: 'r6 brings it',
  terminal: 'r4 brings it',
  fox: 'r4 brings it',
  'minimal-svelte': 'r4 brings it',
  'minimal-js': 'r4 brings it',
  'minimal-vue': 'r4 brings it',
  example: 'r4 brings it',
};

(async () => {
  // What a control is, as the SDK has it: the same selector it finds them in a drag bar by.
  const { controls: control } = await import('../src/sdk/controls.js');
  const browser = await chromium.launch();
  const summary = [];
  // The extensions' buttons show only while their flag is on, as the settings page saves it.
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ extensions: 'on' }));
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
      rule(3, `${when}: the pages lie inside the band, apart, and are this window's tabs`, outside.length === 0 && !overlap && strangers.length === 0,
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
    // The settings page closed with its own button, as settings.cjs closes it.
    const closeSettings = async () => {
      await page.frameLocator('iframe.mock-settings').getByRole('button', { name: 'Close settings' }).click();
      await until(async () => (await regions()).settingsOpen === false);
    };

    // A check that throws fails the rule it was checking, and leaves the rest unchecked.
    let current = 2;
    if (!ready) {
      for (let n = 2; n <= 12; n++) rule(n, 'not checked: the shell never came up', false);
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
        // A tab may be in a sheet one of the shell's controls opens, as a phone's switcher
        // is: each is pressed in turn until the tab shows, and closed again with Escape.
        const otherEl = tabEls.nth(r.tabs.findIndex((t) => t.id === other));
        for (const opener of await ui.locator('[aria-haspopup], [aria-expanded]').filter({ visible: true }).all()) {
          if (await otherEl.isVisible()) break;
          await opener.click();
          if (!(await until(() => otherEl.isVisible(), 500))) await page.keyboard.press('Escape');
        }
        await otherEl.click();
        rule(4, 'a click on a tab makes it active', await until(async () => (await regions()).active === other && (await drawn()).find((t) => t.tab === other)?.selected === 'true'), JSON.stringify(await regions().then((r) => r.active)));
        // Tab groups are Go's, whatever the shell makes of them (docs/groups.md): with two
        // tabs in one, every tab is still an element, and a click still makes it active.
        r = await regions();
        await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'createGroup', ids: r.tabs.slice(0, 2).map((t) => t.id) }) });
        await settle(page);
        d = await drawn();
        rule(4, 'with two tabs in a group, each tab is still an element', r.tabs.every((t) => d.some((x) => x.tab === t.id)), JSON.stringify(d));
        const grouped = r.tabs[1].id === r.active ? r.tabs[0].id : r.tabs[1].id;
        const groupedEl = tabEls.nth((await drawn()).findIndex((t) => t.tab === grouped));
        for (const opener of await ui.locator('[aria-haspopup], [aria-expanded]').filter({ visible: true }).all()) {
          if (await groupedEl.isVisible()) break;
          await opener.click();
          if (!(await until(() => groupedEl.isVisible(), 500))) await page.keyboard.press('Escape');
        }
        await groupedEl.click();
        rule(4, 'and a click on one in the group makes it active', await until(async () => (await regions()).active === grouped), JSON.stringify(await regions().then((r) => r.active)));
        await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'activateTab', id: other }) });
        await until(async () => (await regions()).active === other);
        await settle(page);

        // 5. The address bar.
        current = 5;
        await fire('key', 'focusAddressBar', { tab: other });
        const address = ui.getByRole('textbox', { name: 'Address', exact: true }).or(ui.getByRole('combobox', { name: 'Address', exact: true }));
        const inField = () => address.evaluateAll((els) => document.hasFocus() && els.includes(document.activeElement));
        rule(5, 'focusAddressBar puts the keyboard in the text field named Address', await until(inField), await w.shell().evaluate(() => document.activeElement?.outerHTML.slice(0, 80)));
        await page.keyboard.press('ControlOrMeta+A');
        await page.keyboard.type('contract.test/typed');
        await page.keyboard.press('Enter');
        rule(5, 'an address and Enter take the active tab there', await until(async () => (await regions()).tabs.find((t) => t.id === other)?.url === 'https://contract.test/typed'), JSON.stringify((await regions()).tabs.find((t) => t.id === other)));
        await activeShows('after a navigation');

        // 6. What the shell draws to be pressed is the shell's.
        current = 6;
        // The layout follows what the shell draws a frame or more later, so this looks
        // until every control is the shell's.
        let controls = [];
        let outsideShell = [];
        await until(async () => {
          controls = await w.shell().evaluate((control) => [...document.querySelectorAll(control)]
            .filter((el) => el.checkVisibility({ visibilityProperty: true, opacityProperty: true }))
            .map((el) => ({ what: el.outerHTML.slice(0, 60), rect: el.getBoundingClientRect().toJSON() }))
            .filter(({ rect }) => rect.width > 0 && rect.height > 0 && rect.right > 0 && rect.bottom > 0 && rect.left < innerWidth && rect.top < innerHeight), control);
          outsideShell = [];
          for (const c of controls) {
            const x = Math.min(Math.max(c.rect.x + c.rect.width / 2, 0), 1099);
            const y = Math.min(Math.max(c.rect.y + c.rect.height / 2, 0), 699);
            if ((await regions({ x, y })).hit !== 'shell') outsideShell.push(c.what);
          }
          return controls.length > 0 && outsideShell.length === 0;
        });
        rule(6, `every one of the ${controls.length} controls is the shell's`, controls.length > 0 && outsideShell.length === 0, outsideShell.join(' | '));

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
        rule(8, 'some space is marked drag', r.drag.length > 0, JSON.stringify(r.drag));
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

        // 11. Settings: a control named so, on the shell or in a menu one of its controls
        // opens. A menu item may show its key after the name.
        current = 11;
        const named = /^Settings\b/;
        const settings = ui.getByRole('button', { name: named }).or(ui.getByRole('menuitem', { name: named })).filter({ visible: true });
        // None on the shell: each control that opens something is pressed in turn, until
        // one shows it.
        const openers = await ui.locator('[aria-haspopup], [aria-expanded]').filter({ visible: true }).all();
        // One that shows nothing is closed again with Escape, as rule 12 does.
        while ((await settings.count()) === 0 && openers.length > 0) {
          await openers.shift().click();
          if (!(await until(async () => (await settings.count()) > 0, 500))) await page.keyboard.press('Escape');
        }
        const found = (await settings.count()) > 0;
        rule(11, 'a control named Settings is on the shell, or in a menu one of its controls opens', found);
        if (found) {
          await settings.first().click();
          rule(11, 'pressed, it opens the settings page', await until(async () => (await regions()).settingsOpen === true));
          // Closed again with the page's own button, as settings.cjs closes it.
          await closeSettings();
        }

        // 12. The basic controls, each found only by its role and its name, as a screen
        // reader finds it, on the shell or in a menu one of its controls opens, and pressed
        // or typed into to see it work.
        current = 12;
        // A control is a button or a menu item, whose name may end with its key.
        const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const byName = (name) => {
          const pattern = new RegExp(`^(${name})\\b`);
          return ui.getByRole('button', { name: pattern }).or(ui.getByRole('menuitem', { name: pattern }));
        };
        // reach is the first of what locator finds that shows: on the shell, or once one of
        // the shell's controls that open something is pressed. One that shows nothing is
        // closed again with Escape. null when none shows it.
        const reach = async (locator) => {
          const shown = locator.filter({ visible: true });
          if (await shown.count()) return shown.first();
          for (const opener of await ui.locator('[aria-haspopup], [aria-expanded]').filter({ visible: true }).all()) {
            await opener.click();
            if (await until(async () => (await shown.count()) > 0, 500)) return shown.first();
            await page.keyboard.press('Escape');
          }
          return null;
        };
        // basic checks one control: that it is there, and that pressing it does what done
        // waits for.
        const basic = async (what, locator, press, done) => {
          const found = await reach(locator);
          if (!rule(12, `${what}: found by its role and name`, !!found)) return;
          const which = await found.evaluate((el) => el.outerHTML.slice(0, 80));
          await press(found);
          rule(12, `${what}: does its job`, await done(), which);
        };
        const click = (el) => el.click();
        const active = async () => {
          const r = await regions();
          return r.tabs.find((t) => t.id === r.active);
        };
        // Whether the active tab has loaded url.
        const at = (url) => until(async () => {
          const t = await active();
          return t.url === url && !t.loading;
        });

        // History to go back and forward in, made as a page makes it.
        const tab = (await regions()).active;
        await fire('navigate', 'https://contract.test/one', { tab });
        await at('https://contract.test/one');
        await fire('navigate', 'https://contract.test/two', { tab });
        await at('https://contract.test/two');
        await basic('Back', byName('Back'), click, () => at('https://contract.test/one'));
        await basic('Forward', byName('Forward'), click, () => at('https://contract.test/two'));
        // Reload loads the page again, which its mock page shows.
        const mockPage = await (await page.locator(`iframe.mock-page[data-tab="${tab}"]`).elementHandle()).contentFrame();
        await mockPage.evaluate(() => {
          const shown = document.getElementById('state');
          window.states = [];
          new MutationObserver(() => window.states.push(shown.textContent)).observe(shown, { childList: true, characterData: true, subtree: true });
        });
        await basic('Reload', byName('Reload'), click, async () =>
          await until(() => mockPage.evaluate(() => window.states.includes('Loading'))) && at('https://contract.test/two'));

        const had = (await regions()).tabs.map((t) => t.id);
        let opened = 0;
        await basic('New Tab', byName('New Tab'), click, () => until(async () => {
          const r = await regions();
          opened = r.tabs.find((t) => !had.includes(t.id))?.id || 0;
          return opened !== 0 && r.active === opened;
        }));
        await basic('the text field named Address', address, async (field) => {
          await field.click();
          await page.keyboard.press('ControlOrMeta+A');
          await page.keyboard.type('close.test/tab');
          await page.keyboard.press('Enter');
        }, () => at('https://close.test/tab'));
        // The tab just opened has a title of its own, which no other tab has.
        await until(async () => (await active()).title !== '');
        const title = (await active()).title;
        await basic(`a way to close a tab, Close Tab or Close ${title}`, byName(`Close Tab|Close ${escaped(title)}`), click,
          () => until(async () => !(await regions()).tabs.some((t) => t.id === opened)));

        await basic('Settings', byName('Settings'), click, () => until(async () => (await regions()).settingsOpen === true));
        await closeSettings();
        // Extensions, whose menu's Manage extensions opens the settings page.
        await basic('Extensions', byName('Extensions'), async (el) => {
          await el.click();
          await byName('Manage extensions').filter({ visible: true }).first().click();
        }, () => until(async () => (await regions()).settingsOpen === true));
        await closeSettings();
      } catch (e) {
        rule(current, 'its checks ran to their end', false, e.message.split('\n')[0]);
        for (let n = current + 1; n <= 12; n++) rule(n, 'not checked: an earlier check ended the run', false);
      }
    }

    // 15. A page's dialogs, on every shell.
    if (noDialogsYet[id]) {
      rule(15, `not checked yet: ${noDialogsYet[id]}`, true);
    } else {
      try {
        const ask = ui.getByRole('alertdialog');
        const answers = async () => (await regions()).dialogAnswers || [];
        const before = (await answers()).length;
        await fire('confirm', 'Go on?');
        rule(15, 'a confirm shows as an alertdialog with its words', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('Go on?')));
        rule(15, "with Go's buttons, OK and Cancel, by name", (await ask.getByRole('button', { name: 'OK' }).count()) === 1 && (await ask.getByRole('button', { name: 'Cancel' }).count()) === 1);
        const home = (await regions()).active;
        await fire('menu', 'tab.new');
        rule(15, 'another tab shown hides it', await until(async () => (await regions()).active !== home && (await ask.count()) === 0));
        await ui.locator(`[role=tab][data-tab="${home}"]`).first().click({ force: true });
        rule(15, 'and showing its tab again brings it back', await until(() => ask.isVisible()));
        await ask.getByRole('button', { name: 'OK' }).click();
        rule(15, 'OK answers the page yes', await until(async () => (await answers()).length === before + 1 && (await answers()).at(-1).ok === true));
        await fire('prompt', 'Name?');
        await until(() => ask.isVisible());
        const field = ask.getByRole('textbox');
        rule(15, 'a prompt has a field', (await field.count()) === 1);
        await field.fill('Ada');
        await ask.getByRole('button', { name: 'OK' }).click();
        rule(15, "and OK answers with the field's text", await until(async () => (await answers()).at(-1)?.text === 'Ada'));
      } catch (e) {
        rule(15, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 16. The right-click menu, on a desktop shell.
    const shell = registry.find((s) => s.id === id);
    if (shell.phone) {
      rule(16, 'not for a phone: a long press keeps its own sheet', true);
    } else if (noMenuYet[id]) {
      rule(16, `not checked yet: ${noMenuYet[id]}`, true);
    } else {
      try {
        const menu = ui.getByRole('menu').first();
        const open = async (link) => {
          await fire('contextMenu', JSON.stringify({ x: 60, y: 60, link }));
          return until(() => menu.isVisible());
        };
        const closed = () => until(async () => (await ui.getByRole('menu').count()) === 0);
        rule(16, 'a right click on a link opens a role=menu', await open('https://contract.test/menu'));
        const copy = ui.getByRole('menuitem', { name: /^Copy Link/ });
        rule(16, "Go's Copy Link is a menuitem by its title", (await copy.count()) === 1, JSON.stringify(await ui.getByRole('menuitem').allInnerTexts()));
        await copy.click();
        rule(16, 'choosing it makes Go do it, and the menu goes', (await until(async () => (await regions()).clipboard === 'https://contract.test/menu')) && (await closed()));
        await open('https://contract.test/escape');
        await menu.press('Escape');
        rule(16, 'Escape closes it', await closed());
        await open('https://contract.test/outside');
        const v = (await regions()).visible[0].rect;
        await page.mouse.click(v.x + v.w - 20, v.y + v.h - 20);
        rule(16, 'a press outside closes it, and nothing ran', (await closed()) && (await regions()).clipboard === 'https://contract.test/menu');
      } catch (e) {
        rule(16, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 17. Downloads, on a desktop shell.
    if (shell.phone) {
      rule(17, 'not for a phone yet: a phone\'s downloads come later', true);
    } else if (noDownloadsYet[id]) {
      rule(17, `not checked yet: ${noDownloadsYet[id]}`, true);
    } else {
      try {
        const control = ui.getByRole('button', { name: 'Downloads' });
        await fire('download', 'https://contract.test/files/contract.pdf');
        rule(17, 'a download brings a control named Downloads', await until(() => control.first().isVisible()));
        await control.first().click();
        const entry = ui.getByRole('listitem', { name: 'contract.pdf' });
        rule(17, 'which opens a list with the download by its file name', await until(() => entry.isVisible()));
        rule(17, 'finished, it has Open and Show in Finder', await until(async () => (await entry.getByRole('button', { name: 'Open' }).count()) === 1 && (await entry.getByRole('button', { name: 'Show in Finder' }).count()) === 1));
        await entry.getByRole('button', { name: 'Show in Finder' }).click();
        rule(17, 'Show in Finder makes Go show the file', await until(async () => (await regions()).shown?.length > 0));
        await page.keyboard.press('Escape');
      } catch (e) {
        rule(17, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 20. Find in page, on a desktop shell.
    if (shell.phone) {
      rule(20, 'not for a phone yet', true);
    } else if (noFindYet[id]) {
      rule(20, `not checked yet: ${noFindYet[id]}`, true);
    } else {
      try {
        const field = ui.getByRole('textbox', { name: 'Find in page' });
        await fire('menu', 'find.open');
        rule(20, 'the find key opens a text box named Find in page, with the keyboard in it', await until(async () => (await field.isVisible()) && (await field.evaluate((el) => el === document.activeElement))));
        await field.pressSequentially('mock');
        const shows = (s) => until(async () => (await ui.locator('body').innerText()).includes(s));
        rule(20, 'typing searches, and the count shows', await shows('3'));
        await field.press('Enter');
        rule(20, 'Enter shows the next match', await shows('2'));
        await field.press('Escape');
        rule(20, 'Escape closes it, and Go ends the search', await until(async () => (await field.count()) === 0 && (await regions()).findsStopped?.length > 0));
      } catch (e) {
        rule(20, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 13.
    rule(13, 'it logged no errors', w.errors.length === 0, w.errors.join(' | '));

    await w.page.close();
    await lib.stop();
    for (const [n, name] of Object.entries(rules)) summary.push(`${broken.has(+n) ? 'FAIL' : 'ok  '} ${id} ${n}. ${name}`);
  }
  await browser.close();
  console.log('\n' + summary.join('\n'));
  await lib.done();
})();
