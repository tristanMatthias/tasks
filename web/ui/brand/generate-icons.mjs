// Generate the full favicon / app-icon set from a single source image.
// Source: brand/icon.png (the app-icon with background).
// Output: public/ (Vite copies to web/static, served under /static/).
//   Re-run with:  node brand/generate-icons.mjs
import { favicons } from "favicons";
import { writeFile, mkdir } from "node:fs/promises";

const SOURCE = "brand/icon.png";
const OUT = "public";

const config = {
  path: "/static", // href prefix in the emitted <link> tags (Go serves web/static here)
  appName: "Tasks",
  appShortName: "Tasks",
  background: "#0f1115",
  theme_color: "#0f1115",
  icons: {
    favicons: true, // favicon.ico + favicon-16/32/48
    android: true, // android-chrome-192/512 + web manifest
    appleIcon: true, // apple-touch-icon (180)
    appleStartup: false,
    windows: false,
    yandex: false,
  },
};

const res = await favicons(SOURCE, config);
await mkdir(OUT, { recursive: true });
for (const img of res.images) await writeFile(`${OUT}/${img.name}`, img.contents);
for (const f of res.files) await writeFile(`${OUT}/${f.name}`, f.contents);

console.log("wrote", res.images.length, "images +", res.files.length, "files to", OUT);
console.log("\n--- <head> tags ---\n" + res.html.join("\n"));
