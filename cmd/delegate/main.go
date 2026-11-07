// delegate runs a cheap model as a headless coding worker over any
// OpenAI-compatible chat-completions API. Zero third-party dependencies.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// systemPrompt is the stable cache prefix. Change it rarely: every edit invalidates worker caches.
const systemPrompt = `You are a worker agent doing a delegated coding chore in the current working directory.
Rules:
- Use the tools to inspect and change files. Do only what the task asks; touch only files in scope.
- Prefer small exact edits (edit_file) over rewriting whole files. Read before you edit.
- Use bash with rg/find/ls to locate code instead of reading many whole files.
- If the task gives a check command, run it and fix failures before finishing.
- Never ask questions. If blocked, stop and report blocked with the reason.
When finished, reply WITHOUT any tool call, with exactly this report (max 150 words):
STATUS: done|failed|blocked
FILES_CHANGED: <comma-separated paths, or none>
CHECK_RESULT: <command and pass/fail, or none>
NOTES: <only what the orchestrator must know>`

const maxReport = 2000

// version is set at release build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

var statusRe = regexp.MustCompile(`(?i)STATUS:\s*(\w+)`)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("delegate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tierName := fs.String("t", "", "tier from config (default: default_tier)")
	modelOver := fs.String("m", "", "override as provider:model")
	dir := fs.String("C", ".", "working directory for the worker")
	maxSteps := fs.Int("max-steps", 0, "max model turns (default: config max_steps)")
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: delegate [-t tier] [-m provider:model] [-C dir] TASK_FILE | - | task text...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	fail := func(f string, a ...any) int { fmt.Fprintf(stderr, "delegate: "+f+"\n", a...); return 2 }

	task, err := readTask(fs.Args())
	if err != nil {
		return fail("%v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return fail("%v", err)
	}
	if *maxSteps > 0 {
		cfg.MaxSteps = *maxSteps
	}
	tier, label, err := pickTier(cfg, *tierName, *modelOver)
	if err != nil {
		return fail("%v", err)
	}
	prov, ok := cfg.Providers[tier.Provider]
	if !ok {
		return fail("unknown provider %q", tier.Provider)
	}
	key, err := cfg.apiKey(prov.KeyEnv)
	if err != nil {
		return fail("%v", err)
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		return fail("%v", err)
	}

	session := "delegate-" + randHex(6)
	client := newClient(prov, key, session)
	tools := &Tools{Root: root, BashTimeout: time.Duration(cfg.BashTimeoutS) * time.Second, Changed: map[string]bool{}, HideEnv: map[string]bool{}}
	for _, p := range cfg.Providers {
		tools.HideEnv[p.KeyEnv] = true
	}
	messages := []Message{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": task},
	}
	logPath := writeLogOnExit(session, &messages)
	defer logPath.flush()

	var in, cached, out, steps int
	report := ""
	for steps < cfg.MaxSteps {
		steps++
		body := map[string]any{"model": tier.Model, "messages": messages, "tools": toolSchemas}
		if steps == cfg.MaxSteps {
			// Last turn: force a report instead of another tool call.
			messages = append(messages, Message{"role": "user", "content": "Step limit reached. Stop now and reply with the report."})
			body["messages"], body["tool_choice"] = messages, "none"
		}
		msg, calls, u, err := client.Chat(body)
		if err != nil {
			return fail("%s step %d: %v (log: %s)", label, steps, err, logPath.path)
		}
		in, cached, out = in+u.PromptTokens, cached+u.PromptTokensDetails.CachedTokens, out+u.CompletionTokens
		messages = append(messages, msg)
		if len(calls) == 0 {
			report, _ = msg["content"].(string)
			break
		}
		for _, c := range calls {
			messages = append(messages, Message{"role": "tool", "tool_call_id": c.ID, "content": tools.Run(c.Function.Name, c.Function.Arguments)})
		}
	}

	report = strings.TrimSpace(report)
	if report == "" {
		report = "STATUS: failed\nNOTES: worker produced no report"
	}
	fmt.Fprintln(stdout, truncate(report, maxReport))
	fmt.Fprintf(stderr, "[delegate] %s steps=%d in=%d cached=%s out=%d touched=%s log=%s\n",
		label, steps, in, pct(cached, in), out, changedList(tools.Changed), logPath.path)
	if m := statusRe.FindStringSubmatch(report); m != nil && strings.EqualFold(m[1], "done") {
		return 0
	}
	return 1
}

func readTask(args []string) (string, error) {
	switch {
	case len(args) == 0:
		return "", fmt.Errorf("no task given (file path, - for stdin, or text)")
	case len(args) == 1 && args[0] == "-":
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	case len(args) == 1:
		if b, err := os.ReadFile(args[0]); err == nil {
			return string(b), nil
		}
	}
	return strings.Join(args, " "), nil
}

func pickTier(cfg *Config, name, override string) (Tier, string, error) {
	if override != "" {
		p, m, ok := strings.Cut(override, ":")
		if !ok {
			return Tier{}, "", fmt.Errorf("-m must be provider:model")
		}
		return Tier{Provider: p, Model: m}, override, nil
	}
	if name == "" {
		name = cfg.DefaultTier
	}
	t, ok := cfg.Tiers[name]
	if !ok {
		return Tier{}, "", fmt.Errorf("unknown tier %q", name)
	}
	return t, name + "=" + t.Provider + ":" + t.Model, nil
}

type runLog struct {
	path     string
	messages *[]Message
}

// writeLogOnExit saves the full transcript for debugging; the orchestrator only sees the report.
func writeLogOnExit(session string, msgs *[]Message) *runLog {
	dir := filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "delegate")
	os.MkdirAll(dir, 0o700)
	return &runLog{path: filepath.Join(dir, session+".json"), messages: msgs}
}

func (l *runLog) flush() {
	b, _ := json.MarshalIndent(*l.messages, "", " ")
	os.WriteFile(l.path, b, 0o600)
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func pct(a, b int) string {
	if b == 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", a*100/b)
}

func changedList(m map[string]bool) string {
	if len(m) == 0 {
		return "none"
	}
	var s []string
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}
