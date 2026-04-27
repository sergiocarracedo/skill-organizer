#!/usr/bin/env bash

set -euo pipefail

release_current_branch() {
  git rev-parse --abbrev-ref HEAD 2>/dev/null || true
}

release_version() {
  tr -d '[:space:]' < VERSION
}

release_npm_version() {
  node -p "require('./packages/npm/package.json').version"
}

release_require_branch() {
  local expected="$1"
  local current
  current="$(release_current_branch)"
  if [[ "$current" != "$expected" ]]; then
    printf 'release helper must be run from the %s branch. Current branch: %s\n' "$expected" "${current:-unknown}" >&2
    exit 1
  fi
}

release_require_version_consistency() {
  local repo_version npm_version
  repo_version="$(release_version)"
  npm_version="$(release_npm_version)"

  if [[ -z "$repo_version" ]]; then
    printf 'VERSION is empty\n' >&2
    exit 1
  fi
  if [[ "$repo_version" != "$npm_version" ]]; then
    printf 'VERSION (%s) does not match packages/npm/package.json version (%s)\n' "$repo_version" "$npm_version" >&2
    exit 1
  fi
}

release_require_version_kind() {
  local kind="$1"
  local version
  version="$(release_version)"
  case "$kind" in
    alpha)
      [[ "$version" == *-alpha.* ]] || {
        printf 'Expected an alpha version in VERSION, got %s\n' "$version" >&2
        exit 1
      }
      ;;
    beta)
      [[ "$version" == *-beta.* ]] || {
        printf 'Expected a beta version in VERSION, got %s\n' "$version" >&2
        exit 1
      }
      ;;
    stable)
      [[ "$version" != *-* ]] || {
        printf 'Expected a stable version in VERSION, got %s\n' "$version" >&2
        exit 1
      }
      ;;
    *)
      printf 'Unknown release kind: %s\n' "$kind" >&2
      exit 1
      ;;
  esac
}

release_validate_common() {
  printf 'Running Go validation...\n'
  go test ./...
  go build ./...

  printf 'Running npm wrapper validation...\n'
  node --check packages/npm/scripts/postinstall.js
  node --check packages/npm/bin/skill-organizer.js

  printf 'Checking version consistency...\n'
  release_require_version_consistency
}

release_warn_missing_secret() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    printf 'Warning: %s is not set in the environment. This is fine for local pre-flight checks, but CI publishing will require it.\n' "$name"
  fi
}

release_print_reminder() {
  local branch="$1"
  local flavor="$2"
  printf '\n%s release workflow reminder:\n' "$flavor"
  printf '1. Commit using a conventional commit message.\n'
  printf '2. Push the %s branch.\n' "$branch"
  printf '3. Wait for Release Please to open/update the release PR.\n'
  printf '4. Merge the release PR to create the next %s tag and GitHub release.\n' "$flavor"
}
