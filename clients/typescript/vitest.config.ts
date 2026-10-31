import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globalSetup: ["./test/server.ts"],
    include: ["test/**/*.test.ts"],
    testTimeout: 20_000,
    hookTimeout: 120_000,
    // Tests share one in-process server; keep them in one worker so the
    // operation-coverage check at shutdown sees everything.
    fileParallelism: false,
  },
});
