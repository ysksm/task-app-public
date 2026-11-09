import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["tests/**/*.test.ts"],
    testTimeout: 15_000,
    hookTimeout: 20_000,
    // Each file starts its own server on a free port, so files can run in parallel.
    fileParallelism: true,
  },
});
