#!/usr/bin/env bash

set -euo pipefail

. "$(dirname "$0")/release-common.sh"

release_require_branch main
release_require_version_kind stable

printf 'Running validation on main branch...\n'
release_validate_common
release_warn_missing_secret NPM_TOKEN
release_warn_missing_secret HOMEBREW_TAP_GITHUB_TOKEN

release_print_reminder main stable
