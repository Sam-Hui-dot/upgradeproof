package compose

import (
	"context"
	"strings"
	"testing"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
)

type fakeRunner struct {
	calls   [][]string
	envs    [][]string
	results []command.Result
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args []string, env []string) (command.Result, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	f.envs = append(f.envs, append([]string(nil), env...))
	if len(f.results) == 0 {
		return command.Result{}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestResolveReleaseCapturesEveryServiceImage(t *testing.T) {
	runner := &fakeRunner{results: []command.Result{
		{Stdout: []byte("api-container\n")}, {Stdout: []byte("sha256:api\n")}, {Stdout: []byte("example/api@sha256:digest\n")},
		{Stdout: []byte("migrate-container\n")}, {Stdout: []byte("sha256:migrate\n")}, {},
	}}
	client := New(t.TempDir(), "compose.yml", runner)
	model := Model{Services: map[string]Service{"migrate": {Image: "example/migrate:v2"}, "api": {Image: "example/api:v2"}}}
	got, err := client.ResolveRelease(context.Background(), "upgradeproof-test-run", map[string]string{"APP_TAG": "v2"}, model)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Service != "api" || got[0].Resolved != "example/api@sha256:digest" || got[1].Service != "migrate" || got[1].Resolved != "sha256:migrate" {
		t.Fatalf("unexpected release identities: %#v", got)
	}
	if call := strings.Join(runner.calls[0], " "); !strings.Contains(call, "ps -a -q api") {
		t.Fatalf("one-shot compatible ps command missing: %s", call)
	}
}

func TestApplyConvergesWholeProjectWithReleaseEnvironment(t *testing.T) {
	runner := &fakeRunner{}
	client := New(t.TempDir(), "compose.yml", runner)
	if err := client.Apply(context.Background(), "upgradeproof-test-run", map[string]string{"APP_TAG": "v2"}); err != nil {
		t.Fatal(err)
	}
	call := strings.Join(runner.calls[0], " ")
	if !strings.HasSuffix(call, "up -d --remove-orphans") || strings.Contains(call, "--no-deps") || strings.Contains(call, "--force-recreate") {
		t.Fatalf("unexpected release apply command: %s", call)
	}
	found := false
	for _, item := range runner.envs[0] {
		if item == "APP_TAG=v2" {
			found = true
		}
	}
	if !found {
		t.Fatal("release environment was not overlaid")
	}
}

func TestCleanupOnlyUsesOwnedProject(t *testing.T) {
	runner := &fakeRunner{}
	client := New(t.TempDir(), "compose.yml", runner)
	if err := client.Cleanup(context.Background(), "production", map[string]string{"APP_TAG": "v1"}); err == nil {
		t.Fatal("expected cleanup refusal")
	}
	if len(runner.calls) != 0 {
		t.Fatal("unsafe cleanup reached command runner")
	}
	if err := client.Cleanup(context.Background(), "upgradeproof-repo-path-run", map[string]string{"APP_TAG": "v2"}); err != nil {
		t.Fatal(err)
	}
	call := strings.Join(runner.calls[0], " ")
	if call != "docker compose -f "+client.ComposeFile+" -p upgradeproof-repo-path-run down --volumes --remove-orphans" {
		t.Fatalf("unexpected cleanup command: %s", call)
	}
}

func TestValidateAuditsResolvedAndCanonicalModelForOneState(t *testing.T) {
	resolved := `{"services":{"api":{"image":"api:v2"},"worker":{"image":"worker:v2"}}}`
	canonical := `{"name":"upgradeproof-preflight","services":{"api":{"image":"api:v2"},"worker":{"image":"worker:v2","volumes":[{"type":"bind","source":"/host/data","target":"/data"}]}}}`
	runner := &fakeRunner{results: []command.Result{{Stdout: []byte(resolved)}, {Stdout: []byte(canonical)}}}
	client := New(t.TempDir(), "compose.yml", runner)
	_, err := client.Validate(context.Background(), map[string]string{"APP_TAG": "v2"})
	if err == nil || !strings.Contains(err.Error(), "canonical Compose safety audit") || !strings.Contains(err.Error(), "writable bind") {
		t.Fatalf("canonical unsafe model was not rejected: %v", err)
	}
	if call := strings.Join(runner.calls[0], " "); !strings.Contains(call, "config --format json --no-normalize") {
		t.Fatalf("resolved config command missing: %s", call)
	}
}

func TestRemoveOwnedTargetImageUsesExactRunOwnedTag(t *testing.T) {
	runner := &fakeRunner{}
	client := New(t.TempDir(), "compose.yml", runner)
	if err := client.RemoveOwnedTargetImage(context.Background(), "some-image:latest", "run"); err == nil {
		t.Fatal("expected refusal for non-owned image")
	}
	if len(runner.calls) != 0 {
		t.Fatal("non-owned image reached command runner")
	}
	ref := "example/api:upgradeproof-target-run"
	if err := client.RemoveOwnedTargetImage(context.Background(), ref, "run"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(runner.calls[0], " "); got != "docker image rm "+ref {
		t.Fatalf("unexpected image cleanup command: %s", got)
	}
}
