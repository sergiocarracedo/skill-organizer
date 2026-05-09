export type CommandGroup = {
  slug: string;
  title: string;
  command: string;
  summary: string;
  whyItMatters: string;
  flags?: Array<{ flag: string; description: string }>;
  examples: string[];
};

export const installSnippets = {
  npm: ["npm i -g skill-organizer", "skill-organizer --version"],
  brew: ["brew tap sergiocarracedo/tap", "brew install skill-organizer"],
};

export const commandGroups: CommandGroup[] = [
  {
    slug: "completion",
    title: "Completion",
    command: "skill-organizer completion <shell>",
    summary: "Generate shell completion scripts.",
    whyItMatters:
      "Shell completion makes the command surface easier to discover, especially once you start using skill, service, and overlap subcommands regularly.",
    examples: [
      "skill-organizer completion zsh > ~/.zsh/completions/_skill-organizer",
      "skill-organizer completion bash --no-descriptions > ~/.local/share/bash-completion/completions/skill-organizer",
    ],
  },
  {
    slug: "onboard",
    title: "Onboard",
    command: "skill-organizer onboard",
    summary: "Guide first-time setup for a global skills project.",
    whyItMatters:
      "This is the fastest path from a messy flat skills directory to a managed source tree plus a synchronized target folder.",
    examples: ["skill-organizer onboard", "skill-organizer status"],
  },
  {
    slug: "project",
    title: "Project",
    command: "skill-organizer project <subcommand>",
    summary: "Add, edit, or remove a managed project configuration.",
    whyItMatters:
      "Use project commands when you want explicit control over a project instead of the guided onboarding flow.",
    examples: [
      "skill-organizer project add",
      "skill-organizer project edit --config ~/.agents/.skill-organizer.yml",
      "skill-organizer project remove",
    ],
  },
  {
    slug: "sync",
    title: "Sync",
    command: "skill-organizer sync",
    summary: "Synchronize managed skills into the target folder.",
    whyItMatters:
      "Run sync anytime you want the flat target view refreshed from the organized source of truth.",
    examples: [
      "skill-organizer sync",
      "skill-organizer sync --config ~/.agents/.skill-organizer.yml",
    ],
  },
  {
    slug: "status",
    title: "Status",
    command: "skill-organizer status",
    summary: "Show source, target, and sync status.",
    whyItMatters:
      "Status is the quickest way to understand drift, unmanaged entries, disabled skills, and broken links before you take action.",
    examples: [
      "skill-organizer status",
      "skill-organizer status --config ~/.agents/.skill-organizer.yml",
    ],
  },
  {
    slug: "skill",
    title: "Skill",
    command: "skill-organizer skill <subcommand>",
    summary: "Enable, disable, or move unmanaged source skills.",
    whyItMatters:
      "Skill-level commands let you reshape the source tree without hand-editing generated target directories.",
    flags: [
      {
        flag: "--yes",
        description:
          "Apply unmanaged moves without confirmation when using move-unmanaged.",
      },
      {
        flag: "--to",
        description:
          "Move a single unmanaged skill to a custom path relative to the source root.",
      },
      {
        flag: "--include-disabled",
        description: "Include disabled skills when using skill check-overlap.",
      },
      {
        flag: "--min-overlap-type",
        description:
          "Choose adjacent, partial, duplicate, or the numeric values 1, 2, and 3.",
      },
    ],
    examples: [
      "skill-organizer skill disable my-skills/coding-skills/astro-performance-auditor",
      "skill-organizer skill enable my-skills/coding-skills/astro-performance-auditor",
      "skill-organizer skill move-unmanaged --config ~/.agents/.skill-organizer.yml",
      "skill-organizer skill check-overlap --tool claude",
      "skill-organizer skill check-overlap --min-overlap-type partial",
    ],
  },
  {
    slug: "watched",
    title: "Watched",
    command: "skill-organizer watched <subcommand>",
    summary: "Manage watched project config paths.",
    whyItMatters:
      "The watched registry determines which projects the foreground watcher and background service keep synchronized.",
    examples: [
      "skill-organizer watched list",
      "skill-organizer watched add ~/.agents/.skill-organizer.yml",
      "skill-organizer watched remove",
    ],
  },
  {
    slug: "watch",
    title: "Watch",
    command: "skill-organizer watch",
    summary: "Watch registered projects and keep them synchronized.",
    whyItMatters:
      "Watch mode turns the CLI into a living bridge between what you edit in source and what your tools see in target.",
    examples: ["skill-organizer watch"],
  },
  {
    slug: "service",
    title: "Service",
    command: "skill-organizer service <subcommand>",
    summary: "Manage the background watcher service.",
    whyItMatters:
      "The service layer keeps sync running outside the terminal, making the organized workflow feel native and automatic.",
    examples: [
      "skill-organizer service install",
      "skill-organizer service start",
      "skill-organizer service log-level set debug",
      "skill-organizer service status",
    ],
  },
  {
    slug: "about",
    title: "About",
    command: "skill-organizer about",
    summary: "Show project and build information.",
    whyItMatters:
      "Useful for confirming the installed build, checking provenance, and linking users back to the repository.",
    examples: ["skill-organizer about"],
  },
];

export const workflowSteps = [
  {
    index: "01",
    title: "Choose the target",
    body: "Point the CLI at the flat skills folder your tool already expects, from generic `.agents` layouts to Claude Code, Codex, or Antigravity.",
  },
  {
    index: "02",
    title: "Adopt a source of truth",
    body: "Keep real edits inside `skills-organized/`, where folders can stay nested, intentional, and easy to curate over time.",
  },
  {
    index: "03",
    title: "Generate the flat view",
    body: "Flatten source-relative paths with `--`, write managed metadata, and expose the tool-facing directory through symlinks.",
  },
  {
    index: "04",
    title: "Keep everything in sync",
    body: "Run `sync` on demand or use watch mode and the background service so the target view updates itself while you work.",
  },
];

export const featureCards = [
  {
    title: "Nested source, flat target",
    copy: "Edit the structure you want, expose the compatibility layer your tools require.",
  },
  {
    title: "Non-destructive sync",
    copy: "Managed symlinks are repaired and stale ones removed without deleting unrelated unmanaged target entries.",
  },
  {
    title: "Disable without deleting",
    copy: "Mark a skill disabled in source metadata and remove it from the generated target on the next sync.",
  },
  {
    title: "Watch locally or as a service",
    copy: "Use a foreground watcher while developing or install a background service for continuous updates.",
  },
];
