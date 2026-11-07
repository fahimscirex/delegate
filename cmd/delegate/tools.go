package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxToolOutput = 12000 // chars returned to the model per tool call

// Tool schemas. Order and text must stay stable: they are part of the cached prompt prefix.
var toolSchemas = []map[string]any{
	fn("read_file", "Read a text file. Returns numbered lines.", map[string]any{
		"path":   str("File path relative to the working directory"),
		"offset": num("1-based start line (default 1)"),
		"limit":  num("Max lines (default 400)"),
	}, "path"),
	fn("write_file", "Create or overwrite a file with the given content.", map[string]any{
		"path":    str("File path relative to the working directory"),
		"content": str("Full file content"),
	}, "path", "content"),
	fn("edit_file", "Replace one exact, unique occurrence of old_string with new_string.", map[string]any{
		"path":       str("File path relative to the working directory"),
		"old_string": str("Exact text to replace; must occur exactly once"),
		"new_string": str("Replacement text"),
	}, "path", "old_string", "new_string"),
	fn("bash", "Run a shell command in the working directory. Use for ls, rg, find, tests, git.", map[string]any{
		"command": str("Shell command"),
	}, "command"),
}

func fn(name, desc string, props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "function", "function": map[string]any{
		"name": name, "description": desc,
		"parameters": map[string]any{"type": "object", "properties": props, "required": required},
	}}
}
func str(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
func num(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }

type Tools struct {
	Root        string
	BashTimeout time.Duration
	Changed     map[string]bool
	HideEnv     map[string]bool // env vars (API keys) removed from the bash tool's environment
}

// resolve keeps file tools inside Root. shortcut: bash can still escape; use a worktree/container for real isolation.
func (t *Tools) resolve(p string) (string, error) {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(t.Root, p)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(t.Root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path %q is outside the working directory", p)
	}
	return abs, nil
}

func (t *Tools) Run(name, argsJSON string) string {
	var a map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "ERROR: invalid JSON arguments: " + err.Error()
	}
	s := func(k string) string { v, _ := a[k].(string); return v }
	n := func(k string, def int) int {
		if v, ok := a[k].(float64); ok && v > 0 {
			return int(v)
		}
		return def
	}
	out, err := t.dispatch(name, s, n)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return truncate(out, maxToolOutput)
}

func (t *Tools) dispatch(name string, s func(string) string, n func(string, int) int) (string, error) {
	switch name {
	case "read_file":
		p, err := t.resolve(s("path"))
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(b), "\n")
		off, lim := n("offset", 1), n("limit", 400)
		var sb strings.Builder
		for i := off - 1; i < len(lines) && i < off-1+lim; i++ {
			fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
		}
		if off-1+lim < len(lines) {
			fmt.Fprintf(&sb, "... (%d more lines)\n", len(lines)-(off-1+lim))
		}
		return sb.String(), nil
	case "write_file":
		p, err := t.resolve(s("path"))
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(s("content")), 0o644); err != nil {
			return "", err
		}
		t.Changed[s("path")] = true
		return fmt.Sprintf("wrote %d bytes to %s", len(s("content")), s("path")), nil
	case "edit_file":
		p, err := t.resolve(s("path"))
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		old := s("old_string")
		if old == "" {
			return "", fmt.Errorf("old_string is empty")
		}
		if c := strings.Count(string(b), old); c != 1 {
			return "", fmt.Errorf("old_string found %d times, need exactly 1; add surrounding context", c)
		}
		info, _ := os.Stat(p)
		if err := os.WriteFile(p, []byte(strings.Replace(string(b), old, s("new_string"), 1)), info.Mode()); err != nil {
			return "", err
		}
		t.Changed[s("path")] = true
		return "edited " + s("path"), nil
	case "bash":
		ctx, cancel := context.WithTimeout(context.Background(), t.BashTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "-c", s("command"))
		cmd.Dir = t.Root
		for _, kv := range os.Environ() {
			if k, _, _ := strings.Cut(kv, "="); !t.HideEnv[k] {
				cmd.Env = append(cmd.Env, kv)
			}
		}
		out, err := cmd.CombinedOutput()
		res := string(out)
		if ctx.Err() == context.DeadlineExceeded {
			res += fmt.Sprintf("\n[timed out after %s]", t.BashTimeout)
		} else if err != nil {
			res += "\n[" + err.Error() + "]"
		} else {
			res += "\n[exit 0]"
		}
		return res, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// truncate keeps head and tail, which is where errors and summaries usually are.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	h := max * 2 / 3
	return s[:h] + fmt.Sprintf("\n... [%d chars cut] ...\n", len(s)-max) + s[len(s)-(max-h):]
}
