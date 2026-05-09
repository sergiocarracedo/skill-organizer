import mdx from "@astrojs/mdx";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import icon from "astro-icon";

export default defineConfig({
  site: "https://skill-organizer.sergiocarracedo.es",
  integrations: [mdx(), icon()],
  vite: {
    plugins: [tailwindcss()],
  },
});
