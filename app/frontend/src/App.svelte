<script lang="ts">
  import { onMount } from "svelte";
  import { Events, System } from "@wailsio/runtime";
  import * as Disk from "../bindings/github.com/roystondwork-dev/diskdiet/app/diskservice";
  import type {
    CleanSummary,
    Progress,
    SourceView,
    Summary,
  } from "../bindings/github.com/roystondwork-dev/diskdiet/internal/appcore/models";
  import { formatBytes, isProjects, percent } from "./lib/format";
  import FreeBar from "./lib/FreeBar.svelte";
  import SourceRow from "./lib/SourceRow.svelte";
  import ConfirmDialog from "./lib/ConfirmDialog.svelte";
  import Results from "./lib/Results.svelte";

  type Phase = "scanning" | "ready" | "confirm" | "cleaning" | "done" | "error";

  const AGES = [
    { days: 7, label: "7 days" },
    { days: 30, label: "30 days" },
    { days: 90, label: "90 days" },
    { days: 0, label: "Everything" },
  ];

  let phase = $state<Phase>("scanning");
  let olderThanDays = $state(30);
  let volumes = $state(false);
  let summary = $state<Summary | null>(null);
  let result = $state<CleanSummary | null>(null);
  let error = $state("");
  let version = $state("");
  // Sources the user unticked, and project folders they chose to keep.
  let unselected: Set<string> = $state(new Set());
  let keepProjects: Set<string> = $state(new Set());
  // Live status of each source while scanning or cleaning.
  let activity: Record<string, { name: string; done: boolean }> = $state({});
  const isMac = System.IsMac();

  const sources: SourceView[] = $derived(summary?.sources ?? []);
  const cleanable = $derived(sources.filter((s) => s.prunableBytes > 0 || s.prunableItems > 0));
  const selected = $derived(cleanable.filter((s) => !unselected.has(s.id)));

  function freeFor(s: SourceView): number {
    if (!isProjects(s.id)) return s.prunableBytes;
    return (s.projects ?? []).filter((p) => !keepProjects.has(p.path)).reduce((n, p) => n + p.bytes, 0);
  }

  const selectedBytes = $derived(selected.reduce((n, s) => n + freeFor(s), 0));
  const anyEstimate = $derived(selected.some((s) => s.estimate));

  onMount(() => {
    const off = Events.On("progress", (ev: { data: Progress }) => {
      const p = ev.data;
      activity = { ...activity, [p.id]: { name: p.name, done: p.done } };
    });
    Disk.Version().then((v) => (version = v));
    scan();
    return off;
  });

  async function scan() {
    phase = "scanning";
    activity = {};
    error = "";
    try {
      summary = await Disk.Scan({ olderThanDays, volumes });
      keepProjects = new Set();
      phase = "ready";
    } catch (e) {
      error = String(e);
      phase = "error";
    }
  }

  function setAge(days: number) {
    if (days === olderThanDays || phase === "scanning" || phase === "cleaning") return;
    olderThanDays = days;
    scan();
  }

  function setVolumes(on: boolean) {
    volumes = on;
    scan();
  }

  function toggle(id: string, on: boolean) {
    const next = new Set(unselected);
    if (on) next.delete(id);
    else next.add(id);
    unselected = next;
  }

  function toggleProject(path: string, keep: boolean) {
    const next = new Set(keepProjects);
    if (keep) next.add(path);
    else next.delete(path);
    keepProjects = next;
  }

  async function clean() {
    phase = "cleaning";
    activity = {};
    try {
      result = await Disk.Clean({
        ids: selected.map((s) => s.id),
        keepProjects: [...keepProjects],
        force: false,
      });
      phase = "done";
    } catch (e) {
      error = String(e);
      phase = "error";
    }
  }

  function reveal(path: string) {
    Disk.Reveal(path).catch(() => {});
  }

  const ageWords = $derived(
    olderThanDays === 0 ? "in total" : `unused for ${olderThanDays} days`,
  );
</script>

<div class="window" class:mac={isMac}>
  <header style="--wails-draggable: drag">
    <div class="brand" title={version && version !== "dev" ? `diskdiet ${version}` : "diskdiet"}>
      <span class="logo" aria-hidden="true"></span>
      <span>diskdiet</span>
    </div>
    <div class="controls" style="--wails-draggable: no-drag">
      <span class="label" id="age-label">Count as old</span>
      <div class="segmented" role="radiogroup" aria-labelledby="age-label">
        {#each AGES as a}
          <button
            role="radio"
            aria-checked={olderThanDays === a.days}
            class:on={olderThanDays === a.days}
            disabled={phase === "scanning" || phase === "cleaning"}
            onclick={() => setAge(a.days)}>{a.label}</button
          >
        {/each}
      </div>
      <button
        class="ghost"
        onclick={scan}
        disabled={phase === "scanning" || phase === "cleaning"}
        title="Scan again">Rescan</button
      >
    </div>
  </header>

  <main>
    {#if phase === "scanning"}
      <section class="center" aria-live="polite">
        <div class="spinner" aria-hidden="true"></div>
        <p class="lead">Measuring your caches…</p>
        <ul class="activity">
          {#each Object.entries(activity) as [id, a] (id)}
            <li class:done={a.done}><span class="dot" aria-hidden="true"></span>{a.name}{a.done ? "" : "…"}</li>
          {/each}
        </ul>
      </section>
    {:else if phase === "error"}
      <section class="center">
        <p class="lead error"><span aria-hidden="true">⚠</span> Something went wrong</p>
        <p class="muted selectable">{error}</p>
        <button class="primary" onclick={scan}>Scan again</button>
      </section>
    {:else if phase === "done" && result}
      <Results {result} onrescan={scan} />
    {:else if summary}
      <section class="hero">
        <div>
          <div class="hero-figure">{formatBytes(summary.prunableBytes)}</div>
          <div class="hero-caption">
            can be freed, {ageWords}, of <strong>{formatBytes(summary.bytes)}</strong> your developer
            tools use
          </div>
        </div>
      </section>

      {#if summary.bytes > 0}
        <FreeBar free={summary.prunableBytes} total={summary.bytes} />
      {/if}

      {#if sources.length === 0}
        <p class="muted empty">No caches found. diskdiet looks for npm, Yarn, pnpm, Bun, pip, uv, Cargo, Go, Gradle, Maven, Playwright, Cypress, Xcode, Docker, node_modules and target folders.</p>
      {:else}
        <ul class="sources">
          {#each sources as s (s.id)}
            <SourceRow
              source={s}
              max={sources[0].bytes}
              checked={!unselected.has(s.id)}
              {keepProjects}
              {volumes}
              ontoggle={(on: boolean) => toggle(s.id, on)}
              onkeep={toggleProject}
              onvolumes={setVolumes}
              onreveal={reveal}
            />
          {/each}
        </ul>
      {/if}
      {#if summary.missing?.length}
        <p class="muted missing">Not found: {summary.missing.join(", ")}</p>
      {/if}
    {/if}
  </main>

  {#if summary && (phase === "ready" || phase === "confirm")}
    <footer>
      <div class="muted">
        {#if selected.length}
          {selected.length} of {cleanable.length} selected · {percent(selectedBytes, summary.bytes)} of the space your tools use
        {:else if cleanable.length}
          Nothing selected
        {:else}
          Nothing {ageWords} to free. Try a shorter age.
        {/if}
      </div>
      <button class="primary" disabled={selectedBytes === 0} onclick={() => (phase = "confirm")}>
        Free {anyEstimate ? "up to " : ""}{formatBytes(selectedBytes)}
      </button>
    </footer>
  {/if}

  {#if phase === "confirm"}
    <ConfirmDialog
      sources={selected}
      {freeFor}
      {volumes}
      {olderThanDays}
      total={selectedBytes}
      estimate={anyEstimate}
      oncancel={() => (phase = "ready")}
      onconfirm={clean}
    />
  {/if}

  {#if phase === "cleaning"}
    <div class="overlay" role="dialog" aria-modal="true" aria-label="Cleaning">
      <div class="panel center">
        <div class="spinner" aria-hidden="true"></div>
        <p class="lead">Freeing space…</p>
        <ul class="activity">
          {#each Object.entries(activity) as [id, a] (id)}
            <li class:done={a.done}><span class="dot" aria-hidden="true"></span>{a.name}{a.done ? " ✓" : "…"}</li>
          {/each}
        </ul>
      </div>
    </div>
  {/if}

</div>

<style>
  .window {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--surface-0);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 24px;
    min-height: 56px;
    border-bottom: 1px solid var(--border);
    background: var(--surface-1);
  }
  .mac header {
    padding-left: 88px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 600;
    font-size: 15px;
  }
  .logo {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: conic-gradient(var(--fill) 0 30%, var(--track) 30% 100%);
    mask: radial-gradient(circle, transparent 5px, #000 5.5px);
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .label {
    color: var(--text-secondary);
    font-size: 13px;
  }
  .segmented {
    display: inline-flex;
    padding: 2px;
    border-radius: 8px;
    background: var(--surface-0);
    border: 1px solid var(--border);
  }
  .segmented button {
    border: 0;
    background: transparent;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 13px;
    color: var(--text-secondary);
  }
  .segmented button.on {
    background: var(--surface-2);
    color: var(--text-primary);
    box-shadow: var(--shadow);
    font-weight: 600;
  }
  .segmented button:disabled {
    cursor: default;
  }
  main {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }
  .hero {
    margin-bottom: 16px;
  }
  .hero-figure {
    font-size: 52px;
    font-weight: 650;
    line-height: 1.05;
    letter-spacing: -0.02em;
  }
  .hero-caption {
    color: var(--text-secondary);
    font-size: 15px;
    margin-top: 4px;
  }
  .hero-caption strong {
    color: var(--text-primary);
    font-weight: 600;
  }
  .sources {
    list-style: none;
    padding: 0;
    margin: 20px 0 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .missing,
  .empty {
    margin-top: 16px;
  }
  .muted {
    color: var(--text-muted);
  }
  .selectable {
    user-select: text;
    -webkit-user-select: text;
    max-width: 640px;
    text-align: center;
  }
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 24px;
    border-top: 1px solid var(--border);
    background: var(--surface-1);
  }
  :global(button.primary) {
    border: 0;
    border-radius: 8px;
    padding: 9px 18px;
    background: var(--fill);
    color: var(--on-fill);
    font-weight: 600;
  }
  :global(button.primary:hover:not(:disabled)) {
    background: var(--fill-hover);
  }
  :global(button.primary:disabled) {
    opacity: 0.45;
    cursor: default;
  }
  :global(button.ghost) {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 5px 12px;
    background: var(--surface-2);
    font-size: 13px;
  }
  :global(button.ghost:disabled) {
    opacity: 0.5;
    cursor: default;
  }
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 60%;
  }
  .lead {
    font-size: 17px;
    font-weight: 600;
    margin: 8px 0 0;
  }
  .error {
    color: var(--critical);
  }
  .spinner {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    border: 3px solid var(--track);
    border-top-color: var(--fill);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .activity {
    list-style: none;
    padding: 0;
    margin: 8px 0 0;
    color: var(--text-secondary);
    font-size: 13px;
    min-width: 200px;
  }
  .activity li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 2px 0;
  }
  .activity .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--track);
  }
  .activity li.done .dot {
    background: var(--fill);
  }
  .overlay {
    position: fixed;
    inset: 0;
    background: rgb(0 0 0 / 35%);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .panel {
    background: var(--surface-1);
    border-radius: 12px;
    padding: 28px 40px;
    box-shadow: var(--shadow);
    min-height: auto;
  }
</style>
