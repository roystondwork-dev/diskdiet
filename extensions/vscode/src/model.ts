// Everything the extension shows, computed from diskdiet's JSON with no
// "vscode" import, so it is unit-tested in full.

import type { CleanData, Project, ScanData, SourceReport } from "./schema";

const UNITS = ["B", "kB", "MB", "GB", "TB", "PB"];

/** Decimal units, like Finder, Explorer, Docker and the diskdiet CLI. */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) {
    return "0 B";
  }
  if (n < 1000) {
    return `${Math.round(n)} B`;
  }
  let v = n;
  let i = 0;
  while (v >= 999.95 && i < UNITS.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v.toFixed(1)} ${UNITS[i] as string}`;
}

export function idle(days: number): string {
  if (days >= 365) {
    return `${(days / 365).toFixed(1)} years`;
  }
  if (days >= 60) {
    return `${Math.floor(days / 30)} months`;
  }
  if (days === 1) {
    return "1 day";
  }
  if (days <= 0) {
    return "today";
  }
  return `${days} days`;
}

export function isProjects(id: string): boolean {
  return id === "node_modules" || id === "target";
}

export function cleanable(s: SourceReport): boolean {
  return s.prunableBytes > 0 || s.prunableItems > 0;
}

/** What a clean removes from a source, in plain words. */
export function describe(id: string): string {
  switch (id) {
    case "npm":
      return "Cached packages and metadata, old npx installs and logs";
    case "yarn":
    case "bun":
      return "Cached packages";
    case "pnpm":
      return "Packages no project uses (pnpm store prune) and old metadata";
    case "pip":
      return "Cached downloads and built wheels";
    case "uv":
      return "The whole uv cache (uv cache clean)";
    case "cargo":
      return "Downloaded crates, unpacked sources and git checkouts";
    case "go":
      return "Build cache entries and module versions";
    case "gradle":
      return "Downloaded Gradle versions and per-version caches";
    case "maven":
      return "Artifact versions in ~/.m2";
    case "playwright":
      return "Older browser versions; the newest of each stays";
    case "cypress":
      return "Older Cypress versions; the newest stays";
    case "xcode":
      return "DerivedData and device support files (never Archives)";
    case "docker":
      return "Unused images, stopped containers and build cache (volumes stay)";
    case "node_modules":
      return "node_modules of projects you haven't touched";
    case "target":
      return "target folders of Rust and Maven projects you haven't touched";
    default:
      return "";
  }
}

/** Sources removing whose old entries costs at most a re-download: selected by default. */
export function safeByDefault(id: string): boolean {
  return !isProjects(id) && id !== "docker";
}

export function ageWords(olderThanDays: number): string {
  return olderThanDays <= 0 ? "in total" : `unused for ${olderThanDays} days`;
}

/** Shortens paths in the home folder to ~/…. */
export function tildify(p: string, home: string): string {
  if (home === "" || !p.startsWith(home)) {
    return p;
  }
  const rest = p.slice(home.length);
  if (rest === "") {
    return "~";
  }
  return rest.startsWith("/") || rest.startsWith("\\") ? `~${rest}` : p;
}

/** The size column of a source: "41.3 GB · 33.8 GB can go". */
export function sourceDescription(s: SourceReport): string {
  if (s.unavailable) {
    return "not running";
  }
  const size = formatBytes(s.bytes);
  if (!cleanable(s)) {
    return `${size} · nothing old`;
  }
  return `${size} · ${s.estimate ? "up to " : ""}${formatBytes(s.prunableBytes)} can go`;
}

export function sourceTooltip(s: SourceReport, olderThanDays: number, home: string): string {
  const lines = [`${s.name}: ${formatBytes(s.bytes)}`];
  if (cleanable(s)) {
    lines.push(`${s.estimate ? "Up to " : ""}${formatBytes(s.prunableBytes)} ${ageWords(olderThanDays)} can be freed.`, describe(s.id));
  }
  for (const l of s.locations ?? []) {
    lines.push(tildify(l, home));
  }
  for (const n of s.notes ?? []) {
    lines.push(`• ${n}`);
  }
  if (s.error) {
    lines.push(`⚠ ${s.error}`);
  }
  return lines.join("\n");
}

export function projectLabel(p: Project, home: string): string {
  const dir = p.path.replace(/[\\/](node_modules|target)$/, "");
  return tildify(dir, home);
}

export function projectDescription(p: Project): string {
  return `${formatBytes(p.bytes)} · untouched ${idle(p.idleDays)}`;
}

/** Found sources, biggest first. */
export function foundSources(scan: ScanData): SourceReport[] {
  return scan.sources.filter((s) => s.found).sort((a, b) => b.bytes - a.bytes);
}

export function missingSources(scan: ScanData): string[] {
  return scan.sources.filter((s) => !s.found).map((s) => s.name);
}

export type StatusMode = "aboveThreshold" | "always" | "never";

export interface Status {
  show: boolean;
  text: string;
  tooltip: string;
  /** Above the warning threshold. */
  warn: boolean;
}

/** The status bar item for a scan. */
export function status(scan: ScanData | undefined, mode: StatusMode, warnAboveGB: number, olderThanDays: number): Status {
  if (!scan) {
    return { show: mode === "always", text: "$(sync~spin) diskdiet", tooltip: "diskdiet is checking your caches…", warn: false };
  }
  const free = scan.prunableBytes;
  const warn = warnAboveGB > 0 && free >= warnAboveGB * 1e9;
  const show = mode === "always" || (mode === "aboveThreshold" && warn);
  const text = free > 0 ? `$(database) ${formatBytes(free)}` : "$(database) diskdiet";
  const tooltip =
    free > 0
      ? `diskdiet: ${formatBytes(free)} ${ageWords(olderThanDays)} can be freed, of ${formatBytes(scan.bytes)} your developer tools use. Click to see what.`
      : `diskdiet: nothing ${ageWords(olderThanDays)} to free (${formatBytes(scan.bytes)} used).`;
  return { show, text, tooltip, warn };
}

export interface FreeChoice {
  id: string;
  label: string;
  description: string;
  detail: string;
  picked: boolean;
}

/** The sources offered by "Free Disk Space…", biggest first. */
export function freeChoices(scan: ScanData): FreeChoice[] {
  return foundSources(scan)
    .filter(cleanable)
    .map((s) => ({
      id: s.id,
      label: s.name,
      description: `${s.estimate ? "up to " : ""}${formatBytes(s.prunableBytes)}`,
      detail: describe(s.id),
      picked: safeByDefault(s.id),
    }));
}

/** What the chosen sources would free, minus kept project folders. */
export function plannedBytes(scan: ScanData, ids: readonly string[], keep: readonly string[]): number {
  const kept = new Set(keep);
  let total = 0;
  for (const s of scan.sources) {
    if (!ids.includes(s.id)) {
      continue;
    }
    if (isProjects(s.id) && s.projects) {
      total += s.projects.filter((p) => !kept.has(p.path)).reduce((n, p) => n + p.bytes, 0);
    } else {
      total += s.prunableBytes;
    }
  }
  return total;
}

export interface CleanSummary {
  message: string;
  /** Some sources were skipped or had errors. */
  problems: boolean;
  details: string[];
}

export function summarizeClean(c: CleanData): CleanSummary {
  const details: string[] = [];
  let problems = false;
  for (const r of c.results) {
    if (r.skipped) {
      problems = true;
      details.push(`${r.name}: skipped, ${r.skipped}`);
    } else if (r.errors && r.errors.length > 0) {
      problems = true;
      details.push(`${r.name}: freed ${formatBytes(r.freedBytes)}; ${r.errors.length} could not be removed (${String(r.errors[0])})`);
    } else if (r.trashedBytes) {
      details.push(`${r.name}: moved ${formatBytes(r.trashedBytes)} to the Trash`);
    } else {
      details.push(`${r.name}: freed ${formatBytes(r.freedBytes)}`);
    }
    for (const n of r.notes ?? []) {
      details.push(`${r.name}: ${n}`);
    }
  }
  const trashed = c.trashedBytes ?? 0;
  const trashNote = trashed > 0 ? ` Moved ${formatBytes(trashed)} to the Trash: empty it to free that space too.` : "";
  const message =
    c.results.length === 0
      ? "diskdiet: nothing was old enough to remove."
      : `diskdiet freed ${formatBytes(c.freedBytes)}.${trashNote}${problems ? " Some items were left; see the output." : ""}`;
  return { message, problems, details };
}

/** Disk-full errors as tools print them. */
const DISK_FULL = [
  /\bENOSPC\b/,
  /no space left on device/i,
  /there is not enough space on the disk/i,
  /not enough free space/i,
  /disk quota exceeded/i,
  /\bEDQUOT\b/,
];

// eslint-disable-next-line no-control-regex
const ANSI = /\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*(\x07|\x1b\\)/g;

export function matchDiskFull(text: string): boolean {
  const plain = text.replace(ANSI, "");
  return DISK_FULL.some((re) => re.test(plain));
}

/**
 * Watches a command's output for a disk-full error, across chunk boundaries,
 * and reports it once.
 */
export class DiskFullScanner {
  private tail = "";
  private seen = false;

  /** Feeds output; true the first time a disk-full error appears. */
  push(chunk: string): boolean {
    if (this.seen) {
      return false;
    }
    const text = this.tail + chunk;
    // Keep the end, in case an error is split across chunks.
    this.tail = text.slice(-200);
    if (matchDiskFull(text)) {
      this.seen = true;
      return true;
    }
    return false;
  }
}
