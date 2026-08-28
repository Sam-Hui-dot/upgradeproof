package compose

import (
	"context"
	"strings"
	"testing"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
)

type fakeRunner struct {
	calls   [][]string
	results []command.Result
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args []string, _ []string) (command.Result, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if len(f.results) == 0 {
		return command.Result{}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestResolveImagePrefersRepoDigest(t *testing.T) {
	runner := &fakeRunner{results: []command.Result{{Stdout: []byte("container\n")}, {Stdout: []byte("sha256:id\n")}, {Stdout: []byte("example/app@sha256:digest\n")}}}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	got, err := client.ResolveImage(context.Background(), "upgradeproof-test-run", "app:v1")
	if err != nil || got != "example/app@sha256:digest" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCleanupOnlyUsesOwnedProject(t *testing.T) {
	runner := &fakeRunner{}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	if err := client.Cleanup(context.Background(), "production", "app:v1"); err == nil {
		t.Fatal("expected cleanup refusal")
	}
	if len(runner.calls) != 0 {
		t.Fatal("unsafe cleanup reached command runner")
	}
	if err := client.Cleanup(context.Background(), "upgradeproof-repo-path-run", "app:v1"); err != nil {
		t.Fatal(err)
	}
	call := strings.Join(runner.calls[0], " ")
	if call != "docker compose -f "+client.ComposeFile+" -p upgradeproof-repo-path-run down --volumes --remove-orphans" {
		t.Fatalf("unexpected cleanup command: %s", call)
	}
}

func TestResolveImageFallsBackToImageID(t *testing.T) {
	runner := &fakeRunner{results: []command.Result{{Stdout: []byte("container")}, {Stdout: []byte("sha256:id")}, {}}}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	got, err := client.ResolveImage(context.Background(), "upgradeproof-test-run", "app:local")
	if err != nil || got != "sha256:id" {
		t.Fatalf("got %q, %v", got, err)
	}
}
