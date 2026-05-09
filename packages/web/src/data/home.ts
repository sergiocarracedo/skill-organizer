export type InstallMethod = {
  id: string;
  label: string;
  command?: string;
  secondaryText?: string;
  note: string;
  href: string;
  hrefLabel: string;
  linkOnly?: boolean;
};

export type LogoItem = {
  id:
    | "claude"
    | "opencode"
    | "codex"
    | "cursor"
    | "antigravity"
    | "agents"
    | "macos"
    | "linux";
  name: string;
};

export type TerminalColor =
  | "default"
  | "muted"
  | "cyan"
  | "violet"
  | "amber"
  | "green"
  | "red"
  | "white";

export type TerminalChunk = {
  text: string;
  color?: TerminalColor;
  bold?: boolean;
};

export type TerminalLine = string | TerminalChunk[];

export type TerminalEvent =
  | {
      type: "prompt";
      content: string;
      prompt?: string;
      delay?: number;
    }
  | {
      type: "output";
      content: TerminalLine[];
      delay?: number;
    }
  | {
      type: "progress";
      label: string;
      delay: number;
      color?: TerminalColor;
    }
  | {
      type: "wait";
      delay: number;
    };

export type TerminalScenario = {
  id: string;
  label: string;
  description: string;
  accent: "cyan" | "violet" | "amber";
  events: TerminalEvent[];
};

export type FeatureCard = {
  title: string;
  copy: string;
  shape: "bevel" | "arc" | "slant" | "ticket" | "hex" | "shield";
  rotation: number;
  speed: number;
};

export type OverlapGroupDemo = {
  title: string;
  skills: string[];
  overlapType: "Duplicate" | "Partial";
  score: number;
  why: string;
  recommendation: string;
};

export const heroInstallMethods: InstallMethod[] = [
  {
    id: "npm",
    label: "npm",
    command: "npm i -g skill-organizer",
    secondaryText: "Then run skill-organizer --version",
    note: "Installs the CLI through npm and fetches the matching prebuilt binary for your platform.",
    href: "https://www.npmjs.com/package/skill-organizer",
    hrefLabel: "View npm package",
  },
  {
    id: "pnpm",
    label: "pnpm",
    command: "pnpm add -g skill-organizer",
    secondaryText: "Then run skill-organizer --version",
    note: "Best fit when you already use pnpm for your global CLI tooling.",
    href: "https://www.npmjs.com/package/skill-organizer",
    hrefLabel: "View npm package",
  },
  {
    id: "brew",
    label: "brew",
    command: "brew install sergiocarracedo/tap/skill-organizer",
    secondaryText: "Homebrew tap available if needed",
    note: "Best fit when your dev tooling already lives in Homebrew.",
    href: "https://github.com/sergiocarracedo/homebrew-tap",
    hrefLabel: "Open tap",
  },
  {
    id: "binary",
    label: "binary",
    linkOnly: true,
    note: "Use the release page when you want a direct archive instead of a package manager.",
    href: "https://github.com/sergiocarracedo/skill-organizer/releases",
    hrefLabel: "GitHub Releases",
  },
];

export const heroLogoItems: LogoItem[] = [
  { id: "macos", name: "macOS" },
  { id: "linux", name: "Linux" },
];

export const compatibilityItems: LogoItem[] = [
  { id: "claude", name: "Claude Code" },
  { id: "opencode", name: "OpenCode" },
  { id: "codex", name: "Codex" },
  { id: "cursor", name: "Cursor" },
  { id: "antigravity", name: "Antigravity" },
];

export const terminalScenarios: TerminalScenario[] = [
  {
    id: "install",
    label: "Install",
    description:
      "Install the CLI, then verify the exact binary you will use before touching a project.",
    accent: "cyan",
    events: [
      { type: "prompt", content: "npm i -g skill-organizer" },
      {
        type: "progress",
        label: "Resolving package and platform binary",
        delay: 920,
        color: "cyan",
      },
      {
        type: "output",
        content: [
          [{ text: "+ skill-organizer@latest", color: "green", bold: true }],
          [{ text: "installed package and platform binary", color: "muted" }],
        ],
      },
      { type: "wait", delay: 260 },
      { type: "prompt", content: "skill-organizer --version" },
      {
        type: "output",
        content: [[{ text: "0.0.5", color: "white", bold: true }]],
      },
    ],
  },
  {
    id: "onboard",
    label: "Onboard",
    description:
      "Guide a target into a managed source tree without giving up the flat layout your tool expects.",
    accent: "violet",
    events: [
      { type: "prompt", content: "skill-organizer onboard" },
      {
        type: "output",
        content: [
          [
            { text: "?", color: "amber", bold: true },
            {
              text: " Select the tool to onboard  Generic (.agents) -> ~/.agents/skills",
              color: "white",
            },
          ],
          [
            { text: "?", color: "amber", bold: true },
            {
              text: " Select the source skills-organized folder for your .agents setup  ~/.agents/skills-organized",
              color: "white",
            },
          ],
        ],
      },
      {
        type: "progress",
        label: "Writing project config and syncing links",
        delay: 1180,
        color: "violet",
      },
      {
        type: "output",
        content: [
          [
            {
              text: "Configured Generic (.agents) project at: ~/.agents/.skill-organizer.yml",
              color: "green",
              bold: true,
            },
          ],
          [{ text: "Source: ~/.agents/skills-organized", color: "muted" }],
          [{ text: "Target: ~/.agents/skills", color: "muted" }],
        ],
      },
    ],
  },
  {
    id: "status",
    label: "Status",
    description:
      "Confirm managed, disabled, and synced skills before you start shaping the library.",
    accent: "amber",
    events: [
      {
        type: "prompt",
        content: "skill-organizer status",
      },
      {
        type: "progress",
        label: "Reading source and target state",
        delay: 860,
        color: "amber",
      },
      {
        type: "output",
        content: [
          [{ text: "# Project", color: "amber", bold: true }],
          [
            {
              text: "Config: /works/test/agents-test/.skill-organizer.yml",
              color: "muted",
            },
          ],
          [
            {
              text: "Source: /works/test/agents-test/skills-organized",
              color: "muted",
            },
          ],
          [{ text: "Target: /works/test/agents-test/skills", color: "muted" }],
          [{ text: "", color: "default" }],
          [{ text: "# Skills", color: "amber", bold: true }],
          [{ text: "├─ 3rdparty", color: "cyan", bold: true }],
          [{ text: "│  ├─ agent-tools", color: "cyan", bold: true }],
          [
            {
              text: "│  │  ├─ agent-browser -> 3rdparty--agent-tools--agent-browser [synced]",
              color: "green",
            },
          ],
          [
            {
              text: "│  │  └─ skill-judge -> 3rdparty--agent-tools--skill-judge [synced]",
              color: "green",
            },
          ],
          [{ text: "│  └─ mattpocock", color: "cyan", bold: true }],
          [
            {
              text: "│     ├─ tdd -> 3rdparty--mattpocock--tdd [disabled]",
              color: "white",
            },
          ],
          [
            {
              text: "│     └─ write-a-prd -> 3rdparty--mattpocock--write-a-prd [synced]",
              color: "green",
            },
          ],
          [{ text: "├─ personal", color: "cyan", bold: true }],
          [
            {
              text: "│  ├─ agent-tasks -> personal--agent-tasks [disabled]",
              color: "white",
            },
          ],
          [{ text: "│  └─ coding", color: "cyan", bold: true }],
          [
            {
              text: "│     └─ frontend-project-bootstrap -> personal--coding--frontend-project-bootstrap [synced]",
              color: "green",
            },
          ],
          [{ text: "└─ test -> test [synced]", color: "green" }],
          [{ text: "", color: "default" }],
          [{ text: "# Unmanaged target entries", color: "amber", bold: true }],
          [{ text: "3rdparty--mattpocock--tdd", color: "white" }],
          [{ text: "", color: "default" }],
          [{ text: "# Summary", color: "amber", bold: true }],
          [
            { text: "Total skills:", color: "white", bold: true },
            { text: " 33", color: "white", bold: true },
          ],
          [
            { text: "Managed skills:", color: "white", bold: true },
            { text: " 31", color: "white", bold: true },
          ],
          [
            { text: "Unmanaged skills:", color: "white", bold: true },
            { text: " 1", color: "white", bold: true },
          ],
          [
            { text: "Synced:", color: "white", bold: true },
            { text: " 31", color: "green", bold: true },
          ],
          [
            { text: "Disabled:", color: "white", bold: true },
            { text: " 2", color: "amber", bold: true },
          ],
          [
            { text: "Missing target:", color: "white", bold: true },
            { text: " 0", color: "muted" },
          ],
          [
            { text: "Broken link:", color: "white", bold: true },
            { text: " 0", color: "muted" },
          ],
          [
            { text: "Drifted:", color: "white", bold: true },
            { text: " 0", color: "muted" },
          ],
        ],
      },
    ],
  },
  {
    id: "service",
    label: "Service",
    description:
      "Check whether the background watcher service is actually running.",
    accent: "violet",
    events: [
      { type: "prompt", content: "skill-organizer service status" },
      {
        type: "progress",
        label: "Querying background service",
        delay: 760,
        color: "violet",
      },
      {
        type: "output",
        content: [
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Service status: running", color: "cyan" },
          ],
        ],
      },
    ],
  },
  {
    id: "disable",
    label: "Disable",
    description:
      "Disable a source skill and let sync remove its generated target entry immediately.",
    accent: "amber",
    events: [
      {
        type: "prompt",
        content:
          "skill-organizer skill disable personal/coding/frontend-project-bootstrap",
      },
      {
        type: "progress",
        label: "Updating source metadata and syncing target",
        delay: 980,
        color: "amber",
      },
      {
        type: "output",
        content: [
          [
            { text: "SUCCESS", color: "green", bold: true },
            {
              text: " Disabled skill: personal/coding/frontend-project-bootstrap",
              color: "green",
            },
          ],
          [
            { text: "SUCCESS", color: "green", bold: true },
            {
              text: " Synchronized project config: /tmp/agents-test-disable-PAfKoI/.skill-organizer.yml",
              color: "green",
            },
          ],
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Enabled skills: 30", color: "cyan" },
          ],
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Disabled skills: 3", color: "cyan" },
          ],
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Created links: 0", color: "cyan" },
          ],
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Updated links: 0", color: "cyan" },
          ],
          [
            { text: "INFO", color: "cyan", bold: true },
            { text: " Removed stale links: 1", color: "cyan" },
          ],
        ],
      },
    ],
  },
];

export const installOnboardNotes = [
  {
    step: "01",
    title: "Install once",
    body: "Install from npm, Homebrew, or a release binary, then verify the CLI before touching your skills folder.",
  },
  {
    step: "02",
    title: "Onboard the target",
    body: "Use the guided flow to create the project config, select the target, and set up the sibling skills-organized source tree.",
  },
  {
    step: "03",
    title: "Operate from source",
    body: "Once onboarded, edit the nested source tree and let sync, watch mode, or the background service keep the generated view current.",
  },
];

export const featureCards: FeatureCard[] = [
  {
    title: "Organized source, compatible output",
    copy: "Use a real folder hierarchy for humans while exposing the generated compatibility layer agent tools already know how to read.",
    shape: "bevel",
    rotation: -3.2,
    speed: 0.16,
  },
  {
    title: "Multiple skill projects",
    copy: "Manage global skills and per-project targets from the same CLI instead of treating every setup as a one-off.",
    shape: "shield",
    rotation: 2.2,
    speed: 0.09,
  },
  {
    title: "Disable without deleting",
    copy: "Hide a skill from the generated output without deleting the source folder, metadata, or the curation work behind it.",
    shape: "arc",
    rotation: 2.4,
    speed: 0.08,
  },
  {
    title: "Background sync",
    copy: "Run watch mode in the foreground or install the service so the generated view stays fresh while you work elsewhere.",
    shape: "slant",
    rotation: -1.8,
    speed: 0.2,
  },
  {
    title: "Overlap analysis",
    copy: "Use an installed agent to flag duplicate or ambiguous skills before your library turns into guesswork.",
    shape: "ticket",
    rotation: 3.1,
    speed: 0.11,
  },
  {
    title: "Adopt unmanaged entries",
    copy: "Move loose target folders into a real source tree instead of renaming, copying, or cleaning up generated output by hand.",
    shape: "hex",
    rotation: -2.7,
    speed: 0.18,
  },
];

export const advantageItems = [
  "One organized source of truth for human editing",
  "Tool-readable output that existing agent tools already understand",
  "Manage multiple skill targets with one consistent workflow",
  "Disable or re-enable skills without deleting your curation work",
  "Non-destructive sync that leaves unrelated target entries alone",
];

export const overlapDemoGroups: OverlapGroupDemo[] = [
  {
    title: "Group 1",
    skills: [
      "personal/coding/frontend-project-bootstrap",
      "personal/react/react-project-bootstrap",
    ],
    overlapType: "Duplicate",
    score: 92,
    why: "Both bootstrap modern TypeScript projects with the same tooling stack and validation workflow. The React version is effectively a specialization of the frontend version, but the descriptions are close enough that routing could be ambiguous.",
    recommendation:
      "Merge them or make the separation explicit: reserve the frontend skill for framework-agnostic SPA or library bootstraps and the React skill only for repos that require React-specific files, conventions, and examples.",
  },
  {
    title: "Group 2",
    skills: [
      "3rdparty/mattpocock/write-a-prd",
      "3rdparty/mattpocock/prd-to-plan",
      "3rdparty/mattpocock/prd-to-issues",
      "3rdparty/mattpocock/request-refactor-plan",
    ],
    overlapType: "Partial",
    score: 76,
    why: "These all sit in the planning-artifact pipeline: turning ideas or refactors into structured docs, phased plans, or GitHub issues. They are not duplicates, but their entry points and output boundaries are close enough that a user asking to plan work could plausibly match several.",
    recommendation:
      "Define a strict workflow boundary: write-a-prd creates requirements, prd-to-plan converts an approved PRD into an implementation plan, prd-to-issues converts a finalized PRD into tickets, and request-refactor-plan is reserved for codebase refactors.",
  },
];

export const docsEntryCards = [
  {
    title: "Install guide",
    copy: "Install from npm, Homebrew, or direct binaries and verify the CLI before your first project.",
    href: "/docs/guides/install-and-verify",
  },
  {
    title: "Onboard a tool",
    copy: "Follow the guided flow for generic .agents locations or tool-specific layouts like Claude Code and Codex.",
    href: "/docs/guides/onboard-a-tool",
  },
  {
    title: "Command reference",
    copy: "Jump to the exact syntax for sync, service, overlap analysis, watch mode, and completion output.",
    href: "/docs/reference/onboard",
  },
];

export const faqItems = [
  {
    question: "Why not edit the flat target folder directly?",
    answer:
      "Because that view is generated. Editing the source tree keeps structure, naming, and metadata consistent while sync rebuilds the tool-facing layout safely.",
  },
  {
    question: "Which agent tools work with skill-organizer?",
    answer:
      "Use the generic .agents target for tools that read ~/.agents/skills, or onboard directly for Claude Code, Codex, and Antigravity. The overlap checker can also invoke installed agent CLIs such as Claude Code, OpenCode, Codex, Cursor, and Antigravity.",
  },
  {
    question: "Can I hide a skill without deleting it?",
    answer:
      "Yes. skill-organizer skill disable marks the source skill as disabled and removes it from the generated view on the next sync.",
  },
  {
    question: "Do I have to run sync manually every time?",
    answer:
      "No. You can run sync on demand, use the foreground watcher, or install the background service to keep everything fresh automatically.",
  },
  {
    question: "What does the overlap check do?",
    answer:
      "It asks an installed coding agent to analyze your managed skills for duplication or overlap, renders grouped reports, and can open a plan-only flow or save a remediation prompt under plans/ for tools without verified interactive plan mode.",
  },
];

export const servicePanels = [
  {
    eyebrow: "Registry",
    title: "Feed the watcher with project configs",
    commands: [
      "skill-organizer watched add ~/.agents/.skill-organizer.yml",
      "skill-organizer watched list",
    ],
    note: "The watched registry is the handoff between one-off setup and continuous sync.",
  },
  {
    eyebrow: "Background sync",
    title: "Install, start, and inspect the service",
    commands: [
      "skill-organizer service install",
      "skill-organizer service start",
      "skill-organizer service status",
      "skill-organizer service log-level set debug",
    ],
    note: "Once enabled, the source tree becomes the real editing surface while the generated view stays fresh automatically.",
  },
];
