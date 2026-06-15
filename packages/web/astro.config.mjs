import mdx from "@astrojs/mdx";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import icon from "astro-icon";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const cliVersion = readFileSync(
  resolve(import.meta.dirname, "../../VERSION"),
  "utf8",
).trim();

export default defineConfig({
  site: "https://skill-organizer.sergiocarracedo.es",
  integrations: [mdx(), icon()],
  vite: {
    define: {
      "import.meta.env.PUBLIC_CLI_VERSION": JSON.stringify(cliVersion),
    },
    plugins: [tailwindcss()],
  },
});
