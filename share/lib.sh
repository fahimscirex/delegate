# Shared by install.sh and uninstall.sh (bash 3.2+, Linux/macOS). Sourced, not run.
BIN="$HOME/.local/bin/delegate"
CFG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/delegate"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/delegate"
SKILL_SRC="$PWD/share/skill/delegate"
# Release binary name for this machine, e.g. delegate-darwin-arm64.
case "$(uname -m)" in x86_64 | amd64) ARCH=amd64 ;; arm64 | aarch64) ARCH=arm64 ;; *) ARCH=$(uname -m) ;; esac
ASSET="delegate-$(uname -s | tr "[:upper:]" "[:lower:]")-$ARCH"
RELEASE_URL="${DELEGATE_RELEASE_URL:-https://github.com/fahimscirex/delegate/releases/latest/download}"
SKILL_DIRS="$HOME/.claude/skills $HOME/.config/opencode/skills $HOME/.codex/skills"
# Agent instruction files that get the rules.md block (only if the agent's dir exists).
RULE_FILES="$HOME/.claude/CLAUDE.md $HOME/.config/opencode/AGENTS.md $HOME/.codex/AGENTS.md"
START='<!-- delegate:start (managed by delegate install.sh; uninstall.sh removes it) -->'
END='<!-- delegate:end -->'

# strip_block FILE: remove our marked block plus the one blank line install added before it.
# Writes through `cat >` so symlinked files stay symlinks and keep their permissions.
strip_block() {
  [ -f "$1" ] || return 0
  grep -q '^<!-- delegate:start' "$1" || return 0
  tmp=$(mktemp)
  awk '
    skip { if ($0 == "<!-- delegate:end -->") skip = 0; next }
    /^<!-- delegate:start/ { if (blanks > 0) blanks--; for (; blanks > 0; blanks--) print ""; skip = 1; next }
    /^$/ { blanks++; next }
    { for (; blanks > 0; blanks--) print ""; print }
    END { for (; blanks > 0; blanks--) print "" }
  ' "$1" > "$tmp" && cat "$tmp" > "$1"
  rm -f "$tmp"
}

# add_block FILE: idempotent; replaces an older block with the current rules.md.
add_block() {
  strip_block "$1"
  if [ -s "$1" ] && [ -n "$(tail -c1 "$1")" ]; then echo >> "$1"; fi
  { [ -s "$1" ] && echo; echo "$START"; cat share/rules.md; echo "$END"; } >> "$1"
}

# download_release DEST: fetch $ASSET from the latest release and verify it against checksums.txt.
download_release() {
  tmp=$(mktemp -d)
  if ! curl -fsSL -o "$tmp/$ASSET" "$RELEASE_URL/$ASSET" || ! curl -fsSL -o "$tmp/checksums.txt" "$RELEASE_URL/checksums.txt"; then
    rm -rf "$tmp"; echo "download of $ASSET failed. Install Go and re-run to build from source." >&2; exit 1
  fi
  # sha256sum on Linux, shasum on macOS; same checksum file format.
  if command -v sha256sum >/dev/null; then sum="sha256sum -c -"; else sum="shasum -a 256 -c -"; fi
  if ! (cd "$tmp" && grep " $ASSET\$" checksums.txt | $sum >/dev/null); then
    rm -rf "$tmp"; echo "checksum mismatch for $ASSET; not installing." >&2; exit 1
  fi
  cp "$tmp/$ASSET" "$1"; rm -rf "$tmp"
  echo "downloaded $ASSET from release"
}
