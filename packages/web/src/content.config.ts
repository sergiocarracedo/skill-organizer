import { glob } from "astro/loaders";
import { defineCollection } from "astro:content";
import { z } from "astro:schema";

const docs = defineCollection({
  loader: glob({ pattern: "**/*.{md,mdx}", base: "./src/content/docs" }),
  schema: z.object({
    title: z.string(),
    description: z.string(),
    section: z.enum(["guides", "reference"]),
    order: z.number(),
    commandSlug: z.string().optional(),
    summary: z.string().optional(),
  }),
});

export const collections = { docs };
