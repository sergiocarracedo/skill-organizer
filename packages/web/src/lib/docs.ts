import type { CollectionEntry } from "astro:content";

import { commandGroups } from "@/data/commands";

export const docsSections = [
  {
    slug: "guides",
    title: "Guides",
    description:
      "Narrative flows for install, onboarding, and day-two operations.",
  },
  {
    slug: "reference",
    title: "Reference",
    description: "Precise command surfaces, examples, and flags.",
  },
] as const;

export const sortDocs = (entries: CollectionEntry<"docs">[]) => {
  return [...entries].sort((left, right) => left.data.order - right.data.order);
};

export const getDocsBySection = (
  entries: CollectionEntry<"docs">[],
  section: (typeof docsSections)[number]["slug"],
) => {
  return sortDocs(entries.filter((entry) => entry.data.section === section));
};

export const getCommandBySlug = (slug: string) => {
  return commandGroups.find((command) => command.slug === slug);
};

export const trimCollectionSlug = (id: string) => {
  return id.split("/").at(-1) ?? id;
};
