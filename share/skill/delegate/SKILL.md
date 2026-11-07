---
name: delegate
description: Hand low-reasoning coding chores to a cheap model via the `delegate` CLI to save orchestrator tokens. Use for codebase surveys, boilerplate, tests from a spec, mechanical refactors/renames, lint/type fixes, docs, log triage. Not for planning, architecture, ambiguous debugging, security, or final review.
---

# Delegate chores to a cheap worker

You are the orchestrator: you plan, decide, and verify. `delegate` runs a cheap model
(DeepSeek/GLM/etc. over any OpenAI-compatible gateway) as a headless worker with
read/write/edit/bash tools in a directory, and returns only a short report.

## Delegate vs. keep

| Delegate (cheap worker) | Keep (you) |
|---|---|
| Find/survey code: "where is X used, list call sites" | Planning, architecture, trade-offs |
| Boilerplate, scaffolding, config from a clear spec | Ambiguous bugs, root-cause analysis |
| Tests from a spec; fix lint/type errors | Security, auth, money, data-loss paths |
| Mechanical refactors, renames, pattern migrations | Reviewing the worker's diff |
| Docs, changelogs, log/trace triage | Anything where a wrong answer fails silently |

## Write the task (be specific; the worker has no context but the task)

```
Goal: <one sentence>
Context: <the 2-5 facts it needs: file paths, function names, conventions>
Scope: <files/dirs it may change; everything else is read-only>
Constraints: <style, no new deps, keep API, etc.>
Check: <command that must pass, e.g. `go test ./pkg/...`>
```

## Run

```bash
delegate -C <repo> "<task text>"          # or: delegate -C <repo> task.md
delegate -t mid ...                        # tiers: cheap (default) | mid | strong
```

Independent tasks: run several in background, each in its own git worktree.

## Verify (don't re-read everything)

1. Exit code 0 = worker reported `STATUS: done`; 1 = failed/blocked; 2 = config/API error.
2. `git -C <repo> diff --stat`, then read only the hunks that matter.
3. Run the Check command yourself.

## Escalate

Failed twice on `cheap` → retry on `mid`/`strong` with a sharper task (add what went wrong).
Still failing → do it yourself. Full transcript is at the `log=` path on stderr if needed.
