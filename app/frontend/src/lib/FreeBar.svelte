<script lang="ts">
  import { formatBytes, percent } from "./format";

  // A tall segmented bar: one whole (what your tools use), split into what can
  // be freed and what stays. Labels sit inside a block only when it is wide
  // enough; the legend below always names both parts.
  let { free, total }: { free: number; total: number } = $props();
  const stays = $derived(Math.max(total - free, 0));
  const freeShare = $derived(total ? free / total : 0);
</script>

<div
  class="bar"
  role="img"
  aria-label={`${formatBytes(free)} can be freed (${percent(free, total)}); ${formatBytes(stays)} stays (${percent(stays, total)})`}
>
  {#if free > 0}
    <div class="part free" style:flex-grow={free} title={`Can be freed: ${formatBytes(free)} (${percent(free, total)})`}>
      {#if freeShare >= 0.12}<span>{percent(free, total)}</span>{/if}
    </div>
  {/if}
  {#if stays > 0}
    <div class="part stays" style:flex-grow={stays} title={`Stays: ${formatBytes(stays)} (${percent(stays, total)})`}>
      {#if 1 - freeShare >= 0.12}<span>{percent(stays, total)}</span>{/if}
    </div>
  {/if}
</div>
<div class="legend">
  <span><i class="swatch free" aria-hidden="true"></i>Can be freed</span>
  <span><i class="swatch stays" aria-hidden="true"></i>Stays</span>
</div>

<style>
  .bar {
    display: flex;
    gap: 2px;
    height: 28px;
  }
  .part {
    min-width: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    padding: 0 8px;
    font-size: 12px;
    font-weight: 600;
    overflow: hidden;
    white-space: nowrap;
  }
  .free {
    background: var(--fill);
    color: var(--on-fill);
  }
  .stays {
    background: var(--track);
    color: var(--on-track);
  }
  .legend {
    display: flex;
    gap: 16px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-secondary);
  }
  .legend span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .swatch {
    width: 10px;
    height: 10px;
    border-radius: 3px;
    display: inline-block;
  }
</style>
