// Wrote tones.json from shell/src/tones.js, the JavaScript the Go package replaced, so the
// test holds Go to what every page painted before: node core/tones/testdata/gen.mjs
import { writeFileSync } from 'node:fs';
import { tones } from '../../../shell/src/tones.js';

const hex = (r, g, b) => '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('');
const frames = new Set([
  // The frame colour presets (settings, General) and the workspace colours.
  '#4f9e9e', '#0faaaa', '#5b7fd6', '#faa804', '#d82b1f', '#384562',
  '#3470d0', '#d03434', '#e8a317', '#8a4fd0', '#2e9d5b', '#34404f',
]);
// Custom colours from the picker: a cube, a grey ramp, and a seeded spread.
const steps = [0, 0x33, 0x66, 0x99, 0xcc, 0xff];
for (const r of steps) for (const g of steps) for (const b of steps) frames.add(hex(r, g, b));
for (let v = 0; v < 256; v += 15) frames.add(hex(v, v, v));
let seed = 7;
const next = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) >> 16) % 256;
for (let i = 0; i < 400; i++) frames.add(hex(next(), next(), next()));

const out = {};
for (const f of [...frames].sort()) out[f] = { light: tones(f, false), dark: tones(f, true) };
writeFileSync(new URL('tones.json', import.meta.url), JSON.stringify(out, null, 1) + '\n');
console.log(Object.keys(out).length, 'frames');
