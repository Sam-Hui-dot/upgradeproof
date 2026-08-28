package engine

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/Sam-Hui-dot/upgradeproof/internal/report"
)

type engineRunner struct {
	mu          sync.Mutex
	upTags      []string
	removed     []string
	unsafeTag   string
	configCalls map[string]int
}

func envValue(env []string, key string) string {
	for _, item := range env {
		if strings.HasPrefix(item, key+"=") {
			return strings.TrimPrefix(item, key+"=")
		}
	}
	return ""
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
	tag := envValue(env, "APP_TAG")
	if strings.Contains(joined, " config --format json") {
		if f.configCalls == nil {
			f.configCalls = map[string]int{}
		}
		f.configCalls[tag]++
		mount := ""
		if tag == f.unsafeTag {
			mount = `,"volumes":[{"type":"bind","source":"/host/data","target":"/data"}]`
		}
		return command.Result{Stdout: []byte(fmt.Sprintf(`{"name":"upgradeproof-preflight","services":{"app":{"image":"example/app:%s"%s}}}`, tag, mount))}, nil
	}
	if strings.Contains(joined, " up ") {
		f.upTags = append(f.upTags, tag)
	}
	if strings.Contains(joined, " ps -a -q app") {
		return command.Result{Stdout: []byte("container-id\n")}, nil
	}
	if len(args) >= 2 && args[0] == "container" && args[1] == "inspect" {
		return command.Result{Stdout: []byte("sha256:image-id\n")}, nil
	}
	if len(args) >= 2 && args[0] == "image" && args[1] == "inspect" {
		return command.Result{Stdout: []byte("example/app@sha256:digest\n")}, nil
	}
	if len(args) >= 3 && args[0] == "image" && args[1] == "rm" {
		for _, removed := range f.removed {
			if removed == args[2] {
				return command.Result{Stderr: []byte("image already removed"), ExitCode: 1}, errors.New("exit status 1")
			}
		}
		f.removed = append(f.removed, args[2])
		return command.Result{}, nil
	}
	if strings.Contains(joined, " logs ") {
		return command.Result{Stdout: []byte("compose evidence")}, nil
	}
	return command.Result{}, nil
}

func releaseConfig(serverURL string) *config.Config {
	d := func(value time.Duration) config.Duration { return config.Duration{Duration: value} }
	return &config.Config{
		Version: 2,
		Compose: config.ComposeConfig{File: "compose.yml"},
		Paths: []config.Path{{Name: "multi-hop",
			From: config.ReleaseState{Env: map[string]string{"APP_TAG": "v1"}},
			Via:  []config.ReleaseState{{Env: map[string]string{"APP_TAG": "v2"}}},
			To: config.ReleaseState{Env: map[string]string{"APP_TAG": "current"}, Build: &config.BuildTarget{
				Services: []string{"app"}, TagEnv: "APP_TAG",
			}},
		}},
		Health: config.HealthConfig{Type: "http", URL: serverURL, Timeout: d(time.Second), Interval: d(time.Millisecond)},
		Seed:   config.Hook{Command: "seed-command", Timeout: d(time.Second)},
		Verify: config.VerifyConfig{Checks: []config.Check{{Name: "preserved", Hook: config.Hook{Command: "verify-command", Timeout: d(time.Second)}}}},
	}
}

func TestReleaseStateOrderAndVerificationFailurePropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	runner := &engineRunner{}
	root := t.TempDir()
	executor := Engine{
		Config:  releaseConfig(server.URL),
		Compose: compose.New(root, filepath.Join(root, "compose.yml"), runner),
		Hooks:   hooks.Runner{CommandRunner: runner, RootDir: root},
		Health:  health.HTTPWaiter{},
		Options: Options{ConfigPath: filepath.Join(root, "upgradeproof.yml"), RootDir: root, ReportDir: filepath.Join(root, "reports"), ToolVersion: "test"},
	}
	result, code := executor.Run(context.Background())
	if code != ExitTestFailed || result.OverallStatus != "failed" {
		t.Fatalf("expected invariant failure code, got code=%d status=%s", code, result.OverallStatus)
	}
	if len(runner.upTags) != 3 || runner.upTags[0] != "v1" || runner.upTags[1] != "v2" || !strings.HasPrefix(runner.upTags[2], "upgradeproof-target-") {
		t.Fatalf("wrong release order: %#v", runner.upTags)
	}
	path := result.Paths[0]
	if len(path.ReleaseStates) != 3 || path.ReleaseStates[1].Step != "via-1" || path.ReleaseStates[1].Services[0].Service != "app" {
		t.Fatalf("multi-service release evidence missing: %+v", path.ReleaseStates)
	}
	if len(path.Checks) != 1 || path.Checks[0].Status != "failed" || path.Checks[0].ExitCode != 1 {
		t.Fatalf("failed invariant not propagated: %+v", path.Checks)
	}
	if len(runner.removed) != 1 || !strings.Contains(runner.removed[0], ":upgradeproof-target-") {
		t.Fatalf("run-owned target image was not removed exactly: %#v", runner.removed)
	}
}

func TestEveryReleaseStateIsResolvedAndSafetyAudited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	runner := &engineRunner{unsafeTag: "v2"}
	root := t.TempDir()
	executor := Engine{Config: releaseConfig(server.URL), Compose: compose.New(root, "compose.yml", runner), Hooks: hooks.Runner{CommandRunner: runner, RootDir: root}, Health: health.HTTPWaiter{}, Options: Options{RootDir: root, ReportDir: filepath.Join(root, "reports")}}
	result, code := executor.Run(context.Background())
	if code != ExitPreflight || result.OverallStatus != "preflight_failed" || result.Paths[0].Status != "preflight_failed" {
		t.Fatalf("unsafe intermediate state not rejected: code=%d report=%+v", code, result)
	}
	if runner.configCalls["v1"] != 2 || runner.configCalls["v2"] != 1 || len(runner.upTags) != 0 {
		t.Fatalf("unexpected state audit/execution calls: configs=%#v up=%#v", runner.configCalls, runner.upTags)
	}
}

func TestAllSelectedPathsAreAuditedBeforeAnyProjectStarts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg := releaseConfig(server.URL)
	unsafe := cfg.Paths[0]
	unsafe.Name = "later-unsafe"
	unsafe.From = config.ReleaseState{Env: map[string]string{"APP_TAG": "unsafe"}}
	unsafe.Via = nil
	unsafe.To = config.ReleaseState{Env: map[string]string{"APP_TAG": "v3"}}
	cfg.Paths = append(cfg.Paths, unsafe)
	runner := &engineRunner{unsafeTag: "unsafe"}
	root := t.TempDir()
	executor := Engine{Config: cfg, Compose: compose.New(root, "compose.yml", runner), Hooks: hooks.Runner{CommandRunner: runner, RootDir: root}, Health: health.HTTPWaiter{}, Options: Options{RootDir: root, ReportDir: filepath.Join(root, "reports")}}
	_, code := executor.Run(context.Background())
	if code != ExitPreflight || len(runner.upTags) != 0 {
		t.Fatalf("a project started before all paths passed safety: code=%d up=%#v", code, runner.upTags)
	}
}

func TestProjectNameGeneration(t *testing.T) {
	name := GenerateProjectName("My Repo!", "V1 → Current", strings.Repeat("a", 80))
	if !compose.IsOwnedProjectName(name) || len(name) > 63 {
		t.Fatalf("unsafe project name: %q", name)
	}
	if name == GenerateProjectName("My Repo!", "other", strings.Repeat("a", 80)) || name == GenerateProjectName("My Repo!", "V1 → Current", strings.Repeat("b", 80)) {
		t.Fatal("distinct paths/runs should not share project names")
	}
}

func TestExitCodeSemantics(t *testing.T) {
	if ExitPassed != 0 || ExitTestFailed != 1 || ExitPreflight != 2 || ExitInfrastructure != 3 {
		t.Fatal("exit code contract changed")
	}
}

func TestInvalidTargetBuildIsAConfigurationPreflightFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg := releaseConfig(server.URL)
	cfg.Paths[0].To.Build.Services = []string{"missing-service"}
	runner := &engineRunner{}
	root := t.TempDir()
	executor := Engine{Config: cfg, Compose: compose.New(root, "compose.yml", runner), Hooks: hooks.Runner{CommandRunner: runner, RootDir: root}, Health: health.HTTPWaiter{}, Options: Options{RootDir: root, ReportDir: filepath.Join(root, "reports")}}
	result, code := executor.Run(context.Background())
	if code != ExitPreflight || result.OverallStatus != "preflight_failed" || len(runner.upTags) != 0 {
		t.Fatalf("invalid target build was not rejected as config preflight: code=%d report=%+v up=%#v", code, result, runner.upTags)
	}
}

func TestOwnedTargetImageCleanupReferencesAreDeduplicated(t *testing.T) {
	got := uniqueStrings([]string{"example/shared:upgradeproof-target-run", "example/shared:upgradeproof-target-run", "example/api:upgradeproof-target-run"})
	if len(got) != 2 || got[0] != "example/shared:upgradeproof-target-run" || got[1] != "example/api:upgradeproof-target-run" {
		t.Fatalf("unexpected unique cleanup references: %#v", got)
	}
}

func TestFinishPathRemovesSharedOwnedTargetImageOnce(t *testing.T) {
	runner := &engineRunner{}
	root := t.TempDir()
	executor := Engine{Compose: compose.New(root, filepath.Join(root, "compose.yml"), runner)}
	ref := "example/shared:upgradeproof-target-run"
	result, code := executor.finishPath(context.Background(), "run", report.PathResult{
		Status:            "passed",
		ProjectName:       "upgradeproof-shared-image-run",
		ArtifactDirectory: root,
	}, ExitPassed, releaseStep{State: config.ReleaseState{Env: map[string]string{"APP_TAG": "upgradeproof-target-run"}}}, []string{ref, ref})
	if code != ExitPassed || result.Status != "passed" {
		t.Fatalf("duplicate cleanup changed successful result: code=%d status=%s", code, result.Status)
	}
	if len(runner.removed) != 1 || runner.removed[0] != ref {
		t.Fatalf("shared run-owned image was not removed exactly once: %#v", runner.removed)
	}
	cleanupStages := 0
	for _, step := range result.Steps {
		if step.Name == "CLEANUP_TARGET_IMAGE" {
			cleanupStages++
		}
	}
	if cleanupStages != 1 {
		t.Fatalf("expected one target image cleanup stage, got %d", cleanupStages)
	}
}
