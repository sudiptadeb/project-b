<script>
  // The shell's line icons: 20px on a 24px box at a 1.5px stroke, in the colour around them.
  let { name, size = 20 } = $props();
  const paths = {
    back: 'M15 5l-7 7 7 7',
    forward: 'M9 5l7 7-7 7',
    reload: 'M19 12a7 7 0 1 1-2.05-4.95M19 4.5V8h-3.5',
    stop: 'M6 6l12 12M18 6L6 18',
    plus: 'M12 5v14M5 12h14',
    close: 'M7 7l10 10M17 7L7 17',
    lock: 'M7.5 11V8.5a4.5 4.5 0 0 1 9 0V11M6.5 11h11v8h-11z',
    warn: 'M12 4l8.5 15h-17zM12 10v4M12 16.8h.01',
    search: 'M10.5 17a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13zM15.5 15.5L20 20',
    globe: 'M12 20a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM4 12h16M12 4c2.2 2.2 3.2 5 3.2 8s-1 5.8-3.2 8c-2.2-2.2-3.2-5-3.2-8s1-5.8 3.2-8z',
    fill: 'M7 7h7M7 7v7M7 7l10 10',
    settings: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 13.5l1.6 1.2-2 3.4-1.9-.7a7 7 0 0 1-2 1.2L14.8 21h-4l-.3-2a7 7 0 0 1-2-1.2l-1.9.7-2-3.4 1.6-1.2a7 7 0 0 1 0-2.4L4.6 9.9l2-3.4 1.9.7a7 7 0 0 1 2-1.2l.3-2h4l.3 2a7 7 0 0 1 2 1.2l1.9-.7 2 3.4-1.6 1.2a7 7 0 0 1 0 2.4z',
    workspace: 'M12 19a7 7 0 1 0 0-14 7 7 0 0 0 0 14z',
    extensions: 'M19.5 11H18V7.5A1.5 1.5 0 0 0 16.5 6H13V4.5a2 2 0 0 0-4 0V6H5.5A1.5 1.5 0 0 0 4 7.5V11h1.5a2 2 0 0 1 0 4H4v3.5A1.5 1.5 0 0 0 5.5 20H9v-1.5a2 2 0 0 1 4 0V20h3.5a1.5 1.5 0 0 0 1.5-1.5V15h1.5a2 2 0 0 0 0-4z',
    chevron: 'M10 7l5 5-5 5',
    group: 'M5 5h14v14H5zM12 9v6M9 12h6',
    update: 'M12 4v11M7.5 10.5L12 15l4.5-4.5M5 19h14',
    link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
    copy: 'M9 9h10v10H9zM5 15V5h10',
    tab: 'M4 6h16v13H4zM4 10h16',
    image: 'M4 5h16v14H4zM4 16l5-5 4 4 2-2 5 5M15.5 9.5h.01',
  };
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === 'more'}
    <g fill="currentColor" stroke="none"><circle cx="12" cy="6" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="12" cy="18" r="1.6" /></g>
  {:else}
    <path d={paths[name]} />
  {/if}
</svg>
