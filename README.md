# delegate

**Your expensive AI plans. A cheap AI does the chores.**

> **TL;DR:** Claude Opus (or any coding agent) runs `delegate "fix the lint errors"`.
> A cheap model does the work and replies with a 150-word report. You spend far fewer Opus tokens.

- One binary. Zero dependencies.
- Works with any OpenAI-compatible gateway (OpenCode Go, OpenRouter, ...).
- Works from any agent that can run a shell command (Claude Code, Codex, OpenCode, ...).

---

## Quick start

**1. Install**

```bash
git clone https://github.com/fahimscirex/delegate && cd delegate
./install.sh
```

**2. Set your API key**

```bash
export OPENCODE_GO_API_KEY=your-key    # put this in ~/.bashrc or ~/.zshrc
```

**3. Try it**

```bash
delegate -C ~/my-project "Fix the failing test in calc_test.go"
```

That's it. From now on your agent delegates chores by itself.

---

## How it works

```
Opus ──"do this chore"──▶ delegate ──▶ cheap model (edits files, runs tests)
  ▲                                         │
  └────────── short report (≤150 words) ◀───┘
```

| Who | Does what |
|---|---|
| **Opus / Fable** (expensive) | Plans, decides, reviews |
| **delegate** (cheap) | Searches code, boilerplate, tests, renames, lint fixes, docs |

---

## Usage

```bash
delegate "task text"              # cheap tier, current folder
delegate -C ~/repo task.md        # task from a file, in another folder
delegate -t strong "task"         # use a smarter tier
echo "task" | delegate -          # task from stdin
```

| Flag | Meaning |
|---|---|
| `-t` | Tier: `cheap` (default), `mid`, `strong` |
| `-C` | Folder the worker works in |
| `-m` | Pick an exact model: `provider:model` |
| `-max-steps` | Stop after N turns (default 40) |

**Exit codes:** `0` done · `1` failed or blocked · `2` setup or API error

---

## Config

File: `~/.config/delegate/config.json` (or set `$DELEGATE_CONFIG`)

**Change models.** Edit one line in `tiers`:

```json
"cheap": { "provider": "opencode-go", "model": "deepseek-v4.1-flash" }
```

**Add a gateway.** Add an entry to `providers`:

```json
"openrouter": { "base_url": "https://openrouter.ai/api/v1", "key_env": "OPENROUTER_API_KEY" }
```

<details>
<summary>All config fields</summary>

- `providers.*.base_url`: OpenAI-compatible root URL (ends in `/v1`).
- `providers.*.key_env`: name of the env var that holds the key.
- `providers.*.headers`: extra headers. `{session}` becomes a per-run id. OpenCode Go needs `x-opencode-session`.
- `tiers.*`: `provider` + `model`.
- `default_tier`, `max_steps`, `bash_timeout_s`: defaults.
- `env_file` (optional): a file of `export KEY=value` lines, read only when the key isn't in the environment.

</details>

---

## Uninstall

```bash
./uninstall.sh                 # removes everything
./uninstall.sh --keep-config   # keeps your config
```

<details>
<summary>What install / uninstall touch</summary>

| Thing | Where |
|---|---|
| Binary | `~/.local/bin/delegate` |
| Config | `~/.config/delegate/` |
| Logs (full transcripts) | `~/.cache/delegate/` |
| Skill link | `~/.claude/skills`, `~/.config/opencode/skills`, `~/.codex/skills` |
| Rules block | `~/.claude/CLAUDE.md`, `~/.config/opencode/AGENTS.md`, `~/.codex/AGENTS.md` |

- The rules block sits between `<!-- delegate:start -->` and `<!-- delegate:end -->`. Uninstall removes only that block.
- Uninstall never touches a skill named `delegate` that isn't ours.
- Same paths on Linux and macOS.

</details>

---

## macOS

Same steps. You don't need Go.

- `install.sh` builds from source if Go is installed.
- Otherwise it downloads the binary for your Mac (Apple Silicon or Intel) from the latest release and checks its checksum.
- Linux works the same way (amd64 and arm64).

---

## Safety

- File tools can't leave the `-C` folder.
- The worker's shell can't see your API keys.
- **The shell itself is not sandboxed.** Use a git worktree or container for risky jobs.

---

<details>
<summary>Why the prompt cache stays warm</summary>

- The system prompt and tool list never change, so the model's cache can reuse them.
- History only grows; nothing is rewritten mid-run.
- Each run sends one session id, so the gateway keeps routing to the same backend.
- The orchestrator only gets the short report, which barely adds to its context.

</details>

<details>
<summary>Developing</summary>

```
cmd/delegate/        Go source (standard library only)
share/               config example, agent rules, skill, shared shell code
.github/workflows/   CI tests and releases
install.sh           install
uninstall.sh         uninstall
```

```bash
make test    # vet + tests (offline, fake provider)
make bin     # build release binaries locally into bin/ (gitignored)
```

**Release:** push a tag. CI tests, builds macOS and Linux binaries, and publishes a GitHub release.

```bash
git tag v0.1.0 && git push origin v0.1.0
```

</details>
