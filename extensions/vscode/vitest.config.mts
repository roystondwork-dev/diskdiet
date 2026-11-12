import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["src/**/*.unit.test.ts"],
    coverage: {
      provider: "v8",
      // extension.ts imports "vscode"; everything it shows is computed in these.
      include: ["src/cli.ts", "src/model.ts", "src/nodeExec.ts"],
      thresholds: { lines: 100, functions: 100, branches: 100, statements: 100 },
    },
  },
});
