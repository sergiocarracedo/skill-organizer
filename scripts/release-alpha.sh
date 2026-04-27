#!/usr/bin/env bash

set -euo pipefail

. "$(dirname "$0")/release-common.sh"

release_require_branch alpha
release_require_version_kind alpha

printf 'Running validation on alpha branch...\n'
release_validate_common
release_warn_missing_secret NPM_TOKEN

release_print_reminder alpha alpha
