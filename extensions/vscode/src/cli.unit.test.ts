import * as path from "node:path";
import { describe, expect, it } from "vitest";
import { ageArgs, DiskdietClient, DiskdietError, errorLine, type ExecFn, resolveBinary } from "./cli";

const scanDoc = { schemaVersion: 1, olderThan: "30d", sources: [], bytes: 0, prunableBytes: 0 };

function fake(result: { stdout?: string; stderr?: string; code?: number }, calls: string[][] = []): ExecFn {
  return (file, args) => {
    calls.push([file, ...args]);
    return Promise.resolve({ stdout: result.stdout ?? "", stderr: result.stderr ?? "", code: result.code ?? 0 });
  };
}

describe("resolveBinary", () => {
  const base = { extensionPath: "/ext", platform: "darwin" as NodeJS.Platform, arch: "arm64" };
  it("prefers the setting", () => {
    expect(resolveBinary({ ...base, configured: " /opt/diskdiet ", exists: () => true })).toBe("/opt/diskdiet");
  });
  it("uses the bundled binary when present", () => {
    expect(resolveBinary({ ...base, configured: "", exists: () => true })).toBe(path.join("/ext", "bin", "darwin-arm64", "diskdiet"));
  });
  it("falls back to PATH, with .exe on Windows", () => {
    expect(resolveBinary({ ...base, configured: "", exists: () => false })).toBe("diskdiet");
    expect(resolveBinary({ ...base, platform: "win32", configured: "", exists: () => false })).toBe("diskdiet.exe");
  });
});

describe("ageArgs", () => {
  it("maps days to flags", () => {
    expect(ageArgs(30)).toEqual(["--older-than", "30d"]);
    expect(ageArgs(7.9)).toEqual(["--older-than", "7d"]);
    expect(ageArgs(0)).toEqual(["--all"]);
    expect(ageArgs(-1)).toEqual(["--all"]);
    expect(ageArgs(Number.NaN)).toEqual(["--all"]);
  });
});

describe("errorLine", () => {
  it("skips the demo banner and the prefix", () => {
    expect(errorLine("diskdiet: demo mode (DISKDIET_DEMO): made-up data\ndiskdiet: unknown source \"x\"\n", 2)).toBe('unknown source "x"');
    expect(errorLine("", 7)).toBe("diskdiet exited with status 7");
  });
});

describe("DiskdietClient", () => {
  it("scans with --json and the age", async () => {
    const calls: string[][] = [];
    const c = new DiskdietClient("dd", fake({ stdout: JSON.stringify(scanDoc) }, calls));
    await expect(c.scan(30)).resolves.toEqual(scanDoc);
    expect(calls[0]).toEqual(["dd", "scan", "--json", "--older-than", "30d"]);
  });

  it("cleans the chosen sources, keeping folders, and accepts exit code 3", async () => {
    const calls: string[][] = [];
    const doc = { schemaVersion: 1, olderThan: "0", dryRun: false, plan: [], results: [], freedBytes: 5 };
    const c = new DiskdietClient("dd", fake({ stdout: JSON.stringify(doc), code: 3 }, calls));
    await expect(c.clean(["npm", "node_modules"], 0, ["/p/node_modules"])).resolves.toEqual(doc);
    expect(calls[0]).toEqual(["dd", "clean", "--json", "--yes", "--only", "npm,node_modules", "--all", "--keep", "/p/node_modules"]);
  });

  it("refuses an empty clean", async () => {
    const c = new DiskdietClient("dd", fake({}));
    await expect(c.clean([], 30)).rejects.toThrow("nothing chosen");
  });

  it("reports failures, non-JSON output and other schema versions", async () => {
    await expect(new DiskdietClient("dd", fake({ code: 2, stderr: "diskdiet: bad flag" })).scan(30)).rejects.toMatchObject({
      message: "bad flag",
      exitCode: 2,
    });
    await expect(new DiskdietClient("dd", fake({ stdout: "hello" })).scan(30)).rejects.toThrow("isn't JSON");
    await expect(new DiskdietClient("dd", fake({ stdout: JSON.stringify({ schemaVersion: 2 }) })).scan(30)).rejects.toThrow("schema version 2");
    await expect(new DiskdietClient("dd", fake({ stdout: "null" })).scan(30)).rejects.toBeInstanceOf(DiskdietError);
  });
});
