#!/usr/bin/env bash
# Remove everything install.sh created (Linux/macOS). The repo itself is left alone.
#   ./uninstall.sh               remove binary, skill links, rule blocks, config, logs
#   ./uninstall.sh --keep-config keep ~/.config/delegate (your edited tiers/providers)
set -euo pipefail
cd "$(dirname "$0")"
. share/lib.sh

keep_config=0
[ "${1:-}" = "--keep-config" ] && keep_config=1

rm_path() { if [ -e "$1" ] || [ -L "$1" ]; then rm -rf "$1" && echo "removed $1"; fi; }

rm_path "$BIN"

# Only remove skill links that point into this repo (any past layout); never touch a user's own skill.
for d in $SKILL_DIRS; do
  link="$d/delegate"
  if [ -L "$link" ] && case "$(readlink "$link")" in "$PWD"/*) true ;; *) false ;; esac; then
    rm "$link" && echo "removed $link"
  elif [ -e "$link" ]; then
    echo "skipped $link (not a link to this repo)"
  fi
done

for f in $RULE_FILES; do
  if [ -f "$f" ] && grep -q '^<!-- delegate:start' "$f"; then
    strip_block "$f" && echo "removed rules from $f"
    [ -s "$f" ] || { rm "$f" && echo "removed empty $f"; }  # file held only our block
  fi
done

[ "$keep_config" = 1 ] || rm_path "$CFG_DIR"
rm_path "$CACHE_DIR"
echo "delegate uninstalled"
