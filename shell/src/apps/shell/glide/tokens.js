// Glide's colours, all from one: the current Space's. The shared tones (tones.js) give the
// floating things theirs, as on every page; Glide adds its own, prefixed --g-, for the frame
// and the sidebar on it. Flat: the frame is one tone of the Space's colour, no gradient.
import { tones } from '@/tones.js';

const rgbOf = (c) => [1, 3, 5].map((i) => parseInt(c.slice(i, i + 2), 16));
const hex = (c) => '#' + c.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
const mix = (from, to, t) => from.map((v, i) => v + ((Array.isArray(to) ? to[i] : to) - v) * t);

function luminance(c) {
  const [r, g, b] = c.map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

// The near-black dark mode is made on, as tones.js's.
const base = [27, 28, 30];

// glideTones is every colour of the window for a Space of colour color, light or dark: the
// shared tones and Glide's own, as CSS variables.
export function glideTones(color, dark) {
  const c = rgbOf(color || '#4f9e9e');
  const shared = tones(hex(c), dark);
  if (dark) {
    const frame = mix(c, base, 0.5);
    return {
      ...shared,
      '--g-frame': hex(frame),
      '--g-ink': '#e8eaed',
      '--g-ink-muted': 'rgba(232, 234, 237, 0.6)',
      '--g-icon': hex(mix(c, 255, 0.62)),
      '--g-line': 'rgba(255, 255, 255, 0.12)',
      '--g-field': 'rgba(255, 255, 255, 0.08)',
      '--g-tile': 'rgba(255, 255, 255, 0.07)',
      '--g-row-hover': 'rgba(255, 255, 255, 0.07)',
      '--g-row-active': 'rgba(255, 255, 255, 0.15)',
      '--g-active-ink': '#ffffff',
      '--g-scrim': 'rgba(0, 0, 0, 0.36)',
      '--g-float-shadow': '0 16px 48px rgba(0, 0, 0, 0.5), 0 1px 3px rgba(0, 0, 0, 0.3)',
    };
  }
  // Light: the frame is the colour lifted toward white, the ink a deep tone of it. Where a
  // light colour leaves the ink short of WCAG's 4.5 on the frame, the ink goes darker.
  const frame = mix(c, 255, 0.38);
  let ink = mix(c, [20, 22, 26], 0.82);
  for (let t = 0.05; contrast(ink, frame) < 4.5 && t <= 1; t += 0.05) ink = mix(ink, 0, t);
  return {
    ...shared,
    '--g-frame': hex(frame),
    '--g-ink': hex(ink),
    '--g-ink-muted': `rgba(${ink.map(Math.round).join(', ')}, 0.72)`,
    '--g-icon': hex(mix(c, 0, 0.55)),
    '--g-line': 'rgba(15, 20, 25, 0.12)',
    '--g-field': 'rgba(255, 255, 255, 0.42)',
    '--g-tile': 'rgba(255, 255, 255, 0.32)',
    '--g-row-hover': 'rgba(255, 255, 255, 0.3)',
    '--g-row-active': 'rgba(255, 255, 255, 0.88)',
    '--g-active-ink': '#1f1f1f',
    '--g-scrim': 'rgba(16, 24, 40, 0.28)',
    '--g-float-shadow': '0 16px 48px rgba(16, 24, 40, 0.22), 0 1px 3px rgba(16, 24, 40, 0.1)',
  };
}

// The style attribute that puts them on an element.
export const styleOf = (vars) => Object.entries(vars).map(([k, v]) => `${k}: ${v}`).join('; ');
