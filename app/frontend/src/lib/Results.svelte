<script lang="ts">
  import type { CleanSummary } from "../../bindings/github.com/roystondwork-dev/diskdiet/internal/appcore/models";
  import { formatBytes } from "./format";

  let { result, onrescan }: { result: CleanSummary; onrescan: () => void } = $props();
  const results = $derived(result.results ?? []);
  const problems = $derived(results.some((r) => r.skipped || (r.errors?.length ?? 0) > 0));
</script>

<section aria-live="polite">
  <div class="hero-figure">{formatBytes(result.freed)}</div>
  <div class="caption">freed{problems ? ", with some items left behind (see below)" : ""}</div>

  <ul>
    {#each results as r (r.id)}
      <li>
        <div class="line">
          <span class="name">{r.name}</span>
          <span class="num">
            {#if r.skipped}
              <span class="warn"><span aria-hidden="true">⏸</span> skipped</span>
            {:else if r.errors?.length}
              <span class="warn"><span aria-hidden="true">⚠</span> {formatBytes(r.freed)}, {r.errors.length} not removed</span>
            {:else}
              <span class="ok"><span aria-hidden="true">✓</span> {formatBytes(r.freed)}</span>
            {/if}
          </span>
        </div>
        {#if r.skipped}<p class="note">{r.skipped}</p>{/if}
        {#each r.notes ?? [] as n}<p class="note">{n}</p>{/each}
        {#each (r.errors ?? []).slice(0, 3) as e}<p class="note err selectable">{e}</p>{/each}
        {#if (r.errors?.length ?? 0) > 3}<p class="note err">… and {r.errors!.length - 3} more</p>{/if}
      </li>
    {:else}
      <li class="note">Nothing was removed.</li>
    {/each}
  </ul>

  <button class="primary" onclick={onrescan}>Scan again</button>
</section>

<style>
  .hero-figure {
    font-size: 52px;
    font-weight: 650;
    line-height: 1.05;
    letter-spacing: -0.02em;
  }
  .caption {
    color: var(--text-secondary);
    font-size: 15px;
    margin: 4px 0 20px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0 0 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 720px;
  }
  li {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px 16px;
  }
  .line {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .name {
    font-weight: 600;
  }
  .ok {
    color: var(--text-primary);
    font-weight: 600;
  }
  .ok span {
    color: var(--good);
  }
  .warn {
    color: var(--text-primary);
  }
  .note {
    margin: 6px 0 0;
    font-size: 12.5px;
    color: var(--text-muted);
  }
  .err {
    color: var(--critical);
  }
  .selectable {
    user-select: text;
    -webkit-user-select: text;
    word-break: break-word;
  }
</style>
