# Homebrew tap notes

The GoReleaser config includes a `brews` section for stable releases only.

It is intentionally configured with:

- `skip_upload: auto`

That means prerelease tags such as `v0.0.1-alpha.1` and `v0.0.1-beta.1` will not be pushed to Homebrew.

Before enabling stable Homebrew publishing, you need:

1. A tap repository, for example `sergiocarracedo/homebrew-tap`
2. A GitHub token stored as `HOMEBREW_TAP_GITHUB_TOKEN`
3. A real repository license value if you do not want to keep `UNLICENSED`

Stable installation target:

```bash
brew tap sergiocarracedo/tap
brew install skill-organizer
```
