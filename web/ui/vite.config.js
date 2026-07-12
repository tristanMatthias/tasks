import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath } from "node:url";

// Dev: served at / on :5180 with HMR, proxying the API to a local tasksd (:7842).
// Build: emitted to ../static with base /static/ so the Go binary embeds it and
// the asset URLs match the server's `GET /static/` route.
export default defineConfig(({ command }) => ({
  plugins: [svelte(), tailwindcss()],
  base: command === "build" ? "/static/" : "/",
  resolve: {
    alias: {
      // $lib is the vendored design system (shadcn primitives + cn); feature code
      // lives under domain-named roots (screaming architecture).
      $lib: fileURLToPath(new URL("./src/design-system", import.meta.url)),
      $tasks: fileURLToPath(new URL("./src/tasks", import.meta.url)),
      $board: fileURLToPath(new URL("./src/board", import.meta.url)),
      $shared: fileURLToPath(new URL("./src/shared", import.meta.url)),
    },
  },
  build: {
    outDir: "../static",
    emptyOutDir: true,
    assetsDir: "assets",
  },
  server: {
    host: "0.0.0.0",
    port: 5180,
    strictPort: true,
    allowedHosts: true, // allow access via Tailscale IP/MagicDNS hostnames
    proxy: {
      // New dev tasksd on 7850 (the old beads UI stays on 7842 for comparison).
      "/api": "http://127.0.0.1:7850",
      "/mcp": "http://127.0.0.1:7850",
      "/auth": "http://127.0.0.1:7850",
    },
  },
  preview: {
    host: "0.0.0.0",
    port: 5190,
    strictPort: true,
    allowedHosts: true,
    proxy: {
      "/api": "http://127.0.0.1:7850",
      "/mcp": "http://127.0.0.1:7850",
      "/auth": "http://127.0.0.1:7850",
    },
  },
}));
