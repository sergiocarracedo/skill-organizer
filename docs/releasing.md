# Releasing

This project uses:

- `VERSION` as the repo version source of truth
- [Release Please](https://github.com/googleapis/release-please) for version bumps, changelog, and release PRs
- [GoReleaser](https://goreleaser.com/) for cross-platform GitHub release binaries

The current phase is **alpha**.

## Versioning

Versions follow semver and may include prerelease identifiers:

- `0.0.1-alpha.1`
- `0.1.0-beta.1`
- `0.1.0`

The version is stored in:

```text
VERSION
```

Release Please updates both:

- `VERSION`
- `release-please-manifest.json`
- `packages/npm/package.json`

Release Please also maintains:

- `CHANGELOG.md`

## Conventional Commits

Commits should follow conventional commit style:

- `feat: add onboard flow`
- `fix: sync after enable and disable`
- `docs: document service logs`
- `refactor: simplify status rendering`
- `chore: release 0.0.1-alpha.1`

Breaking changes should use either:

- `feat!: change config format`
- a `BREAKING CHANGE:` footer

## Branches

Recommended release tracks:

- `alpha`: prerelease track for early adopters
- `beta`: optional prerelease stabilization track
- `main`: stable releases

For now, use `alpha` as the main release branch.

Current repository automation supports separate release tracks:

- `alpha` uses `release-please-config.alpha.json`
- `beta` uses `release-please-config.beta.json`
- `main` uses `release-please-config.stable.json`

## Release Flow

### Alpha releases

1. Merge conventional-commit changes into `alpha`.
2. Release Please opens or updates a release PR.
3. Merge the release PR.
4. Release Please creates a tag and GitHub prerelease.
5. The release workflow runs GoReleaser and uploads binaries.

For a local pre-flight check on the `alpha` branch, you can run:

```bash
./scripts/release-alpha.sh
```

This helper validates:

- current branch is `alpha`
- `VERSION` matches `packages/npm/package.json`
- current version is an alpha prerelease
- `go test ./...`
- `go build ./...`
- npm wrapper script syntax

It also warns if `NPM_TOKEN` is not set in the current environment.

### Beta releases

1. Merge stabilization changes into `beta`.
2. Release Please opens or updates a beta release PR.
3. Merge the release PR.
4. Release Please creates a beta tag and GitHub prerelease.
5. The release workflow runs GoReleaser and uploads binaries.

For a local pre-flight check on the `beta` branch, you can run:

```bash
./scripts/release-beta.sh
```

This helper validates the same shared checks as alpha, but requires:

- current branch is `beta`
- current version is a beta prerelease

### Stable releases

1. Merge release-ready changes into `main`.
2. Merge the Release Please release PR.
3. Release Please creates the stable tag and GitHub release.
4. GoReleaser uploads release artifacts.

For a local pre-flight check on the `main` branch, you can run:

```bash
./scripts/release-stable.sh
```

This helper validates the same shared checks, but requires:

- current branch is `main`
- current version is stable with no prerelease suffix

It also warns if either of these environment variables are missing:

- `NPM_TOKEN`
- `HOMEBREW_TAP_GITHUB_TOKEN`

## GitHub Actions

Workflows:

- `.github/workflows/ci.yml`
- `.github/workflows/release-please.yml`
- `.github/workflows/release.yml`
- `.github/workflows/publish-npm.yml`

The CI workflow also validates pull request titles against a conventional commit pattern.

## GoReleaser

GoReleaser builds archives for:

- Linux amd64, arm64
- macOS amd64, arm64
- Windows amd64

It injects build metadata into the binary version output:

- version
- commit
- date

## npm distribution

This repository now includes an npm wrapper package in:

```text
packages/npm
```

The npm package downloads the matching prebuilt GitHub Release binary during `postinstall`.
It also verifies the archive against the published `checksums.txt` file before extraction.

Recommended tags:

- alpha releases: `npm publish --tag alpha`
- beta releases: `npm publish --tag beta`
- stable releases: `npm publish --tag latest`

Example user installs:

```bash
npm i -g skill-organizer@alpha
npm i -g skill-organizer@beta
npm i -g skill-organizer
```

What you need to do:

1. Reserve the npm package name.
2. Add an `NPM_TOKEN` secret to GitHub Actions.
3. Confirm the `name`, `homepage`, `repository`, and `bugs` fields in `packages/npm/package.json`.
4. Publish by pushing a release tag or merging a Release Please PR that creates one.

The publish workflow determines the dist-tag automatically:

- `*-alpha.*` -> `alpha`
- `*-beta.*` -> `beta`
- anything else -> `latest`

## Homebrew distribution

Homebrew should publish **stable releases only**.

Recommended setup:

1. Create a separate tap repository, for example `sergiocarracedo/homebrew-tap`.
2. Add a token named `HOMEBREW_TAP_GITHUB_TOKEN`.
3. Publish only from stable tags on `main`.

Example user install:

```bash
brew tap sergiocarracedo/tap
brew install skill-organizer
```

What you need to do:

1. Create the tap repository.
2. Add a token with permission to push formula updates to the tap repo.
3. Review the generated brew metadata in `.goreleaser.yaml`.

The repository already includes a stable-only GoReleaser `brews` section. Because it uses `skip_upload: auto`, prerelease tags are ignored automatically.

## First alpha release

Recommended first public release:

```text
0.0.1-alpha.1
```

To prepare it:

1. Create or use the `alpha` branch.
2. Push the current changes there.
3. Let Release Please open the release PR.
4. Merge the release PR.

## Files involved in releases

- `VERSION`
- `CHANGELOG.md`
- `release-please-manifest.json`
- `release-please-config.alpha.json`
- `release-please-config.beta.json`
- `release-please-config.stable.json`
- `.goreleaser.yaml`
- `packages/npm/package.json`
- `scripts/release-common.sh`
- `scripts/release-alpha.sh`
- `scripts/release-beta.sh`
- `scripts/release-stable.sh`

## Notes

- Homebrew prereleases are intentionally not published.
- npm prereleases should use dist-tags.
- Stable releases should come from `main`.
