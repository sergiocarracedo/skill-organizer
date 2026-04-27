#!/usr/bin/env bash

set -euo pipefail

AGENTS_DIR="${HOME}/.agents"
CONFIG_FILE="${AGENTS_DIR}/.skill-organizer.yml"
CLI_BIN="/works/opensource/skill-organizer/cli/skill-organizer"

run_step() {
  local command="$1"
  printf '\n$ %s\n' "$command"
  sleep 1
  eval "$command"
  sleep 1
}

main() {
  export TERM=xterm-256color

  run_step "cd \"${AGENTS_DIR}\" && pwd"
  run_step "ls"
  run_step "ls skills | grep asciinema || true"
  run_step "npx skills add https://github.com/terrylica/cc-skills --skill asciinema-recorder --agent universal --yes --global"
  run_step "ls skills | grep asciinema || true"
  run_step "\"${CLI_BIN}\" status --config \"${CONFIG_FILE}\""
}

main "$@"
