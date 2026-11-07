package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Provider struct {
	BaseURL string            `json:"base_url"` // OpenAI-compatible root, e.g. https://host/v1
	KeyEnv  string            `json:"key_env"`  // env var holding the API key
	Headers map[string]string `json:"headers"`  // extra headers; "{session}" is replaced per run
}

type Tier struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Config struct {
	EnvFile      string              `json:"env_file"` // optional `export KEY=value` file read for missing keys
	DefaultTier  string              `json:"default_tier"`
	MaxSteps     int                 `json:"max_steps"`
	BashTimeoutS int                 `json:"bash_timeout_s"`
	Providers    map[string]Provider `json:"providers"`
	Tiers        map[string]Tier     `json:"tiers"`
}

func configPath() string {
	if p := os.Getenv("DELEGATE_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "delegate", "config.json")
}

// xdgDir uses XDG paths on every OS (not ~/Library on macOS) so install.sh/uninstall.sh agree with the binary.
func xdgDir(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func loadConfig() (*Config, error) {
	path := configPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w (copy config.example.json there)", path, err)
	}
	c := &Config{DefaultTier: "cheap", MaxSteps: 40, BashTimeoutS: 120}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// apiKey returns the key from the environment, falling back to env_file.
// The key is never exported into the process env, so the worker's bash tool can't see it.
func (c *Config) apiKey(name string) (string, error) {
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	if c.EnvFile != "" {
		if f, err := os.Open(expandHome(c.EnvFile)); err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimPrefix(strings.TrimSpace(sc.Text()), "export ")
				k, v, ok := strings.Cut(line, "=")
				if ok && strings.TrimSpace(k) == name {
					return strings.Trim(strings.TrimSpace(v), `"'`), nil
				}
			}
		}
	}
	return "", fmt.Errorf("API key %s not set (env or env_file)", name)
}
