// A thin client for the diskdiet CLI's JSON output. No cache logic lives here:
// every call runs `diskdiet … --json`, so the extension follows exactly the
// CLI's safety rules.

import * as path from "node:path";
import { type CleanData, type ScanData, SCHEMA_VERSION } from "./schema";

export interface ExecResult {
  stdout: string;
  stderr: string;
  code: number;
}

/** Runs a program; resolves with its exit code instead of rejecting on non-zero exit. */
export type ExecFn = (file: string, args: readonly string[], timeoutMs: number) => Promise<ExecResult>;

export class DiskdietError extends Error {
  constructor(
    message: string,
    readonly exitCode?: number,
  ) {
    super(message);
    this.name = "DiskdietError";
  }
}

export interface ResolveOptions {
  /** The diskdiet.path setting; empty means unset. */
  configured: string;
  /** Directory of the installed extension. */
  extensionPath: string;
  platform: NodeJS.Platform;
  arch: string;
  exists: (file: string) => boolean;
}

/**
 * Picks the binary: the setting, else the one bundled for this OS and CPU in
 * bin/<platform>-<arch>/ (release packages carry all six), else diskdiet on PATH.
 */
export function resolveBinary(o: ResolveOptions): string {
  if (o.configured.trim() !== "") {
    return o.configured.trim();
  }
  const exe = o.platform === "win32" ? "diskdiet.exe" : "diskdiet";
  const bundled = path.join(o.extensionPath, "bin", `${o.platform}-${o.arch}`, exe);
  return o.exists(bundled) ? bundled : exe;
}

/** Scans walk the home folder, which can take a while on big disks. */
const SCAN_TIMEOUT_MS = 5 * 60_000;
/** Cleans run package managers' own prune commands too. */
const CLEAN_TIMEOUT_MS = 60 * 60_000;

/** The age flag for a number of days; 0 means everything. */
export function ageArgs(olderThanDays: number): string[] {
  if (!Number.isFinite(olderThanDays) || olderThanDays <= 0) {
    return ["--all"];
  }
  return ["--older-than", `${Math.floor(olderThanDays)}d`];
}

/** The first meaningful line of diskdiet's error output. */
export function errorLine(stderr: string, code: number): string {
  for (const raw of stderr.split("\n")) {
    const line = raw.trim();
    if (line !== "" && !line.startsWith("diskdiet: demo mode")) {
      return line.replace(/^diskdiet: /, "");
    }
  }
  return `diskdiet exited with status ${code}`;
}

export class DiskdietClient {
  constructor(
    private readonly binary: string,
    private readonly exec: ExecFn,
  ) {}

  private async json<T extends { schemaVersion: number }>(args: string[], timeoutMs: number, okCodes: number[]): Promise<T> {
    const r = await this.exec(this.binary, args, timeoutMs);
    if (!okCodes.includes(r.code)) {
      throw new DiskdietError(errorLine(r.stderr, r.code), r.code);
    }
    let doc: unknown;
    try {
      doc = JSON.parse(r.stdout);
    } catch {
      throw new DiskdietError("diskdiet printed something that isn't JSON; is diskdiet.path pointing at diskdiet?", r.code);
    }
    const v = (doc as { schemaVersion?: unknown } | null)?.schemaVersion;
    if (v !== SCHEMA_VERSION) {
      throw new DiskdietError(`diskdiet's output has schema version ${String(v)}, this extension reads ${SCHEMA_VERSION}; update one of them`, r.code);
    }
    return doc as T;
  }

  /** Measures every source; changes nothing. */
  scan(olderThanDays: number): Promise<ScanData> {
    return this.json<ScanData>(["scan", "--json", ...ageArgs(olderThanDays)], SCAN_TIMEOUT_MS, [0]);
  }

  /**
   * Removes what is old in the given sources. diskdiet scans again and removes
   * exactly what that scan plans, minus the folders in keep. Exit code 3 means
   * some items could not be removed; the result says which.
   */
  clean(ids: readonly string[], olderThanDays: number, keep: readonly string[] = [], trash = false): Promise<CleanData> {
    if (ids.length === 0) {
      return Promise.reject(new DiskdietError("nothing chosen to clean"));
    }
    const args = ["clean", "--json", "--yes", "--only", ids.join(","), ...ageArgs(olderThanDays)];
    for (const k of keep) {
      args.push("--keep", k);
    }
    if (trash) {
      args.push("--trash");
    }
    return this.json<CleanData>(args, CLEAN_TIMEOUT_MS, [0, 3]);
  }
}
