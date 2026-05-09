import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const fallbackVersion = process.env.SKILL_ORGANIZER_VERSION?.trim() || "dev";

const readCliVersion = () => {
  const candidates = [
    fileURLToPath(new URL("../../../VERSION", import.meta.url)),
    fileURLToPath(new URL("../../../../VERSION", import.meta.url)),
  ];

  for (const filePath of candidates) {
    if (existsSync(filePath)) {
      return readFileSync(filePath, "utf8").trim();
    }
  }

  return fallbackVersion;
};

export const cliVersion = readCliVersion();
