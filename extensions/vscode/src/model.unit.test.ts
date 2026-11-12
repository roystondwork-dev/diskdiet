import { describe, expect, it } from "vitest";
import {
  ageWords,
  cleanable,
  describe as describeSource,
  DiskFullScanner,
  formatBytes,
  freeChoices,
  idle,
  isProjects,
  matchDiskFull,
  missingSources,
  plannedBytes,
  projectDescription,
  projectLabel,
  safeByDefault,
  sourceDescription,
  sourceTooltip,
  status,
  summarizeClean,
  tildify,
} from "./model";
import type { CleanData, ScanData, SourceReport } from "./schema";

const GB = 1e9;

function src(over: Partial<SourceReport>): SourceReport {
  return { id: "npm", name: "npm cache", found: true, bytes: 40 * GB, items: 10, prunableBytes: 30 * GB, prunableItems: 9, ...over };
}

const scan: ScanData = {
  schemaVersion: 1,
  olderThan: "30d",
  bytes: 60 * GB,
  prunableBytes: 41 * GB,
  sources: [
    src({ locations: ["/h/.npm"] }),
    src({ id: "docker", name: "Docker", bytes: 9 * GB, prunableBytes: 5 * GB, estimate: true, notes: ["volumes kept"] }),
    src({
      id: "node_modules",
      name: "node_modules",
      bytes: 11 * GB,
      prunableBytes: 6 * GB,
      prunableItems: 2,
      projects: [
        { path: "/h/code/a/node_modules", bytes: 4 * GB, idleDays: 100 },
        { path: "/h/code/b/node_modules", bytes: 2 * GB, idleDays: 40 },
      ],
    }),
    src({ id: "pip", name: "pip cache", bytes: GB, prunableBytes: 0, prunableItems: 0 }),
    src({ id: "bun", name: "Bun cache", found: false, bytes: 0, prunableBytes: 0, prunableItems: 0 }),
  ],
};

describe("formatting", () => {
  it("formats bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(-1)).toBe("0 B");
    expect(formatBytes(Number.NaN)).toBe("0 B");
    expect(formatBytes(999)).toBe("999 B");
    expect(formatBytes(1500)).toBe("1.5 kB");
    expect(formatBytes(41.3 * GB)).toBe("41.3 GB");
    expect(formatBytes(999.96 * GB)).toBe("1.0 TB");
    expect(formatBytes(5e21)).toBe("5000000.0 PB");
  });
  it("formats idle times", () => {
    expect([0, 1, 12, 75, 400].map(idle)).toEqual(["today", "1 day", "12 days", "2 months", "1.1 years"]);
  });
  it("words the age", () => {
    expect(ageWords(0)).toBe("in total");
    expect(ageWords(7)).toBe("unused for 7 days");
  });
  it("shortens home paths", () => {
    expect(tildify("/h", "/h")).toBe("~");
    expect(tildify("/h/x", "/h")).toBe("~/x");
    expect(tildify("C:\\h\\x", "C:\\h")).toBe("~\\x");
    expect(tildify("/hx", "/h")).toBe("/hx");
    expect(tildify("/other", "/h")).toBe("/other");
    expect(tildify("/h/x", "")).toBe("/h/x");
  });
});

describe("sources", () => {
  it("knows project sources, safe defaults and descriptions", () => {
    expect(isProjects("target") && isProjects("node_modules") && !isProjects("npm")).toBe(true);
    expect(safeByDefault("npm") && !safeByDefault("docker") && !safeByDefault("target")).toBe(true);
    for (const id of ["npm", "yarn", "bun", "pnpm", "pip", "uv", "cargo", "go", "gradle", "maven", "playwright", "cypress", "xcode", "docker", "node_modules", "target"]) {
      expect(describeSource(id)).not.toBe("");
    }
    expect(describeSource("nope")).toBe("");
  });
  it("describes a row", () => {
    expect(sourceDescription(src({}))).toBe("40.0 GB · 30.0 GB can go");
    expect(sourceDescription(src({ estimate: true }))).toBe("40.0 GB · up to 30.0 GB can go");
    expect(sourceDescription(src({ prunableBytes: 0, prunableItems: 0 }))).toBe("40.0 GB · nothing old");
    expect(sourceDescription(src({ unavailable: true }))).toBe("not running");
    expect(cleanable(src({ prunableBytes: 0, prunableItems: 1 }))).toBe(true);
  });
  it("builds a tooltip", () => {
    const t = sourceTooltip(src({ locations: ["/h/.npm"], notes: ["n"], error: "e", estimate: true }), 30, "/h");
    expect(t).toContain("Up to 30.0 GB unused for 30 days can be freed.");
    expect(t).toContain("~/.npm");
    expect(t).toContain("• n");
    expect(t).toContain("⚠ e");
    expect(sourceTooltip(src({ prunableBytes: 0, prunableItems: 0 }), 30, "/h")).toBe("npm cache: 40.0 GB");
    expect(sourceTooltip(src({}), 0, "/h")).toContain("\n30.0 GB in total can be freed.");
  });
  it("labels projects", () => {
    const p = { path: "/h/code/a/node_modules", bytes: 4 * GB, idleDays: 100 };
    expect(projectLabel(p, "/h")).toBe("~/code/a");
    expect(projectLabel({ ...p, path: "C:\\h\\rs\\target" }, "C:\\h")).toBe("~\\rs");
    expect(projectDescription(p)).toBe("4.0 GB · untouched 3 months");
  });
  it("lists missing sources", () => {
    expect(missingSources(scan)).toEqual(["Bun cache"]);
  });
});

describe("status", () => {
  it("shows a spinner while the first scan runs", () => {
    expect(status(undefined, "always", 10, 30)).toMatchObject({ show: true, warn: false });
    expect(status(undefined, "aboveThreshold", 10, 30).show).toBe(false);
  });
  it("shows above the threshold, always, or never", () => {
    expect(status(scan, "aboveThreshold", 10, 30)).toMatchObject({ show: true, warn: true, text: "$(database) 41.0 GB" });
    expect(status(scan, "aboveThreshold", 50, 30)).toMatchObject({ show: false, warn: false });
    expect(status(scan, "never", 10, 30).show).toBe(false);
    expect(status(scan, "always", 0, 30)).toMatchObject({ show: true, warn: false });
    const none = status({ ...scan, prunableBytes: 0 }, "always", 10, 7);
    expect(none.text).toBe("$(database) diskdiet");
    expect(none.tooltip).toContain("nothing unused for 7 days");
  });
});

describe("free", () => {
  it("offers cleanable sources, caches ticked by default", () => {
    const c = freeChoices(scan);
    expect(c.map((x) => x.id)).toEqual(["npm", "node_modules", "docker"]);
    expect(c.map((x) => x.picked)).toEqual([true, false, false]);
    expect(c[2]?.description).toBe("up to 5.0 GB");
  });
  it("adds up what will be freed, minus kept folders", () => {
    expect(plannedBytes(scan, ["npm", "node_modules"], [])).toBe(36 * GB);
    expect(plannedBytes(scan, ["npm", "node_modules"], ["/h/code/a/node_modules"])).toBe(32 * GB);
    expect(plannedBytes({ ...scan, sources: [src({ id: "target", name: "t", prunableBytes: 7 })] }, ["target"], [])).toBe(7);
  });
  it("summarizes a clean", () => {
    const c: CleanData = {
      schemaVersion: 1,
      olderThan: "30d",
      dryRun: false,
      plan: [],
      freedBytes: 31 * GB,
      results: [
        { id: "npm", name: "npm cache", freedBytes: 30 * GB, removed: 9, notes: ["ok"] },
        { id: "pnpm", name: "pnpm store", freedBytes: 0, removed: 0, skipped: '"pnpm add" is running' },
        { id: "docker", name: "Docker", freedBytes: GB, removed: 1, errors: ["denied"] },
      ],
    };
    const s = summarizeClean(c);
    expect(s.problems).toBe(true);
    expect(s.message).toBe("diskdiet freed 31.0 GB. Some items were left; see the output.");
    expect(s.details).toEqual([
      "npm cache: freed 30.0 GB",
      "npm cache: ok",
      'pnpm store: skipped, "pnpm add" is running',
      "Docker: freed 1.0 GB; 1 could not be removed (denied)",
    ]);
    expect(summarizeClean({ ...c, results: [], freedBytes: 0 }).message).toBe("diskdiet: nothing was old enough to remove.");
    expect(summarizeClean({ ...c, results: [{ id: "x", name: "X", freedBytes: 1, removed: 1, errors: [] }] }).problems).toBe(false);
  });
});

describe("disk full", () => {
  it("matches the errors tools print", () => {
    for (const line of [
      "npm ERR! code ENOSPC",
      "write /tmp/x: no space left on device",
      "\u001b[31mError: No space left on device\u001b[0m",
      "There is not enough space on the disk.",
      "Not enough free space to extract",
      "Disk quota exceeded",
      "EDQUOT",
    ]) {
      expect(matchDiskFull(line), line).toBe(true);
    }
    expect(matchDiskFull("all good, 3 packages installed")).toBe(false);
  });
  it("finds an error split across chunks, once", () => {
    const s = new DiskFullScanner();
    expect(s.push("installing... no space ")).toBe(false);
    expect(s.push("left on device\n")).toBe(true);
    expect(s.push("ENOSPC again")).toBe(false);
  });
});
