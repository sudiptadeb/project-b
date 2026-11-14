<script>
  // Glide's line icons: 16px on a 16 grid, a 1.5px stroke with round caps and joins, in
  // the colour around them. Our own, drawn for this shell.
  let { name, size = 16 } = $props();
  const paths = {
    sidebar: 'M4 3h8a2 2 0 0 1 2 2v6a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2v-6a2 2 0 0 1 2 -2ZM6 3v10',
    back: 'M10 3.5 5.5 8l4.5 4.5',
    forward: 'M6 3.5 10.5 8 6 12.5',
    reload: 'M13 8a5 5 0 1 1-1.6-3.7M13 2.5v3h-3',
    close: 'm4.5 4.5 7 7M11.5 4.5l-7 7',
    plus: 'M8 3v10M3 8h10',
    more: 'M3.4 8a0.6 0.6 0 1 0 1.2 0a0.6 0.6 0 1 0 -1.2 0ZM7.4 8a0.6 0.6 0 1 0 1.2 0a0.6 0.6 0 1 0 -1.2 0ZM11.4 8a0.6 0.6 0 1 0 1.2 0a0.6 0.6 0 1 0 -1.2 0Z',
    link: 'M7 9a3 3 0 0 0 4.2.2l2-2a3 3 0 0 0-4.2-4.2l-.8.8M9 7a3 3 0 0 0-4.2-.2l-2 2a3 3 0 0 0 4.2 4.2l.8-.8',
    folder: 'M2.5 5a1.5 1.5 0 0 1 1.5-1.5h2.5l1.5 1.5h4A1.5 1.5 0 0 1 13.5 6.5V11A1.5 1.5 0 0 1 12 12.5H4A1.5 1.5 0 0 1 2.5 11Z',
    chevronDown: 'm4.5 6.5 3.5 3.5 3.5-3.5',
    chevronRight: 'm6.5 4.5 3.5 3.5-3.5 3.5',
    search: 'M2.5 7a4.5 4.5 0 1 0 9 0a4.5 4.5 0 1 0 -9 0ZM10.5 10.5l3 3',
    tab: 'M4 3.5h8a1.5 1.5 0 0 1 1.5 1.5v6a1.5 1.5 0 0 1 -1.5 1.5h-8a1.5 1.5 0 0 1 -1.5 -1.5v-6a1.5 1.5 0 0 1 1.5 -1.5ZM2.5 6.5h11',
    pin: 'M9.5 2.5 13.5 6.5 11 7.5 8.5 10l.5 3-6-6 3 .5L8.5 5ZM5.5 10.5l-3 3',
    split: 'M4 3h8a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1 -1.5 1.5h-8a1.5 1.5 0 0 1 -1.5 -1.5v-7a1.5 1.5 0 0 1 1.5 -1.5ZM8 3v10',
    clock: 'M2.5 8a5.5 5.5 0 1 0 11 0a5.5 5.5 0 1 0 -11 0ZM8 5v3l2 1.5',
    bolt: 'M9 2.5 4 9h4l-1 4.5L12 7H8Z',
    globe: 'M2.5 8a5.5 5.5 0 1 0 11 0a5.5 5.5 0 1 0 -11 0ZM2.5 8h11M8 2.5c1.6 1.6 2.3 3.5 2.3 5.5S9.6 11.9 8 13.5C6.4 11.9 5.7 10 5.7 8S6.4 4.1 8 2.5Z',
    star: 'm8 2.5 1.7 3.5 3.8.5-2.8 2.6.7 3.8L8 11.1l-3.4 1.8.7-3.8-2.8-2.6 3.8-.5Z',
    gear: 'M6 8a2 2 0 1 0 4 0a2 2 0 1 0 -4 0ZM8 2v1.5M8 12.5V14M2 8h1.5M12.5 8H14M3.8 3.8l1 1M11.2 11.2l1 1M3.8 12.2l1-1M11.2 4.8l1-1',
    extensions: 'M12.5 7.5H12V5.2a.7.7 0 0 0-.7-.7H9v-.6a1.4 1.4 0 0 0-2.8 0v.6H3.7a.7.7 0 0 0-.7.7v2.2h.6a1.4 1.4 0 0 1 0 2.8H3v2.3c0 .4.3.7.7.7H6v-.6a1.4 1.4 0 0 1 2.8 0v.6h2.5c.4 0 .7-.3.7-.7v-2.3h.5a1.4 1.4 0 0 0 0-2.8Z',
    update: 'M8 2.5v8M5 7.5l3 3 3-3M3.5 13h9',
    warn: 'M8 2.5 14 13H2ZM8 6.5v3M8 11.2h.01',
    space: 'M3.5 8a4.5 4.5 0 1 0 9 0a4.5 4.5 0 1 0 -9 0Z',
  };
</script>

<svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  <path d={paths[name]} />
</svg>
