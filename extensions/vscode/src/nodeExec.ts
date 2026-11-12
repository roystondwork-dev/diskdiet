import { type ExecFileException, execFile } from "node:child_process";
import { DiskdietError, type ExecFn, type ExecResult } from "./cli";

/** ExecFn backed by child_process.execFile. */
export const nodeExec: ExecFn = (file, args, timeoutMs) =>
  new Promise((resolve, reject) => {
    execFile(file, [...args], { timeout: timeoutMs, maxBuffer: 64 * 1024 * 1024, windowsHide: true }, (err, stdout, stderr) => {
      const r = settle(err, stdout, stderr, file, timeoutMs);
      if (r instanceof DiskdietError) {
        reject(r);
      } else {
        resolve(r);
      }
    });
  });

/**
 * Maps an execFile outcome to a result (any exit code) or an error (the
 * program couldn't run). Separate from nodeExec so every case is testable.
 */
export function settle(err: ExecFileException | null, stdout: string, stderr: string, file: string, timeoutMs: number): ExecResult | DiskdietError {
  if (err === null) {
    return { stdout, stderr, code: 0 };
  }
  if (typeof err.code === "number") {
    return { stdout, stderr, code: err.code };
  }
  if (err.code === "ENOENT") {
    return new DiskdietError(`diskdiet was not found ("${file}"). Install it or set "diskdiet.path".`);
  }
  return new DiskdietError(err.killed ? `diskdiet timed out after ${Math.round(timeoutMs / 1000)} s` : err.message);
}
