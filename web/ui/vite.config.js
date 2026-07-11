import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";

// Build the Svelte UI to ../static so the Go binary embeds it (web/web.go embeds
// `static`). base "/static/" makes emitted asset URLs match the server's
// `GET /static/` route; index.html is served at "/". Dev proxies the API/MCP to
// a local tasksd on :7842.
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  base: "/static/",
  build: {
    outDir: "../static",
    emptyOutDir: true,
    assetsDir: "assets",
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:7842",
      "/mcp": "http://127.0.0.1:7842",
      "/auth": "http://127.0.0.1:7842",
    },
  },
});
