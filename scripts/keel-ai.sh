#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
LEDGER="${KEEL_LEDGER:-$HOME/tasks.md}"
SKILL="$ROOT/docs/keel-agent-skill.md"
LEDGER_DIR=$(dirname "$LEDGER")

if [ "$#" -eq 0 ]; then
  printf "AI prompt> " >&2
  IFS= read -r PROMPT
else
  PROMPT="$*"
fi

if [ -z "$PROMPT" ]; then
  echo "keel-ai: empty prompt" >&2
  exit 2
fi

exec opencode run \
  --dir "$LEDGER_DIR" \
  --file "$SKILL" \
  --file "$LEDGER" \
  --title "Keel task agent" \
  "Use the attached Keel skill. The canonical ledger is $LEDGER. User request: $PROMPT"
