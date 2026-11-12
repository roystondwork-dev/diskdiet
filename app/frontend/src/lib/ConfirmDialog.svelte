<script lang="ts">
  import { onMount } from "svelte";
  import type { SourceView } from "../../bindings/github.com/roystondwork-dev/diskdiet/internal/appcore/models";
  import { formatBytes, describe } from "./format";

  let {
    sources,
    freeFor,
    volumes,
    olderThanDays,
    total,
    estimate,
    oncancel,
    onconfirm,
  }: {
    sources: SourceView[];
    freeFor: (s: SourceView) => number;
    volumes: boolean;
    olderThanDays: number;
    total: number;
    estimate: boolean;
    oncancel: () => void;
    onconfirm: () => void;
  } = $props();

  let cancelBtn: HTMLButtonElement;
  onMount(() => cancelBtn?.focus());

  function onkey(e: KeyboardEvent) {
    if (e.key === "Escape") oncancel();
  }
</script>

<svelte:window onkeydown={onkey} />

<div class="overlay" role="presentation" onclick={(e) => e.target === e.currentTarget && oncancel()}>
  <div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-desc">
    <h2 id="confirm-title">Free {estimate ? "up to " : ""}{formatBytes(total)}?</h2>
    <p id="confirm-desc" class="muted">
      {olderThanDays === 0 ? "Everything in these caches" : `What hasn't been used for ${olderThanDays} days`} will be
      removed. Package managers download it again when a project needs it.
    </p>
    <ul>
      {#each sources as s (s.id)}
        <li>
          <div>
            <div class="name">{s.name}</div>
            <div class="muted small">{describe(s.id, volumes)}</div>
          </div>
          <div class="num amount">{s.estimate ? "up to " : ""}{formatBytes(freeFor(s))}</div>
        </li>
      {/each}
    </ul>
    <div class="buttons">
      <button class="ghost" bind:this={cancelBtn} onclick={oncancel}>Cancel</button>
      <button class="primary" onclick={onconfirm}>Free {formatBytes(total)}</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgb(0 0 0 / 35%);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }
  .dialog {
    background: var(--surface-1);
    border-radius: 12px;
    box-shadow: var(--shadow);
    padding: 24px;
    width: min(520px, 100%);
    max-height: 100%;
    overflow-y: auto;
  }
  h2 {
    margin: 0 0 6px;
    font-size: 19px;
  }
  .muted {
    color: var(--text-muted);
    margin: 0;
  }
  .small {
    font-size: 12.5px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 16px 0;
  }
  li {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    padding: 10px 0;
    border-top: 1px solid var(--border);
  }
  .name {
    font-weight: 600;
  }
  .amount {
    font-weight: 600;
    white-space: nowrap;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }
  .buttons :global(.ghost) {
    padding: 8px 16px;
    font-size: 14px;
  }
</style>
