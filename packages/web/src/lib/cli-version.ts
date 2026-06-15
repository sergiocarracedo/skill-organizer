export const getCliVersion = () =>
  import.meta.env.PUBLIC_CLI_VERSION?.trim() || "dev";

export const cliVersion = getCliVersion();
