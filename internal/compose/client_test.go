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

func TestValidateAuditsResolvedComposeModel(t *testing.T) {
	model := `{"services":{"app":{"image":"app:v1","container_name":"fixed"}}}`
	runner := &fakeRunner{results: []command.Result{{Stdout: []byte(model)}}}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	err := client.Validate(context.Background(), "app:v1")
	if err == nil || !strings.Contains(err.Error(), "fixed container_name") {
		t.Fatalf("resolved unsafe model was not rejected: %v", err)
	}
	call := strings.Join(runner.calls[0], " ")
	if !strings.Contains(call, "-p upgradeproof-preflight config --format json --no-normalize") {
		t.Fatalf("resolved config command missing: %s", call)
	}
}

func TestValidateAuditsCanonicalComposeModel(t *testing.T) {
	resolved := `{"services":{"app":{"image":"app:v1"}}}`
	canonical := `{"name":"upgradeproof-preflight","services":{"app":{"image":"app:v1","volumes":[{"type":"bind","source":"/host/data","target":"/data"}]}}}`
	runner := &fakeRunner{results: []command.Result{{Stdout: []byte(resolved)}, {Stdout: []byte(canonical)}}}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	err := client.Validate(context.Background(), "app:v1")
	if err == nil || !strings.Contains(err.Error(), "canonical Compose safety audit") || !strings.Contains(err.Error(), "writable bind") {
		t.Fatalf("canonical unsafe model was not rejected: %v", err)
	}
}

func TestRemoveOwnedTargetImageUsesExactTag(t *testing.T) {
	runner := &fakeRunner{}
	client := New(t.TempDir(), "compose.yml", "app", runner)
	if err := client.RemoveOwnedTargetImage(context.Background(), "some-image:latest", "run"); err == nil {
		t.Fatal("expected refusal for non-owned image")
	}
	if len(runner.calls) != 0 {
		t.Fatal("non-owned image reached command runner")
	}
	if err := client.RemoveOwnedTargetImage(context.Background(), "upgradeproof-target:run", "run"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(runner.calls[0], " "); got != "docker image rm upgradeproof-target:run" {
		t.Fatalf("unexpected image cleanup command: %s", got)
	}
}
