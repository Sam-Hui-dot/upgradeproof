package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `version: 1
compose:
  file: compose.yml
  service: app
  image_env: UPGRADEPROOF_IMAGE
paths:
  - name: multi
    from: app:v1
    via: [app:v2, app:v3]
    to:
      build:
        service: app
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
	got := strings.Join(append(append([]string{cfg.Paths[0].From}, cfg.Paths[0].Via...), "build:"+cfg.Paths[0].To.Build.Service), ",")
	if got != "app:v1,app:v2,app:v3,build:app" {
		t.Fatalf("unexpected declared order: %s", got)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	_, err := Load(writeConfig(t, validConfig+"surprise: true\n"))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected strict decoding error, got %v", err)
	}
}

func TestLoadRejectsAmbiguousTarget(t *testing.T) {
	body := strings.Replace(validConfig, "to:\n      build:", "to:\n      image: app:target\n      build:", 1)
	_, err := Load(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected target ambiguity error, got %v", err)
	}
}

func TestLoadRejectsInvalidDurationAndPathShape(t *testing.T) {
	for _, replacement := range []string{"timeout: forever", "from: \"\""} {
		body := validConfig
		if strings.HasPrefix(replacement, "timeout") {
			body = strings.Replace(body, "timeout: 10s", replacement, 1)
		} else {
			body = strings.Replace(body, "from: app:v1", replacement, 1)
		}
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Fatalf("expected error for %s", replacement)
		}
	}
}
