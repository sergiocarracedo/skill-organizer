# skill-organizer npm package

This package installs the `skill-organizer` CLI by downloading the matching prebuilt binary from GitHub Releases.

Agent-first install and onboarding instructions are documented in [`../../AGENTS_README.md`](../../AGENTS_README.md).

## Install

```bash
npm i -g skill-organizer
```

Then verify:

```bash
skill-organizer --version
```

## Notes

- The install script downloads a release artifact for your current OS and architecture.
- The package version must match an existing GitHub release and its uploaded assets.
- Installs with `--ignore-scripts` are not supported.

## Troubleshooting

- If install fails with a download error, verify the matching version exists in GitHub Releases.
- If the binary is missing after install, reinstall without `--ignore-scripts`.
