import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it, vi } from "vitest";

vi.stubEnv(
  "PUBLIC_CLI_VERSION",
  readFileSync(
    fileURLToPath(new URL("../../../VERSION", import.meta.url)),
    "utf8",
  ).trim(),
);

import { getCliVersion } from "@/lib/cli-version";
import { getCommandBySlug, trimCollectionSlug } from "@/lib/docs";

const cliVersionFilePath = fileURLToPath(
  new URL("../../../VERSION", import.meta.url),
);

describe("docs helpers", () => {
  it("keeps the displayed CLI version in sync with the repo VERSION", () => {
    expect(getCliVersion()).toBe(
      readFileSync(cliVersionFilePath, "utf8").trim(),
    );
  });

  it("returns the correct command metadata", () => {
    expect(getCommandBySlug("skill")?.command).toBe(
      "skill-organizer skill <subcommand>",
    );
  });

  it("trims collection ids to route slugs", () => {
    expect(trimCollectionSlug("reference/service")).toBe("service");
  });
});
