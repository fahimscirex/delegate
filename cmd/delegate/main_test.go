package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fake provider: scripted replies, records requests. Exercises the full loop offline.
func TestLoopWritesEditsAndReports(t *testing.T) {
	work := t.TempDir()
	replies := []string{
		`{"role":"assistant","content":null,"tool_calls":[{"id":"1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"a.txt\",\"content\":\"hello world\"}"}}]}`,
		`{"role":"assistant","content":null,"reasoning_content":"keep me","tool_calls":[{"id":"2","type":"function","function":{"name":"edit_file","arguments":"{\"path\":\"a.txt\",\"old_string\":\"world\",\"new_string\":\"go\"}"}}]}`,
		`{"role":"assistant","content":null,"tool_calls":[{"id":"3","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"../escape.txt\"}"}}]}`,
		`{"role":"assistant","content":"STATUS: done\nFILES_CHANGED: a.txt\nCHECK_RESULT: none\nNOTES: ok"}`,
	}
	var reqs []map[string]any
	var sessions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad auth header %q", r.Header.Get("Authorization"))
		}
		sessions = append(sessions, r.Header.Get("x-session"))
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		reqs = append(reqs, body)
		i := len(reqs) - 1
		w.Write([]byte(`{"choices":[{"message":` + replies[i] + `}],"usage":{"prompt_tokens":100,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":90}}}`))
	}))
	defer srv.Close()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(cfgPath, []byte(`{"default_tier":"cheap","providers":{"fake":{"base_url":"`+srv.URL+`/v1","key_env":"DELEGATE_TEST_KEY","headers":{"x-session":"{session}"}}},"tiers":{"cheap":{"provider":"fake","model":"m"}}}`), 0o644)
	t.Setenv("DELEGATE_CONFIG", cfgPath)
	t.Setenv("DELEGATE_TEST_KEY", "k")

	var out, errb bytes.Buffer
	code := run([]string{"-C", work, "do", "the", "thing"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if b, _ := os.ReadFile(filepath.Join(work, "a.txt")); string(b) != "hello go" {
		t.Fatalf("file = %q", b)
	}
	if !strings.HasPrefix(out.String(), "STATUS: done") {
		t.Fatalf("report = %q", out.String())
	}
	if !strings.Contains(errb.String(), "cached=90%") || !strings.Contains(errb.String(), "touched=a.txt") {
		t.Fatalf("stats = %q", errb.String())
	}
	if len(reqs) != 4 || sessions[0] == "" || sessions[0] != sessions[3] || !strings.HasPrefix(sessions[0], "delegate-") {
		t.Fatalf("reqs=%d sessions=%v", len(reqs), sessions)
	}
	// Prefix stability: each request extends the previous one (cache-friendly append-only history).
	last := reqs[3]["messages"].([]any)
	if len(last) != 8 || last[0].(map[string]any)["content"] != systemPrompt {
		t.Fatalf("history len %d", len(last))
	}
	if last[4].(map[string]any)["reasoning_content"] != "keep me" {
		t.Fatal("provider-specific field not round-tripped")
	}
	if !strings.Contains(last[7].(map[string]any)["content"].(string), "outside the working directory") {
		t.Fatalf("escape not blocked: %v", last[7])
	}
}

func TestStepLimitForcesReport(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		if lastBody["tool_choice"] == "none" {
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"STATUS: failed\nNOTES: ran out"}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"x","type":"function","function":{"name":"bash","arguments":"{\"command\":\"true\"}"}}]}}]}`))
	}))
	defer srv.Close()
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":{"f":{"base_url":"`+srv.URL+`","key_env":"DELEGATE_TEST_KEY"}},"tiers":{"cheap":{"provider":"f","model":"m"}}}`), 0o644)
	t.Setenv("DELEGATE_CONFIG", cfgPath)
	t.Setenv("DELEGATE_TEST_KEY", "k")
	var out, errb bytes.Buffer
	if code := run([]string{"-C", t.TempDir(), "-max-steps", "3", "loop"}, &out, &errb); code != 1 {
		t.Fatalf("exit %d, stderr %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "ran out") {
		t.Fatalf("report %q", out.String())
	}
}

func TestEnvFileKey(t *testing.T) {
	f := filepath.Join(t.TempDir(), "env")
	os.WriteFile(f, []byte("# c\nexport OTHER=x\nexport DELEGATE_NOPE_KEY=\"sekret\"\n"), 0o600)
	c := &Config{EnvFile: f}
	if k, err := c.apiKey("DELEGATE_NOPE_KEY"); err != nil || k != "sekret" {
		t.Fatalf("got %q %v", k, err)
	}
}

func TestBashToolHidesAPIKeys(t *testing.T) {
	t.Setenv("DELEGATE_SECRET_KEY", "sekret")
	t.Setenv("DELEGATE_VISIBLE", "fine")
	tl := &Tools{Root: t.TempDir(), BashTimeout: 5e9, Changed: map[string]bool{}, HideEnv: map[string]bool{"DELEGATE_SECRET_KEY": true}}
	out := tl.Run("bash", `{"command":"echo k=$DELEGATE_SECRET_KEY v=$DELEGATE_VISIBLE"}`)
	if strings.Contains(out, "sekret") || !strings.Contains(out, "v=fine") {
		t.Fatalf("got %q", out)
	}
}
