package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
	"github.com/Sam-Hui-dot/upgradeproof/internal/safety"
)

type Client struct {
	Runner      command.Runner
	RootDir     string
	ComposeFile string
}

type Service struct {
	Image string `json:"image"`
}

type Model struct {
	Services map[string]Service `json:"services"`
}

type ServiceImage struct {
	Service   string
	Container string
	Requested string
	Resolved  string
}

// Validate resolves and audits one complete Compose release state. It must be
// called for every from/via/to environment, not only for the first source.
func (c Client) Validate(ctx context.Context, env map[string]string) (Model, error) {
	resolved, err := c.runProject(ctx, "upgradeproof-preflight", env, "config", "--format", "json", "--no-normalize")
	if err != nil {
		return Model{}, fmt.Errorf("docker compose resolved config failed: %w: %s", err, strings.TrimSpace(string(resolved.Stderr)))
	}
	if err := safety.CheckResolvedCompose(resolved.Stdout); err != nil {
		return Model{}, fmt.Errorf("resolved Compose safety audit: %w", err)
	}
	result, err := c.runProject(ctx, "upgradeproof-preflight", env, "config", "--format", "json")
	if err != nil {
		return Model{}, fmt.Errorf("docker compose canonical config failed: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	var model Model
	if err := json.Unmarshal(result.Stdout, &model); err != nil {
		return Model{}, fmt.Errorf("decode docker compose config JSON: %w", err)
	}
	if len(model.Services) == 0 {
		return Model{}, errors.New("resolved Compose model has no services")
	}
	if err := safety.CheckCanonicalCompose(result.Stdout); err != nil {
		return Model{}, fmt.Errorf("canonical Compose safety audit: %w", err)
	}
	return model, nil
}

func (c Client) Build(ctx context.Context, project string, env map[string]string, services []string) error {
	args := []string{"build"}
	args = append(args, services...)
	result, err := c.runProject(ctx, project, env, args...)
	if err != nil {
		return fmt.Errorf("build target release services: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

// Apply asks Compose to converge the whole project on a release state. Compose
// owns dependency ordering, service_completed_successfully, and change-based
// recreation; UpgradeProof does not synthesize a dependency graph.
func (c Client) Apply(ctx context.Context, project string, env map[string]string) error {
	result, err := c.runProject(ctx, project, env, "up", "-d", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("apply Compose release state: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (c Client) Logs(ctx context.Context, project string, env map[string]string) ([]byte, error) {
	result, err := c.runProject(ctx, project, env, "logs", "--no-color", "--timestamps")
	if err != nil {
		return append(result.Stdout, result.Stderr...), err
	}
	return result.Stdout, nil
}

func (c Client) ResolveRelease(ctx context.Context, project string, env map[string]string, model Model) ([]ServiceImage, error) {
	names := make([]string, 0, len(model.Services))
	for name := range model.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	var identities []ServiceImage
	for _, serviceName := range names {
		ps, err := c.runProject(ctx, project, env, "ps", "-a", "-q", serviceName)
		if err != nil {
			return nil, fmt.Errorf("locate service %q container: %w", serviceName, err)
		}
		containerIDs := strings.Fields(string(ps.Stdout))
		if len(containerIDs) == 0 {
			// A resolved model can contain profile-gated services that were not
			// materialized by this release state. Evidence covers actual project
			// containers, including exited one-shot services via `ps -a`.
			continue
		}
		for _, containerID := range containerIDs {
			resolved, err := c.resolveContainerImage(ctx, containerID)
			if err != nil {
				return nil, fmt.Errorf("resolve service %q image: %w", serviceName, err)
			}
			identities = append(identities, ServiceImage{Service: serviceName, Container: containerID, Requested: model.Services[serviceName].Image, Resolved: resolved})
		}
	}
	if len(identities) == 0 {
		return nil, errors.New("compose project has no materialized service containers")
	}
	return identities, nil
}

func (c Client) resolveContainerImage(ctx context.Context, containerID string) (string, error) {
	inspect, err := c.Runner.Run(ctx, c.RootDir, "docker", []string{"container", "inspect", "--format", "{{.Image}}", containerID}, os.Environ())
	if err != nil {
		return "", fmt.Errorf("inspect service container image: %w", err)
	}
	imageID := strings.TrimSpace(string(inspect.Stdout))
	if imageID == "" {
		return "", errors.New("container image identity is empty")
	}
	repo, repoErr := c.Runner.Run(ctx, c.RootDir, "docker", []string{"image", "inspect", "--format", "{{join .RepoDigests \",\"}}", imageID}, os.Environ())
	if repoErr == nil && strings.TrimSpace(string(repo.Stdout)) != "" {
		return strings.TrimSpace(string(repo.Stdout)), nil
	}
	return imageID, nil
}

func (c Client) Cleanup(ctx context.Context, project string, env map[string]string) error {
	if !IsOwnedProjectName(project) {
		return fmt.Errorf("refusing cleanup for non-UpgradeProof project %q", project)
	}
	result, err := c.runProject(ctx, project, env, "down", "--volumes", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("project-scoped cleanup: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (c Client) RemoveOwnedTargetImage(ctx context.Context, imageRef, runID string) error {
	suffix := ":upgradeproof-target-" + runID
	if runID == "" || !strings.HasSuffix(imageRef, suffix) {
		return fmt.Errorf("refusing image removal for non-owned target %q", imageRef)
	}
	result, err := c.Runner.Run(ctx, c.RootDir, "docker", []string{"image", "rm", imageRef}, os.Environ())
	if err != nil {
		return fmt.Errorf("remove run-owned target image %q: %w: %s", imageRef, err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func IsOwnedProjectName(project string) bool {
	if !strings.HasPrefix(project, "upgradeproof-") || len(project) > 63 {
		return false
	}
	for _, r := range project {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func (c Client) runProject(ctx context.Context, project string, env map[string]string, args ...string) (command.Result, error) {
	base := []string{"compose", "-f", c.ComposeFile, "-p", project}
	base = append(base, args...)
	return c.Runner.Run(ctx, c.RootDir, "docker", base, withEnvironment(os.Environ(), env))
}

func withEnvironment(base []string, overlay map[string]string) []string {
	keys := make(map[string]bool, len(overlay))
	for key := range overlay {
		keys[strings.ToUpper(key)] = true
	}
	result := make([]string, 0, len(base)+len(overlay))
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if ok && keys[strings.ToUpper(key)] {
			continue
		}
		result = append(result, item)
	}
	names := make([]string, 0, len(overlay))
	for key := range overlay {
		names = append(names, key)
	}
	sort.Strings(names)
	for _, key := range names {
		result = append(result, key+"="+overlay[key])
	}
	return result
}

func New(root, composeFile string, runner command.Runner) Client {
	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(root, composeFile)
	}
	return Client{Runner: runner, RootDir: root, ComposeFile: composeFile}
}
