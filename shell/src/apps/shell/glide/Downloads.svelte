<script>
  import { clearDownloads, downloadAction, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';

  // The Downloads control, there once the profile has a download, ringed with the running
  // ones' progress, and the list it opens beside it: each download by its file name, its
  // progress or how it ended, and what can be done to it (docs/daily-driver.md).
  let open = $state(false);
  let button = $state();
  let at = $state({ left: 0, top: 0, bottom: 0, up: false });

  const running = $derived(go.downloads.filter((d) => d.state === 'progress'));
  const progress = $derived.by(() => {
    if (!running.length) return 0;
    if (running.some((d) => d.total <= 0)) return -1;
    return running.reduce((n, d) => n + d.received, 0) / running.reduce((n, d) => n + d.total, 0);
  });

  // A size as Finder gives it: 1000 to a kilobyte, one decimal place under ten.
  function size(n) {
    if (n < 1000) return `${n} B`;
    for (const [unit, by] of [['KB', 1e3], ['MB', 1e6], ['GB', 1e9], ['TB', 1e12]]) {
      const v = n / by;
      if (v < 1000 || unit === 'TB') return `${v < 10 ? +v.toFixed(1) : Math.round(v)} ${unit}`;
    }
  }

  function status(d) {
    if (d.state === 'progress') return (d.paused ? 'Paused, ' : '') + (d.total > 0 ? `${size(d.received)} of ${size(d.total)}` : size(d.received));
    if (d.state === 'done') return d.total > 0 ? size(d.total) : 'Done';
    return d.state === 'cancelled' ? 'Cancelled' : 'Failed';
  }

  function show() {
    const r = button.getBoundingClientRect();
    const up = r.top > innerHeight / 2;
    at = { left: Math.min(r.left, innerWidth - 348), top: r.bottom + 6, bottom: innerHeight - r.top + 6, up };
    open = true;
  }

  function close(refocus) {
    open = false;
    if (refocus) button?.focus();
  }

  $effect(() => {
    if (open) return pushEscape(() => close(true));
  });
  // The list emptied, or another tab shown, closes it.
  $effect(() => {
    if (!go.downloads.length) open = false;
  });
  let lastActive = go.active;
  $effect(() => {
    if (go.active !== lastActive) {
      lastActive = go.active;
      open = false;
    }
  });
</script>

{#if go.downloads.length}
  <button bind:this={button} class="g-icon downloads" aria-label="Downloads" title="Downloads" aria-haspopup="dialog" aria-expanded={open} onclick={() => (open ? close(true) : show())}>
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 2.5v8M4.5 7 8 10.5 11.5 7M3 13.5h10" /></svg>
    {#if running.length}
      <svg class="ring" width="26" height="26" viewBox="0 0 26 26" aria-hidden="true">
        <circle cx="13" cy="13" r="11.5" fill="none" stroke="var(--line)" stroke-width="2" />
        <circle cx="13" cy="13" r="11.5" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-dasharray="72.3" stroke-dashoffset={progress < 0 ? 54 : 72.3 * (1 - progress)} transform="rotate(-90 13 13)" />
      </svg>
    {/if}
  </button>
{/if}

{#if open}
  <div class="cover" use:shell role="presentation" onpointerdown={(e) => e.target === e.currentTarget && close(false)}>
    <div class="panel" role="dialog" aria-label="Downloads" style:left="{at.left}px" style:top={at.up ? 'auto' : at.top + 'px'} style:bottom={at.up ? at.bottom + 'px' : 'auto'}>
      <div class="head">
        <span>Downloads</span>
        <button type="button" class="text-button quiet" disabled={!go.downloads.some((d) => d.state !== 'progress')} onclick={() => clearDownloads()}>Clear</button>
      </div>
      <ul role="list">
        {#each go.downloads as d (d.id)}
          <li role="listitem" aria-label={d.name} class={d.state}>
            <svg class="file" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><path d="M4 1.75h5L12.25 5v9.25H4z" /><path d="M9 1.75V5h3.25" /></svg>
            <div class="name" title={d.path}>{d.name}</div>
            {#if d.state === 'progress'}
              <div class="bar" role="progressbar" aria-label={d.name} aria-valuemin="0" aria-valuemax={d.total > 0 ? d.total : undefined} aria-valuenow={d.received}>
                <span style:width={d.total > 0 ? (100 * d.received) / d.total + '%' : '30%'}></span>
              </div>
            {/if}
            <div class="status">{status(d)}</div>
            <div class="actions">
              {#if d.state === 'done'}
                <button type="button" class="text-button quiet" onclick={() => downloadAction(d.id, 'open')}>Open</button>
                <button type="button" class="text-button quiet" onclick={() => downloadAction(d.id, 'reveal')}>Show in Finder</button>
              {:else if d.state === 'progress'}
                <button type="button" class="text-button quiet" onclick={() => downloadAction(d.id, d.paused ? 'resume' : 'pause')}>{d.paused ? 'Resume' : 'Pause'}</button>
                <button type="button" class="text-button quiet" onclick={() => downloadAction(d.id, 'cancel')}>Cancel</button>
              {/if}
              <button type="button" class="g-icon remove" aria-label="Remove {d.name} from the list" title="Remove from the list" onclick={() => downloadAction(d.id, 'remove')}>
                <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
              </button>
            </div>
          </li>
        {/each}
      </ul>
    </div>
  </div>
{/if}

<style>
  .downloads {
    position: relative;
  }

  .ring {
    position: absolute;
    inset: 50% auto auto 50%;
    transform: translate(-50%, -50%);
    pointer-events: none;
  }

  .cover {
    position: fixed;
    inset: 0;
    z-index: 100;
  }

  .panel {
    position: fixed;
    width: 340px;
    max-height: min(480px, calc(100vh - 80px));
    overflow: auto;
    padding: 4px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
    box-shadow: var(--shadow-float);
    color: var(--text);
    font-size: 13px;
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    padding: 0 4px 0 12px;
    font-weight: 600;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: grid;
    grid-template-columns: 16px 1fr auto;
    gap: 2px 10px;
    padding: 8px 4px 8px 12px;
  }

  li + li {
    border-top: 1px solid var(--line);
  }

  .file {
    grid-row: 1;
    grid-column: 1;
    color: var(--muted);
  }

  .name {
    grid-column: 2;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  li.cancelled .name,
  li.failed .name {
    color: var(--muted);
    text-decoration: line-through;
  }

  .bar {
    grid-column: 2;
    height: 4px;
    overflow: hidden;
    border-radius: 2px;
    background: var(--accent-tint);
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--accent);
  }

  .status {
    grid-column: 2;
    color: var(--muted);
    font-size: 12px;
  }

  .actions {
    display: flex;
    grid-column: 3;
    grid-row: 1 / span 3;
    align-items: center;
    gap: 8px;
  }

  .text-button {
    height: 26px;
    padding: 0 8px;
    font-size: 12px;
  }
</style>
