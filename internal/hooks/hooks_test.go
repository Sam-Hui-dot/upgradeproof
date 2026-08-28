package hooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
)

type hookFake struct{ block bool }

func (f hookFake) Run(ctx context.Context, _ string, _ string, _ []string, env []string) (command.Result, error) {
	if len(env) != 9 {
		return command.Result{}, errors.New("unexpected environment exposure")
	}
	if f.block {
		<-ctx.Done()
		return command.Result{Stdout: []byte("before-timeout"), Stderr: []byte("timed")}, ctx.Err()
	}
	return command.Result{Stdout: []byte("out"), Stderr: []byte("err"), ExitCode: 0}, nil
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
