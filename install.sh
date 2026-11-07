#!/usr/bin/env bash
# Install delegate on Linux or macOS (bash 3.2+). Undo with ./uninstall.sh.
# Binary source, first match wins:
#   1. DELEGATE_BIN=/path/to/binary ./install.sh
#   2. build from source, if Go is installed
#   3. download from the latest GitHub release (checksum verified)
set -euo pipefail
cd "$(dirname "$0")"
. share/lib.sh

mkdir -p "$(dirname "$BIN")"
if [ -n "${DELEGATE_BIN:-}" ]; then
  cp "$DELEGATE_BIN" "$BIN"
elif command -v go >/dev/null; then
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN" ./cmd/delegate
  echo "built from source"
else
  download_release "$BIN"
fi
chmod +x "$BIN"
# macOS: binaries downloaded outside git can be quarantined; clear it so it runs.
[ "$(uname)" = Darwin ] && xattr -d com.apple.quarantine "$BIN" 2>/dev/null || true

[ -f "$CFG_DIR/config.json" ] || { mkdir -p "$CFG_DIR"; cp share/config.example.json "$CFG_DIR/config.json"; echo "seeded $CFG_DIR/config.json"; }

for d in $SKILL_DIRS; do
  [ -d "$d" ] || continue
  # Never clobber (or nest a link inside) a user's own 'delegate' skill.
  if [ -e "$d/delegate" ] && [ ! -L "$d/delegate" ]; then echo "skipped $d/delegate (exists, not ours)"; continue; fi
  ln -sfn "$SKILL_SRC" "$d/delegate" && echo "linked skill -> $d"
done

for f in $RULE_FILES; do
  [ -d "$(dirname "$f")" ] || continue
  add_block "$f" && echo "added rules -> $f"
done

echo "installed $BIN"
case ":$PATH:" in
  *":$(dirname "$BIN"):"*) ;;
  *) echo "note: $(dirname "$BIN") is not on PATH; add: export PATH=\"\$HOME/.local/bin:\$PATH\" to your shell rc" ;;
esac
