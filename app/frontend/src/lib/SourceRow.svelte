<script lang="ts">
  import type { SourceView } from "../../bindings/github.com/roystondwork-dev/diskdiet/internal/appcore/models";
  import { formatBytes, idle, describe } from "./format";

  let {
    source,
    max,
    checked,
    keepProjects,
    volumes,
    ontoggle,
    onkeep,
    onvolumes,
    onreveal,
  }: {
    source: SourceView;
    max: number;
    checked: boolean;
    keepProjects: Set<string>;
    volumes: boolean;
    ontoggle: (on: boolean) => void;
    onkeep: (path: string, keep: boolean) => void;
    onvolumes: (on: boolean) => void;
    onreveal: (path: string) => void;
  } = $props();

  const cleanable = $derived(source.prunableBytes > 0 || source.prunableItems > 0);
  const projects = $derived(source.projects ?? []);
  const free = $derived(
    source.id === "node_modules"
      ? projects.filter((p) => !keepProjects.has(p.path)).reduce((n, p) => n + p.bytes, 0)
      : source.prunableBytes,
  );
  // Bar: length is the source's size relative to the biggest source; the fill
  // inside is what can be freed.
  const width = $derived(max > 0 ? Math.max((source.bytes / max) * 100, source.bytes > 0 ? 1 : 0) : 0);
  const fillShare = $derived(source.bytes > 0 ? Math.min(free / source.bytes, 1) * 100 : 0);
  const shown = 8;
  let showAll = $state(false);
  const visible = $derived(showAll ? projects : projects.slice(0, shown));
</script>

<li class="row" class:off={!checked || !cleanable}>
  <div class="head">
    <label class="check">
      <input
        type="checkbox"
        checked={checked && cleanable}
        disabled={!cleanable}
        onchange={(e) => ontoggle((e.currentTarget as HTMLInputElement).checked)}
        aria-label={`Clean ${source.name}`}
      />
      <span class="name">{source.name}</span>
    </label>
    <div class="figures num">
      {#if source.unavailable}
        <span class="muted">not running</span>
      {:else}
        <span class="size">{formatBytes(source.bytes)}</span>
        <span class="free">
          {#if cleanable}
            {source.estimate ? "up to " : ""}{formatBytes(free)} can go
          {:else}
            nothing old
          {/if}
        </span>
      {/if}
    </div>
  </div>

  {#if !source.unavailable}
    <div
      class="track"
      style:width={`${width}%`}
      role="img"
      aria-label={`${source.name}: ${formatBytes(source.bytes)}, ${formatBytes(free)} can be freed`}
    >
      {#if fillShare > 0}<div class="fill" style:width={`${fillShare}%`}></div>{/if}
    </div>
  {/if}

  <div class="meta">
    {#if cleanable}<span class="desc">{describe(source.id, volumes)}</span>{/if}
    {#each source.locations ?? [] as loc}
      {#if loc.path}
        <button class="link" onclick={() => onreveal(loc.path)} title="Show in file manager"
          >{source.id === "node_modules" ? `searched ${loc.display === "~" ? "your home folder" : loc.display}` : loc.display}</button
        >
      {:else}
        <span class="muted">{loc.display}</span>
      {/if}
    {/each}
    {#if source.id === "node_modules"}
      <span class="muted">{source.items} folder{source.items === 1 ? "" : "s"} found</span>
    {/if}
  </div>

  {#if source.id === "docker" && !source.unavailable}
    <label class="option">
      <input type="checkbox" checked={volumes} onchange={(e) => onvolumes((e.currentTarget as HTMLInputElement).checked)} />
      Also remove unused volumes <span class="muted">(they can hold databases)</span>
    </label>
  {/if}

  {#each source.notes ?? [] as note}
    <p class="note">{note}</p>
  {/each}
  {#if source.error}
    <p class="note error"><span aria-hidden="true">⚠</span> {source.error}</p>
  {/if}

  {#if projects.length > 0}
    <details>
      <summary>{projects.length} project{projects.length === 1 ? "" : "s"} you haven't touched — untick any to keep</summary>
      <ul class="projects">
        {#each visible as p (p.path)}
          <li>
            <label>
              <input
                type="checkbox"
                checked={!keepProjects.has(p.path)}
                disabled={!checked}
                onchange={(e) => onkeep(p.path, !(e.currentTarget as HTMLInputElement).checked)}
              />
              <span class="path" title={p.path}>{p.display}</span>
            </label>
            <span class="num size">{formatBytes(p.bytes)}</span>
            <span class="muted when">{p.idleDays === 0 ? "changed today" : `untouched ${idle(p.idleDays)}`}</span>
            <button class="link" onclick={() => onreveal(p.path)} title="Show in file manager">Show</button>
          </li>
        {/each}
      </ul>
      {#if projects.length > shown}
        <button class="link more" onclick={() => (showAll = !showAll)}>
          {showAll ? "Show fewer" : `Show all ${projects.length}`}
        </button>
      {/if}
    </details>
  {/if}
</li>

<style>
  .row {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 16px;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 12px;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
  }
  .check input {
    width: 16px;
    height: 16px;
    accent-color: var(--fill);
  }
  .name {
    font-weight: 600;
    font-size: 15px;
  }
  .figures {
    display: flex;
    gap: 14px;
    align-items: baseline;
  }
  .size {
    font-weight: 600;
  }
  .free {
    color: var(--text-secondary);
    min-width: 120px;
    text-align: right;
  }
  .track {
    height: 8px;
    border-radius: 4px;
    background: var(--track);
    margin: 10px 0 8px 26px;
    max-width: calc(100% - 26px);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 4px;
    background: var(--fill);
  }
  .off .fill {
    opacity: 0.35;
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    padding-left: 26px;
    font-size: 12.5px;
    color: var(--text-secondary);
  }
  .muted {
    color: var(--text-muted);
  }
  .link {
    border: 0;
    background: none;
    padding: 0;
    color: var(--fill);
    font-size: 12.5px;
  }
  .link:hover {
    text-decoration: underline;
  }
  .option {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 10px 0 0 26px;
    font-size: 13px;
  }
  .option input {
    accent-color: var(--fill);
  }
  .note {
    margin: 6px 0 0 26px;
    font-size: 12.5px;
    color: var(--text-muted);
  }
  .note.error {
    color: var(--critical);
  }
  details {
    margin: 10px 0 0 26px;
    font-size: 13px;
  }
  summary {
    cursor: pointer;
    color: var(--text-secondary);
  }
  .projects {
    list-style: none;
    padding: 0;
    margin: 8px 0 0;
  }
  .projects li {
    display: grid;
    grid-template-columns: 1fr auto 140px auto;
    gap: 12px;
    align-items: center;
    padding: 4px 0;
    border-top: 1px solid var(--border);
  }
  .projects label {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .projects input {
    accent-color: var(--fill);
  }
  .path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
    text-align: left;
  }
  .when {
    font-size: 12px;
  }
  .more {
    margin-top: 6px;
  }
</style>
