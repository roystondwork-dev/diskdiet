import type { ExecFileException } from "node:child_process";
import { describe, expect, it } from "vitest";
import { DiskdietError } from "./cli";
import { nodeExec, settle } from "./nodeExec";

function err(fields: Partial<ExecFileException>): ExecFileException {
  return Object.assign(new Error(fields.message ?? "boom"), fields);
}

describe("settle", () => {
  it("passes results through", () => {
    expect(settle(null, "out", "err", "dd", 1000)).toEqual({ stdout: "out", stderr: "err", code: 0 });
    expect(settle(err({ code: 3 }), "o", "e", "dd", 1000)).toEqual({ stdout: "o", stderr: "e", code: 3 });
  });
  it("explains a missing binary, timeouts and other failures", () => {
    expect((settle(err({ code: "ENOENT" }), "", "", "dd", 1000) as DiskdietError).message).toContain('"diskdiet.path"');
    expect((settle(err({ killed: true }), "", "", "dd", 5000) as DiskdietError).message).toBe("diskdiet timed out after 5 s");
    expect(settle(err({ message: "weird" }), "", "", "dd", 1000)).toBeInstanceOf(DiskdietError);
  });
});

describe("nodeExec", () => {
  it("runs a program and resolves with its exit code", async () => {
    const r = await nodeExec(process.execPath, ["-e", "process.stdout.write('hi'); process.exit(3)"], 10_000);
    expect(r).toEqual({ stdout: "hi", stderr: "", code: 3 });
  });
  it("rejects when the program doesn't exist", async () => {
    await expect(nodeExec("/no/such/diskdiet", [], 10_000)).rejects.toBeInstanceOf(DiskdietError);
  });
});
