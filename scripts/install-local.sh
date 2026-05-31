#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BIN_DIR="${HOME}/bin"
LINK="${BIN_DIR}/keel"

mkdir -p "$BIN_DIR"
ln -sf "$ROOT/keel" "$LINK"

cat <<EOF
Installed keel launcher:
  $LINK -> $ROOT/keel

Run:
  keel

If your shell cannot find it, add this to your shell profile:
  export PATH="\$HOME/bin:\$PATH"
EOF
