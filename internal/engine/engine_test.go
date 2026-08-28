package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
	"github.com/Sam-Hui-dot/upgradeproof/internal/compose"
	"github.com/Sam-Hui-dot/upgradeproof/internal/config"
	"github.com/Sam-Hui-dot/upgradeproof/internal/health"
	"github.com/Sam-Hui-dot/upgradeproof/internal/hooks"
)

type engineRunner struct {
	mu       sync.Mutex
	upImages []string
	removed  []string
}

func (f *engineRunner) Run(_ context.Context, _ string, name string, args []string, env []string) (command.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	joined := strings.Join(args, " ")
	if name != "docker" {
		if strings.Contains(joined, "verify-command") {
			return command.Result{Stderr: []byte("invariant failed"), ExitCode: 1}, errors.New("exit status 1")
		}
		return command.Result{Stdout: []byte("hook ok"), ExitCode: 0}, nil
	}
	if strings.Contains(joined, " config --format json") {
		return command.Result{Stdout: []byte(`{"services":{"app":{}}}`)}, nil
	}
	if strings.Contains(joined, " up ") {
		for _, item := range env {
			if strings.HasPrefix(item, "UPGRADEPROOF_IMAGE=") {
				f.upImages = append(f.upImages, strings.TrimPrefix(item, "UPGRADEPROOF_IMAGE="))
			}
		}
	}
	if strings.Contains(joined, " ps -q app") {
		return command.Result{Stdout: []byte("container-id\n")}, nil
	}
	if len(args) >= 2 && args[0] == "container" && args[1] == "inspect" {
		return command.Result{Stdout: []byte("sha256:image-id\n")}, nil
	}
	if len(args) >= 2 && args[0] == "image" && args[1] == "inspect" {
		return command.Result{Stdout: []byte("example/app@sha256:digest\n")}, nil
	}
	if len(args) >= 3 && args[0] == "image" && args[1] == "rm" {
		f.removed = append(f.removed, args[2])
		return command.Result{}, nil
	}
	if strings.Contains(joined, " logs ") {
		return command.Result{Stdout: []byte("compose evidence")}, nil
	}
	return command.Result{}, nil
}

func TestMultiHopOrderAndVerificationFailurePropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	d := func(value time.Duration) config.Duration { return config.Duration{Duration: value} }
	cfg := &config.Config{
		Version: 1,
		Compose: config.ComposeConfig{File: "compose.yml", Service: "app", ImageEnv: "UPGRADEPROOF_IMAGE"},
		Paths:   []config.Path{{Name: "multi-hop", From: "app:v1", Via: []string{"app:v2"}, To: config.Target{Build: &config.BuildTarget{Service: "app"}}}},
		Health:  config.HealthConfig{Type: "http", URL: server.URL, Timeout: d(time.Second), Interval: d(time.Millisecond)},
		Seed:    config.Hook{Command: "seed-command", Timeout: d(time.Second)},
		Verify:  config.VerifyConfig{Checks: []config.Check{{Name: "preserved", Hook: config.Hook{Command: "verify-command", Timeout: d(time.Second)}}}},
	}
	runner := &engineRunner{}
	root := t.TempDir()
	executor := Engine{
		Config:  cfg,
		Compose: compose.New(root, filepath.Join(root, "compose.yml"), "app", runner),
		Hooks:   hooks.Runner{CommandRunner: runner, RootDir: root},
		Health:  health.HTTPWaiter{},
		Options: Options{ConfigPath: filepath.Join(root, "upgradeproof.yml"), RootDir: root, ReportDir: filepath.Join(root, "reports"), ToolVersion: "test"},
	}
	result, code := executor.Run(context.Background())
	if code != ExitTestFailed || result.OverallStatus != "failed" {
		t.Fatalf("expected invariant failure code, got code=%d status=%s", code, result.OverallStatus)
	}
	if got := strings.Join(runner.upImages, ","); !strings.HasPrefix(got, "app:v1,app:v2,upgradeproof-target:") {
		t.Fatalf("wrong upgrade order: %s", got)
	}
	path := result.Paths[0]
	if len(path.Checks) != 1 || path.Checks[0].Status != "failed" || path.Checks[0].ExitCode != 1 {
		t.Fatalf("failed invariant not propagated: %+v", path.Checks)
	}
	stages := map[string]string{}
	for _, item := range path.Steps {
		stages[item.Name] = item.Status
	}
	if stages["CLEANUP"] != "passed" || stages["CLEANUP_TARGET_IMAGE"] != "passed" {
		t.Fatalf("cleanup stages missing: %+v", path.Steps)
	}
	if len(runner.removed) != 1 || !strings.HasPrefix(runner.removed[0], "upgradeproof-target:") {
		t.Fatalf("run-owned target image was not removed exactly: %#v", runner.removed)
	}
}

func TestProjectNameGeneration(t *testing.T) {
	name := GenerateProjectName("My Repo!", "V1 → Current", strings.Repeat("a", 80))
	if !compose.IsOwnedProjectName(name) || len(name) > 63 {
		t.Fatalf("unsafe project name: %q", name)
	}
	if name == GenerateProjectName("My Repo!", "other", strings.Repeat("a", 80)) {
		t.Fatal("different paths should not share project names")
	}
	if name == GenerateProjectName("My Repo!", "V1 → Current", strings.Repeat("b", 80)) {
		t.Fatal("different runs should not share project names after truncation")
	}
}

func TestExitCodeSemantics(t *testing.T) {
	if ExitPassed != 0 || ExitTestFailed != 1 || ExitPreflight != 2 || ExitInfrastructure != 3 {
		t.Fatal("exit code contract changed")
	}
}
