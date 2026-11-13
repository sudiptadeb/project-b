// The Show window regions flag on the mock: the window page outlines every rectangle the
// window manager tracks, each kind in the real window's colour, over everything and taking
// no presses; with the flag off it outlines nothing. The flag is read at launch, from the
// profile's flags.json, as the real engine's switches are.
// Run: node shell/e2e/outlines.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

// Each kind's border, as docs/windows.md has them.
const look = {
  frame: ['#9ca3af', '2 2'],
  page: ['#16a34a', null],
  shell: ['#dc2626', null],
  drag: ['#2563eb', '4 3'],
  buttons: ['#a855f7', null],
  settings: ['#f97316', null],
};

const flags = (value) => fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify(value));

// drawn is every outline the window page draws, with the rectangle it outlines.
const drawn = (page) => page.evaluate(() => [...document.querySelectorAll('.outlines rect')].map((el) => ({
  kind: el.dataset.kind,
  stroke: el.getAttribute('stroke'),
  dash: el.getAttribute('stroke-dasharray'),
  rect: { x: +el.getAttribute('x') - 0.5, y: +el.getAttribute('y') - 0.5, w: +el.getAttribute('width') + 1, h: +el.getAttribute('height') + 1 },
})));

const key = (r) => `${r.x},${r.y},${r.w},${r.h}`;
// same is whether the outlines of kind lie exactly on rects, one each.
const same = (outlines, kind, rects) =>
  JSON.stringify(outlines.filter((o) => o.kind === kind).map((o) => key(o.rect)).sort()) === JSON.stringify(rects.map(key).sort());

// tracked is what the outlines should be, from what the mock says the window tracks.
const tracked = (r) => ({
  frame: [{ x: r.frameWidth, y: r.frameWidth, w: r.window.W - 2 * r.frameWidth, h: r.window.H - 2 * r.frameWidth }],
  page: r.visible.map((v) => v.rect),
  shell: r.regions,
  drag: r.drag,
  buttons: r.window.Controls.w > 0 ? [r.window.Controls] : [],
  settings: r.settingsOpen ? [{ x: 0, y: 0, w: r.window.W, h: r.window.H }] : [],
});

const outlinesMatch = async (page, when) => {
  const r = await regions();
  const want = tracked(r);
  const got = await drawn(page);
  for (const kind of Object.keys(look)) {
    check(`${when}: one ${kind} outline on each of its ${want[kind].length} rectangles`, same(got, kind, want[kind]),
      JSON.stringify({ drawn: got.filter((o) => o.kind === kind).map((o) => o.rect), tracked: want[kind] }));
  }
  check(`${when}: each kind in its colour and dash`, got.every((o) => o.stroke === look[o.kind][0] && o.dash === look[o.kind][1]), JSON.stringify(got.map((o) => [o.kind, o.stroke, o.dash])));
  return r;
};

(async () => {
  const browser = await chromium.launch();

  flags({ showRegions: 'on' });
  await lib.start();
  let w = await lib.openWindow(browser);
  check('the shell comes up with one tab shown', await until(async () => (await regions()).visible.length === 1));
  await settle(w.page);
  const r = await outlinesMatch(w.page, 'one tab');
  await w.page.screenshot({ path: path.join(lib.out, 'outlines.png') });
  const tab = r.visible[0];
  const label = await w.page.locator('.outlines .label').evaluateAll((els) => els.map((el) => [el.textContent, el.getBoundingClientRect().x, el.getBoundingClientRect().y]));
  check('the page outline says its tab at its top left', JSON.stringify(label) === JSON.stringify([[`tab ${tab.tab}`, tab.rect.x + 3, tab.rect.y + 2]]), JSON.stringify(label));
  // The outlines take no presses: under the middle of each outlined rectangle is what the
  // window holds there, the shell's frame or a page's.
  const points = [tab.rect, ...r.regions, ...r.drag].map((g) => ({ x: g.x + g.w / 2, y: g.y + g.h / 2 }));
  check('the outlines take no presses', await w.page.evaluate((points) => points.every(({ x, y }) => document.elementFromPoint(x, y).matches('iframe')), points));

  await lib.fire('menu', 'app.settings');
  check('settings opens', await until(async () => (await regions()).settingsOpen));
  await until(async () => (await drawn(w.page)).some((o) => o.kind === 'settings'));
  await outlinesMatch(w.page, 'the settings page open');
  await w.page.screenshot({ path: path.join(lib.out, 'outlines-settings.png') });
  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  flags({});
  await lib.start();
  w = await lib.openWindow(browser);
  await until(async () => (await regions()).visible.length === 1);
  await settle(w.page);
  check('with the flag off nothing is outlined', (await drawn(w.page)).length === 0 && (await w.page.locator('.outlines').isHidden()));
  await w.page.close();
  await lib.stop();

  await browser.close();
  await lib.done();
})();
