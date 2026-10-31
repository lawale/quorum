// Vitest global setup: builds and runs the Quorum test server (the Go
// apitest harness as a process), so the suite exercises the real server under
// OpenAPI validation. At teardown the server exits non-zero if any traffic
// disagreed with the document or any documented operation was never called,
// which fails the run.
import { spawn, type ChildProcess } from "node:child_process";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import type { TestProject } from "vitest/node";

declare module "vitest" {
  export interface ProvidedContext {
    quorumUrl: string;
  }
}

const repoRoot = resolve(import.meta.dirname, "../../..");

let server: ChildProcess | undefined;
let buildDir: string | undefined;
let stderr = "";

export async function setup(project: TestProject): Promise<void> {
  buildDir = mkdtempSync(join(tmpdir(), "quorum-apiserver-"));
  const bin = join(buildDir, "apiserver");
  execFileSync("go", ["build", "-o", bin, "./internal/apitest/cmd/apiserver"], { cwd: repoRoot, stdio: "inherit" });

  server = spawn(bin, [], { cwd: repoRoot, stdio: ["ignore", "pipe", "pipe"] });
  server.stderr!.setEncoding("utf8");
  server.stderr!.on("data", (chunk: string) => {
    stderr += chunk;
    process.stderr.write(chunk);
  });

  const url = await new Promise<string>((resolveUrl, reject) => {
    let out = "";
    server!.stdout!.setEncoding("utf8");
    server!.stdout!.on("data", (chunk: string) => {
      out += chunk;
      const nl = out.indexOf("\n");
      if (nl >= 0) resolveUrl(out.slice(0, nl).trim());
    });
    server!.once("exit", (code) => reject(new Error(`apiserver exited with ${code} before reporting its URL\n${stderr}`)));
  });
  project.provide("quorumUrl", url);
}

export async function teardown(): Promise<void> {
  if (!server) return;
  const exit = new Promise<number | null>((resolveCode) => server!.once("exit", (code) => resolveCode(code)));
  server.kill("SIGTERM");
  const code = await exit;
  if (buildDir) rmSync(buildDir, { recursive: true, force: true });
  if (code !== 0) {
    throw new Error(`apiserver reported contract failures (exit ${code}); see output above`);
  }
}
