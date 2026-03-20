<script lang="ts">
  let {
    suggestions = [],
    placeholder = '',
    oncommit,
  }: {
    suggestions: string[];
    placeholder?: string;
    oncommit?: (value: string) => void;
  } = $props();

  let value = $state('');
  let open = $state(false);
  let inputEl: HTMLInputElement | undefined = $state();
  let highlightIdx = $state(-1);

  let filtered = $derived(
    value.trim()
      ? suggestions.filter((s) => s.toLowerCase().includes(value.toLowerCase().trim()))
      : suggestions
  );

  function select(item: string) {
    open = false;
    highlightIdx = -1;
    oncommit?.(item);
    value = '';
  }

  function commit() {
    const v = value.trim();
    if (!v) return;
    open = false;
    highlightIdx = -1;
    oncommit?.(v);
    value = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (highlightIdx >= 0 && highlightIdx < filtered.length) {
        select(filtered[highlightIdx]);
      } else {
        commit();
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open && filtered.length > 0) {
        open = true;
        highlightIdx = 0;
      } else if (highlightIdx < filtered.length - 1) {
        highlightIdx++;
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (highlightIdx > 0) {
        highlightIdx--;
      }
    } else if (e.key === 'Escape') {
      open = false;
      highlightIdx = -1;
    }
  }

  function handleInput() {
    open = suggestions.length > 0 && filtered.length > 0;
    highlightIdx = -1;
  }

  function handleFocus() {
    if (suggestions.length > 0 && filtered.length > 0) {
      open = true;
    }
  }

  function handleBlur() {
    setTimeout(() => {
      open = false;
      highlightIdx = -1;
    }, 150);
  }
</script>

<div class="relative">
  <input
    bind:this={inputEl}
    type="text"
    bind:value
    oninput={handleInput}
    onfocus={handleFocus}
    onblur={handleBlur}
    onkeydown={handleKeydown}
    {placeholder}
    class="px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary w-full"
    autocomplete="off"
  />

  {#if open && filtered.length > 0}
    <ul class="absolute z-50 mt-1 w-full max-h-48 overflow-auto rounded-lg border border-outline-variant/30 bg-surface-container-lowest shadow-lg">
      {#each filtered as item, idx}
        <li>
          <button
            type="button"
            onmousedown={() => select(item)}
            class="w-full text-left px-3 py-2 text-sm cursor-pointer transition-colors {idx === highlightIdx ? 'bg-primary/10 text-primary' : 'text-on-surface hover:bg-surface-container'}"
          >
            {item}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
