<script>
  import { closeFind, findNext, findText, focusPage, keyLabel, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';

  // Find in page (docs/daily-driver.md): a bar hung from the active card's top right while
  // its tab's search is open, the count at the field's end. Enter shows the next match,
  // Shift+Enter the one before, Escape closes it. focus() is the find key's answer.
  let field = $state();
  let text = $state('');

  const open = $derived(!!go.find && go.find.tab === go.active);
  const missed = $derived(!!go.find?.text && !go.find.matches);
  const count = $derived(!go.find?.text ? '' : go.find.matches ? `${go.find.active} of ${go.find.matches}` : 'No results');
  const tip = (title, action) => (keyLabel(go, action) ? `${title} (${keyLabel(go, action)})` : title);

  // The field follows Go's text when another tab's search shows, not while typing.
  let shownTab = 0;
  $effect(() => {
    if (go.find && go.find.tab !== shownTab) {
      shownTab = go.find.tab;
      text = go.find.text;
    }
    if (!go.find) shownTab = 0;
  });
  $effect(() => {
    if (open) return pushEscape(close);
  });

  export function focus() {
    send({ t: 'focusShell' });
    queueMicrotask(() => {
      field?.focus();
      field?.select();
    });
  }

  function close() {
    closeFind(go.find?.tab);
    focusPage();
  }

  function onkeydown(e) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    findNext(go.find.tab, !e.shiftKey);
  }
</script>

{#if open}
  <div class="find" use:shell role="search" aria-label="Find in page">
    <div class="box" class:missed>
      <input bind:this={field} bind:value={text} class="text-field" type="text" aria-label="Find in page" placeholder="Find in page" spellcheck="false" oninput={() => findText(go.find.tab, text)} {onkeydown} />
      <span class="count" role="status">{count}</span>
    </div>
    <button type="button" class="g-icon" aria-label="Previous match" title={tip('Previous', 'find.previous')} disabled={!go.find.matches} onclick={() => findNext(go.find.tab, false)}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 10l4-4 4 4" /></svg>
    </button>
    <button type="button" class="g-icon" aria-label="Next match" title={tip('Next', 'find.next')} disabled={!go.find.matches} onclick={() => findNext(go.find.tab, true)}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6l4 4 4-4" /></svg>
    </button>
    <button type="button" class="g-icon" aria-label="Close find bar" title="Close (Esc)" onclick={close}>
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
    </button>
  </div>
{/if}

<style>
  .find {
    position: absolute;
    top: 8px;
    right: 8px;
    z-index: 4;
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 6px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
    box-shadow: var(--shadow-float);
    color: var(--text);
  }

  .box {
    position: relative;
    margin-right: 4px;
  }

  .text-field {
    width: 260px;
    height: 28px;
    padding-right: 84px;
    box-sizing: border-box;
  }

  .count {
    position: absolute;
    top: 50%;
    right: 10px;
    transform: translateY(-50%);
    color: var(--muted);
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
  }

  .missed .text-field {
    border-color: var(--danger);
  }

  .missed .count {
    color: var(--danger);
  }
</style>
