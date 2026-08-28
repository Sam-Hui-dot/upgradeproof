package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
)

type Client struct {
	Runner      command.Runner
	RootDir     string
	ComposeFile string
	Service     string
}

type Model struct {
	Services map[string]json.RawMessage `json:"services"`
}

func (c Client) Validate(ctx context.Context, image string) error {
	result, err := c.run(ctx, image, "config", "--format", "json")
	if err != nil {
		return fmt.Errorf("docker compose config failed: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	var model Model
	if err := json.Unmarshal(result.Stdout, &model); err != nil {
		return fmt.Errorf("decode docker compose config JSON: %w", err)
	}
	if _, ok := model.Services[c.Service]; !ok {
		return fmt.Errorf("compose service %q does not exist after interpolation", c.Service)
	}
	return nil
}

func (c Client) Build(ctx context.Context, project, imageTag string) error {
	result, err := c.runProject(ctx, project, imageTag, "build", c.Service)
	if err != nil {
		return fmt.Errorf("build target: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (c Client) UpFrom(ctx context.Context, project, image string) error {
	result, err := c.runProject(ctx, project, image, "up", "-d", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("start source: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (c Client) Upgrade(ctx context.Context, project, image string) error {
	result, err := c.runProject(ctx, project, image, "up", "-d", "--no-deps", "--force-recreate", c.Service)
	if err != nil {
		return fmt.Errorf("recreate service %q: %w: %s", c.Service, err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (c Client) Logs(ctx context.Context, project, image string) ([]byte, error) {
	result, err := c.runProject(ctx, project, image, "logs", "--no-color", "--timestamps")
	if err != nil {
		return append(result.Stdout, result.Stderr...), err
	}
	return result.Stdout, nil
}

func (c Client) ResolveImage(ctx context.Context, project, requested string) (string, error) {
	ps, err := c.runProject(ctx, project, requested, "ps", "-q", c.Service)
	if err != nil {
		return "", fmt.Errorf("locate service container: %w", err)
	}
	containerID := strings.TrimSpace(string(ps.Stdout))
	if containerID == "" {
		return "", errors.New("compose returned no service container ID")
	}
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

func (c Client) Cleanup(ctx context.Context, project, image string) error {
	if !IsOwnedProjectName(project) {
		return fmt.Errorf("refusing cleanup for non-UpgradeProof project %q", project)
	}
	result, err := c.runProject(ctx, project, image, "down", "--volumes", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("project-scoped cleanup: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
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

func (c Client) run(ctx context.Context, image string, args ...string) (command.Result, error) {
	base := []string{"compose", "-f", c.ComposeFile}
	base = append(base, args...)
	return c.Runner.Run(ctx, c.RootDir, "docker", base, withImage(os.Environ(), image))
}

func (c Client) runProject(ctx context.Context, project, image string, args ...string) (command.Result, error) {
	base := []string{"compose", "-f", c.ComposeFile, "-p", project}
	base = append(base, args...)
	return c.Runner.Run(ctx, c.RootDir, "docker", base, withImage(os.Environ(), image))
}

func withImage(env []string, image string) []string {
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(strings.ToUpper(item), "UPGRADEPROOF_IMAGE=") {
			result = append(result, item)
		}
	}
	return append(result, "UPGRADEPROOF_IMAGE="+image)
}

func New(root, composeFile, service string, runner command.Runner) Client {
	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(root, composeFile)
	}
	return Client{Runner: runner, RootDir: root, ComposeFile: composeFile, Service: service}
}
