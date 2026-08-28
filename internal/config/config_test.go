package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `version: 2
compose:
  file: compose.yml
paths:
  - name: multi
    from:
      env:
        APP_TAG: v1
    via:
      - env:
          APP_TAG: v2
      - env:
          APP_TAG: v3
    to:
      env:
        APP_TAG: current
      build:
        services: [app]
        tag_env: APP_TAG
health:
  type: http
  url: http://127.0.0.1:8080/health
  timeout: 10s
  interval: 1s
seed:
  command: ./seed.sh
  timeout: 5s
verify:
  checks:
    - name: preserved
      command: ./check.sh
      timeout: 5s
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "upgradeproof.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadStrictAndMultiHopOrdering(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	states := append([]ReleaseState{cfg.Paths[0].From}, cfg.Paths[0].Via...)
	states = append(states, cfg.Paths[0].To)
	var tags []string
	for _, state := range states {
		tags = append(tags, state.Env["APP_TAG"])
	}
	if got := strings.Join(tags, ","); got != "v1,v2,v3,current" {
		t.Fatalf("unexpected declared order: %s", got)
	}
	if got := cfg.Paths[0].To.Build.Services[0]; got != "app" {
		t.Fatalf("unexpected build service: %s", got)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	_, err := Load(writeConfig(t, validConfig+"surprise: true\n"))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected strict decoding error, got %v", err)
	}
}

func TestLoadRejectsBuildOutsideTarget(t *testing.T) {
	body := strings.Replace(validConfig, "APP_TAG: v1", "APP_TAG: v1\n      build:\n        services: [app]\n        tag_env: APP_TAG", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "only allowed for a target") {
		t.Fatalf("expected source build rejection, got %v", err)
	}
}

func TestLoadRejectsInvalidReleaseState(t *testing.T) {
	tests := []struct{ old, replacement string }{
		{"timeout: 10s", "timeout: forever"},
		{"env:\n        APP_TAG: v1", "env: {}"},
		{"APP_TAG: v1", "UPGRADEPROOF_FAKE: v1"},
		{"tag_env: APP_TAG", "tag_env: OTHER_TAG"},
	}
	for _, tc := range tests {
		body := strings.Replace(validConfig, tc.old, tc.replacement, 1)
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Fatalf("expected error for replacement %q", tc.replacement)
		}
	}
}
