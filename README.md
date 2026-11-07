# delegate

**Your expensive AI plans. A cheap AI does the chores.**

> **TL;DR:** Opus runs `delegate "fix the lint errors"`. A cheap model does it and sends back a short report.
> Fewer Opus tokens, same result.

- One binary, no dependencies
- Any OpenAI-compatible gateway (OpenCode Go, OpenRouter, ...)
- Any agent that can run a shell command (Claude Code, OpenCode, Codex, ...)

---

## Quick start

**1. Install**

```bash
git clone https://github.com/fahimscirex/delegate && cd delegate
./install.sh
```

**2. Add your API key** to `~/.bashrc` or `~/.zshrc`

```bash
export OPENCODE_GO_API_KEY=your-key
```

**3. Start a new agent session.** That's it.

---

## How it works

```
Opus ──"do this chore"──▶ delegate ──▶ cheap model (edits files, runs tests)
  ▲                                         │
  └────────── short report (≤150 words) ◀───┘
```

| Expensive model | Cheap model |
|---|---|
| Plans | Searches code |
| Decides | Writes boilerplate and tests |
| Reviews | Renames, lint fixes, docs |

---

## Three ways to use it

**1. Do nothing.** Your agent delegates chores by itself.

**2. Ask for it** in Claude Code:

```
/delegate add unit tests for parseConfig
```

In OpenCode or Codex, say: "use the delegate skill to ...".

**3. Run it yourself:**

```bash
delegate -C ~/repo "Fix the failing test in calc_test.go"
```

| Flag | Meaning |
|---|---|
| `-t` | Tier: `cheap` (default), `mid`, `strong` |
| `-C` | Folder to work in |
| `-m` | Exact model: `provider:model` |
| `-max-steps` | Max turns (default 40) |
| `-version` | Print version |

Exit codes: `0` done · `1` failed · `2` setup error

---

## Config

File: `~/.config/delegate/config.json`

**Change a model:**

```json
"cheap": { "provider": "opencode-go", "model": "deepseek-v4.1-flash" }
```

**Add a gateway:**

```json
"openrouter": { "base_url": "https://openrouter.ai/api/v1", "key_env": "OPENROUTER_API_KEY" }
```

<details>
<summary>All fields</summary>

- `providers.*.base_url`: OpenAI-compatible URL, ends in `/v1`
- `providers.*.key_env`: env var that holds the key
- `providers.*.headers`: extra headers; `{session}` becomes a per-run id (OpenCode Go needs `x-opencode-session`)
- `tiers.*`: `provider` + `model`
- `default_tier`, `max_steps`, `bash_timeout_s`: defaults
- `env_file`: optional file of `export KEY=value` lines, used only if the key isn't in the environment
- `$DELEGATE_CONFIG`: use a different config file

</details>

---

## Customize the skill

| Edit | To change | Then |
|---|---|---|
| `share/skill/delegate/SKILL.md` | What gets delegated, and how | Nothing (it's a link) |
| `share/rules.md` | The always-on rules | Re-run `./install.sh` |

Start a new agent session after editing. Mid-session edits break the prompt cache.

---

## Uninstall

```bash
./uninstall.sh                 # remove everything
./uninstall.sh --keep-config   # keep your config
```

---

## Good to know

- **macOS:** no Go needed. Install downloads the right binary from the latest release.
- **Updating:** `git pull && ./install.sh`
- **Safety:** the worker can't see your API keys, and file edits stay in the `-C` folder.
- **But:** its shell isn't sandboxed. Use a git worktree for risky jobs.

---

<details>
<summary>What install touches</summary>

| Thing | Where |
|---|---|
| Binary | `~/.local/bin/delegate` |
| Config | `~/.config/delegate/` |
| Logs | `~/.cache/delegate/` |
| Skill link | `~/.claude/skills`, `~/.config/opencode/skills`, `~/.codex/skills` |
| Rules block | `~/.claude/CLAUDE.md`, `~/.config/opencode/AGENTS.md`, `~/.codex/AGENTS.md` |

- Binary source, first match wins: `DELEGATE_BIN` → build with Go → download release (checksum checked).
- The rules block sits between `<!-- delegate:start -->` and `<!-- delegate:end -->`. Uninstall removes only that block.
- A skill named `delegate` that isn't ours is never touched.

</details>

<details>
<summary>Why the prompt cache stays warm</summary>

- The system prompt and tools never change, so they stay cached.
- History only grows; nothing is rewritten.
- One session id per run keeps the gateway on the same backend.
- Opus only gets the short report, not the whole job.

</details>

<details>
<summary>Developing and releasing</summary>

```
cmd/delegate/        Go source (standard library only)
share/               config example, rules, skill, shell helpers
.github/workflows/   CI tests and releases
```

```bash
make test                                  # offline tests
make bin                                   # local release build into bin/
git tag v0.1.1 && git push origin v0.1.1   # CI builds and publishes a release
```

</details>
