package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
)

type hookFake struct {
	block bool
	env   *[]string
}

func (f hookFake) Run(ctx context.Context, _ string, _ string, _ []string, env []string) (command.Result, error) {
	if f.env != nil {
		*f.env = append([]string(nil), env...)
	}
	if f.block {
		<-ctx.Done()
		return command.Result{Stdout: []byte("before-timeout"), Stderr: []byte("timed")}, ctx.Err()
	}
	return command.Result{Stdout: []byte("out"), Stderr: []byte("err"), ExitCode: 0}, nil
}

func environmentMap(items []string) map[string]string {
	result := map[string]string{}
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[strings.ToUpper(key)] = value
		}
	}
	return result
}

func TestHookCapturesStdoutAndStderr(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr")
	result := (Runner{CommandRunner: hookFake{}, RootDir: dir}).Run(context.Background(), "seed", "ignored", time.Second, Environment{}, stdout, stderr)
	if result.Status != "passed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	gotOut, _ := os.ReadFile(stdout)
	gotErr, _ := os.ReadFile(stderr)
	if string(gotOut) != "out" || string(gotErr) != "err" {
		t.Fatalf("capture mismatch: %q %q", gotOut, gotErr)
	}
}

func TestHookTimeout(t *testing.T) {
	dir := t.TempDir()
	result := (Runner{CommandRunner: hookFake{block: true}, RootDir: dir}).Run(context.Background(), "seed", "ignored", 10*time.Millisecond, Environment{}, filepath.Join(dir, "out"), filepath.Join(dir, "err"))
	if result.Status != "failed" || result.Error == "" {
		t.Fatalf("expected timeout failure: %+v", result)
	}
}

func TestHookInheritsEnvironmentAndOverlaysControlledValues(t *testing.T) {
	t.Setenv("HOOK_TEST_ORDINARY", "ordinary-value")
	t.Setenv("PATH", "inherited-path")
	t.Setenv("HOME", "inherited-home")
	t.Setenv("UPGRADEPROOF_PROJECT", "forged-project")
	for _, phase := range []string{"seed", "verify"} {
		t.Run(phase, func(t *testing.T) {
			var captured []string
			dir := t.TempDir()
			env := Environment{Project: "owned-project", RunID: "run", Phase: phase}
			result := (Runner{CommandRunner: hookFake{env: &captured}, RootDir: dir}).Run(context.Background(), phase, "ignored", time.Second, env, filepath.Join(dir, "out"), filepath.Join(dir, "err"))
			if result.Status != "passed" {
				t.Fatalf("unexpected hook result: %+v", result)
			}
			got := environmentMap(captured)
			if got["HOOK_TEST_ORDINARY"] != "ordinary-value" || got["PATH"] != "inherited-path" || got["HOME"] != "inherited-home" {
				t.Fatalf("ordinary environment not inherited: %#v", got)
			}
			if got["UPGRADEPROOF_PROJECT"] != "owned-project" {
				t.Fatalf("forged project was not overwritten: %q", got["UPGRADEPROOF_PROJECT"])
			}
			if got["UPGRADEPROOF_PHASE"] != phase {
				t.Fatalf("tool phase overlay mismatch: %q", got["UPGRADEPROOF_PHASE"])
			}
			projectEntries := 0
			for _, item := range captured {
				key, _, ok := strings.Cut(item, "=")
				if ok && strings.EqualFold(key, "UPGRADEPROOF_PROJECT") {
					projectEntries++
				}
			}
			if projectEntries != 1 {
				t.Fatalf("expected exactly one tool-owned project entry, got %d", projectEntries)
			}
		})
	}
}
