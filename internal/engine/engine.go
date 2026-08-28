package engine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sam-Hui-dot/upgradeproof/internal/compose"
	"github.com/Sam-Hui-dot/upgradeproof/internal/config"
	"github.com/Sam-Hui-dot/upgradeproof/internal/health"
	"github.com/Sam-Hui-dot/upgradeproof/internal/hooks"
	"github.com/Sam-Hui-dot/upgradeproof/internal/report"
	"github.com/Sam-Hui-dot/upgradeproof/internal/safety"
)

const (
	ExitPassed         = 0
	ExitTestFailed     = 1
	ExitPreflight      = 2
	ExitInfrastructure = 3
)

type Options struct {
	ConfigPath    string
	RootDir       string
	ReportDir     string
	SelectedPath  string
	KeepOnFailure bool
	ToolVersion   string
}

type Engine struct {
	Config  *config.Config
	Compose compose.Client
	Hooks   hooks.Runner
	Health  health.HTTPWaiter
	Options Options
}

type releaseStep struct {
	Name  string
	State config.ReleaseState
	Model compose.Model
}

func (e *Engine) Run(ctx context.Context) (report.Report, int) {
	run := report.Report{ToolVersion: e.Options.ToolVersion, RunID: newRunID(), StartedAt: time.Now().UTC(), ConfigPath: e.Options.ConfigPath, OverallStatus: "passed"}
	paths, err := e.Config.SelectPath(e.Options.SelectedPath)
	if err != nil {
		run.OverallStatus = "preflight_failed"
		run.FinishedAt = time.Now().UTC()
		return run, ExitPreflight
	}
	prepared := make([][]releaseStep, len(paths))
	for i, path := range paths {
		steps := materializeSteps(path, run.RunID)
		for j := range steps {
			model, validateErr := e.Compose.Validate(ctx, steps[j].State.Env)
			if validateErr != nil {
				err = fmt.Errorf("path %q release state %s: %w", path.Name, steps[j].Name, validateErr)
				break
			}
			steps[j].Model = model
		}
		if err == nil {
			err = validateBuildOwnership(steps[len(steps)-1], run.RunID)
		}
		if err != nil {
			project := GenerateProjectName(filepath.Base(e.Options.RootDir), path.Name, run.RunID)
			failed := report.PathResult{Name: path.Name, ProjectName: project, ArtifactDirectory: e.pathArtifactDir(run.RunID, path.Name)}
			stage(&failed, "PREFLIGHT", func() error { return err })
			code := ExitInfrastructure
			failed.Status = "infrastructure_failed"
			run.OverallStatus = "infrastructure_failed"
			if safety.IsViolation(err) {
				code = ExitPreflight
				failed.Status = "preflight_failed"
				run.OverallStatus = "preflight_failed"
			}
			run.Paths = append(run.Paths, failed)
			run.FinishedAt = time.Now().UTC()
			e.writeReports(run)
			return run, code
		}
		prepared[i] = steps
	}

	exitCode := ExitPassed
	for i, path := range paths {
		result, code := e.runPath(ctx, run.RunID, path, prepared[i])
		run.Paths = append(run.Paths, result)
		if code > exitCode {
			exitCode = code
		}
	}
	run.FinishedAt = time.Now().UTC()
	switch exitCode {
	case ExitPassed:
		run.OverallStatus = "passed"
	case ExitTestFailed:
		run.OverallStatus = "failed"
	case ExitPreflight:
		run.OverallStatus = "preflight_failed"
	default:
		if ctx.Err() != nil {
			run.OverallStatus = "interrupted"
		} else {
			run.OverallStatus = "infrastructure_failed"
		}
	}
	if err := e.writeReports(run); err != nil && exitCode < ExitInfrastructure {
		exitCode = ExitInfrastructure
		run.OverallStatus = "infrastructure_failed"
	}
	return run, exitCode
}

func (e *Engine) runPath(ctx context.Context, runID string, path config.Path, steps []releaseStep) (result report.PathResult, exitCode int) {
	artifactDir := e.pathArtifactDir(runID, path.Name)
	project := GenerateProjectName(filepath.Base(e.Options.RootDir), path.Name, runID)
	result = report.PathResult{Name: path.Name, Status: "passed", ProjectName: project, ArtifactDirectory: artifactDir}
	stage(&result, "PREFLIGHT", func() error { return nil })
	stage(&result, "RESOLVE_RELEASE_STATES", func() error { return nil })
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		stage(&result, "PREPARE_PROJECT", func() error { return err })
		result.Status = "infrastructure_failed"
		return result, ExitInfrastructure
	}
	stage(&result, "PREPARE_PROJECT", func() error { return nil })

	current := steps[0]
	var ownedImages []string
	if err := stage(&result, "APPLY_FROM", func() error { return e.Compose.Apply(ctx, project, current.State.Env) }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
	}
	if err := e.waitAndCapture(ctx, &result, current, "FROM", "compose-source.log"); err != nil {
		return e.finishPath(ctx, runID, result, ExitTestFailed, current, ownedImages)
	}
	if err := stage(&result, "RESOLVE_FROM_RELEASE", func() error { return e.resolve(ctx, &result, project, current) }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
	}

	seedEnv := e.hookEnvironment(runID, project, "seed", path.Name, current, artifactDir)
	var seed report.CheckResult
	if err := stage(&result, "SEED", func() error {
		seed = e.Hooks.Run(ctx, "seed", e.Config.Seed.Command, e.Config.Seed.Timeout.Duration, seedEnv, filepath.Join(artifactDir, "seed.stdout"), filepath.Join(artifactDir, "seed.stderr"))
		result.Hooks = append(result.Hooks, seed)
		if seed.Status != "passed" {
			return errors.New(seed.Error)
		}
		return nil
	}); err != nil {
		result.Status = "failed"
		return e.finishPath(ctx, runID, result, ExitTestFailed, current, ownedImages)
	}
	if err := stage(&result, "CAPTURE_SEEDED_STATE", func() error { return e.captureLog(ctx, &result, project, current, "compose-seeded.log") }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
	}

	for i := 1; i < len(steps); i++ {
		current = steps[i]
		isTarget := i == len(steps)-1
		if isTarget && current.State.Build != nil {
			if err := stage(&result, "BUILD_TARGET_RELEASE", func() error {
				return e.Compose.Build(ctx, project, current.State.Env, current.State.Build.Services)
			}); err != nil {
				result.Status = "infrastructure_failed"
				return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
			}
			for _, serviceName := range current.State.Build.Services {
				ownedImages = append(ownedImages, current.Model.Services[serviceName].Image)
			}
		}
		applyName := "APPLY_TARGET"
		waitLabel := "TARGET"
		filename := "compose-target.log"
		resolveName := "RESOLVE_TARGET_RELEASE"
		if !isTarget {
			applyName = fmt.Sprintf("APPLY_STEP_%d", i)
			waitLabel = fmt.Sprintf("STEP_%d", i)
			filename = fmt.Sprintf("compose-step-%d.log", i)
			resolveName = fmt.Sprintf("RESOLVE_STEP_%d_RELEASE", i)
		}
		if err := stage(&result, applyName, func() error { return e.Compose.Apply(ctx, project, current.State.Env) }); err != nil {
			result.Status = "infrastructure_failed"
			return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
		}
		if err := e.waitAndCapture(ctx, &result, current, waitLabel, filename); err != nil {
			return e.finishPath(ctx, runID, result, ExitTestFailed, current, ownedImages)
		}
		if err := stage(&result, resolveName, func() error { return e.resolve(ctx, &result, project, current) }); err != nil {
			result.Status = "infrastructure_failed"
			return e.finishPath(ctx, runID, result, ExitInfrastructure, current, ownedImages)
		}
	}

	verifyFailed := false
	stage(&result, "VERIFY", func() error {
		for _, check := range e.Config.Verify.Checks {
			name := hooks.SafeName(check.Name)
			hookEnv := e.hookEnvironment(runID, project, "verify", path.Name, current, artifactDir)
			checkResult := e.Hooks.Run(ctx, check.Name, check.Command, check.Timeout.Duration, hookEnv, filepath.Join(artifactDir, "verify-"+name+".stdout"), filepath.Join(artifactDir, "verify-"+name+".stderr"))
			result.Checks = append(result.Checks, checkResult)
			if checkResult.Status != "passed" {
				verifyFailed = true
			}
		}
		if verifyFailed {
			return errors.New("one or more invariants failed")
		}
		return nil
	})
	if verifyFailed {
		result.Status = "failed"
		exitCode = ExitTestFailed
	}
	return e.finishPath(ctx, runID, result, exitCode, current, ownedImages)
}

func (e *Engine) waitAndCapture(ctx context.Context, result *report.PathResult, step releaseStep, label, filename string) error {
	if err := stage(result, "WAIT_"+label+"_HEALTH", func() error {
		return e.Health.Wait(ctx, e.Config.Health.URL, e.Config.Health.Timeout.Duration, e.Config.Health.Interval.Duration)
	}); err != nil {
		result.Status = "failed"
		e.captureLog(ctx, result, result.ProjectName, step, filename)
		return err
	}
	if err := stage(result, "CAPTURE_"+label+"_EVIDENCE", func() error {
		return e.captureLog(ctx, result, result.ProjectName, step, filename)
	}); err != nil {
		result.Status = "infrastructure_failed"
		return err
	}
	return nil
}

func (e *Engine) finishPath(ctx context.Context, runID string, result report.PathResult, code int, current releaseStep, ownedImages []string) (report.PathResult, int) {
	if ctx.Err() != nil {
		result.Status = "interrupted"
		code = ExitInfrastructure
	}
	evidenceCtx, evidenceCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer evidenceCancel()
	if err := stage(&result, "CAPTURE_EVIDENCE", func() error {
		return e.captureLog(evidenceCtx, &result, result.ProjectName, current, "compose-final.log")
	}); err != nil && code < ExitInfrastructure {
		code = ExitInfrastructure
		result.Status = "infrastructure_failed"
	}
	stage(&result, "REPORT", func() error { return nil })
	if code != ExitPassed && e.Options.KeepOnFailure && ctx.Err() == nil {
		addSkippedStage(&result, "CLEANUP", "kept because --keep-on-failure was set")
		if len(ownedImages) > 0 {
			addSkippedStage(&result, "CLEANUP_TARGET_IMAGES", "kept with failed Compose project because --keep-on-failure was set")
		}
		return result, code
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := stage(&result, "CLEANUP", func() error { return e.Compose.Cleanup(cleanupCtx, result.ProjectName, current.State.Env) }); err != nil {
		result.Status = "infrastructure_failed"
		return result, ExitInfrastructure
	}
	for _, imageRef := range ownedImages {
		ref := imageRef
		if err := stage(&result, "CLEANUP_TARGET_IMAGE", func() error { return e.Compose.RemoveOwnedTargetImage(cleanupCtx, ref, runID) }); err != nil {
			result.Status = "infrastructure_failed"
			code = ExitInfrastructure
		}
	}
	return result, code
}

func (e *Engine) resolve(ctx context.Context, result *report.PathResult, project string, step releaseStep) error {
	identities, err := e.Compose.ResolveRelease(ctx, project, step.State.Env, step.Model)
	if err != nil {
		return err
	}
	release := report.ReleaseIdentity{Step: step.Name}
	for _, identity := range identities {
		release.Services = append(release.Services, report.ImageIdentity{Service: identity.Service, Container: identity.Container, Requested: identity.Requested, Resolved: identity.Resolved})
	}
	result.ReleaseStates = append(result.ReleaseStates, release)
	return nil
}

func (e *Engine) captureLog(ctx context.Context, result *report.PathResult, project string, step releaseStep, filename string) error {
	b, err := e.Compose.Logs(ctx, project, step.State.Env)
	writeErr := os.WriteFile(filepath.Join(result.ArtifactDirectory, filename), b, 0o644)
	if err != nil {
		return err
	}
	return writeErr
}

func (e *Engine) hookEnvironment(runID, project, phase, pathName string, current releaseStep, artifactDir string) hooks.Environment {
	return hooks.Environment{RunID: runID, Project: project, Phase: phase, Path: pathName, FromStep: "from", CurrentStep: current.Name, TargetStep: "to", ComposeFile: e.Compose.ComposeFile, ReportDir: artifactDir, ReleaseEnv: current.State.Env}
}

func materializeSteps(path config.Path, runID string) []releaseStep {
	steps := []releaseStep{{Name: "from", State: cloneState(path.From)}}
	for i, state := range path.Via {
		steps = append(steps, releaseStep{Name: fmt.Sprintf("via-%d", i+1), State: cloneState(state)})
	}
	target := cloneState(path.To)
	if target.Build != nil {
		target.Env[target.Build.TagEnv] = "upgradeproof-target-" + runID
	}
	return append(steps, releaseStep{Name: "to", State: target})
}

func cloneState(state config.ReleaseState) config.ReleaseState {
	copyState := state
	copyState.Env = make(map[string]string, len(state.Env))
	for key, value := range state.Env {
		copyState.Env[key] = value
	}
	if state.Build != nil {
		copyBuild := *state.Build
		copyBuild.Services = append([]string(nil), state.Build.Services...)
		copyState.Build = &copyBuild
	}
	return copyState
}

func validateBuildOwnership(target releaseStep, runID string) error {
	if target.State.Build == nil {
		return nil
	}
	suffix := ":upgradeproof-target-" + runID
	for _, serviceName := range target.State.Build.Services {
		service, ok := target.Model.Services[serviceName]
		if !ok {
			return fmt.Errorf("target build service %q does not exist in resolved Compose model", serviceName)
		}
		if !strings.HasSuffix(service.Image, suffix) {
			return fmt.Errorf("target build service %q image %q is not tagged through build.tag_env %q", serviceName, service.Image, target.State.Build.TagEnv)
		}
	}
	return nil
}

func (e *Engine) pathArtifactDir(runID, pathName string) string {
	return filepath.Join(e.Options.ReportDir, runID, hooks.SafeName(pathName))
}

func (e *Engine) writeReports(value report.Report) error {
	dir := filepath.Join(e.Options.ReportDir, value.RunID)
	return errors.Join(report.WriteJSON(filepath.Join(dir, "report.json"), value), report.WriteJUnit(filepath.Join(dir, "junit.xml"), value))
}

func stage(result *report.PathResult, name string, fn func() error) error {
	s := report.Stage{Name: name, StartedAt: time.Now().UTC(), Status: "passed"}
	err := fn()
	s.FinishedAt = time.Now().UTC()
	s.Duration = s.FinishedAt.Sub(s.StartedAt).String()
	if err != nil {
		s.Status = "failed"
		s.Error = err.Error()
	}
	result.Steps = append(result.Steps, s)
	return err
}

func addSkippedStage(result *report.PathResult, name, reason string) {
	now := time.Now().UTC()
	result.Steps = append(result.Steps, report.Stage{Name: name, StartedAt: now, FinishedAt: now, Duration: "0s", Status: "skipped", Error: reason})
}

func GenerateProjectName(repo, pathName, runID string) string {
	sum := sha256.Sum256([]byte(repo + "\x00" + pathName + "\x00" + runID))
	suffix := hex.EncodeToString(sum[:6])
	prefix := strings.Trim("upgradeproof-"+safe(repo)+"-"+safe(pathName), "-")
	maxPrefix := 63 - 1 - len(suffix)
	if len(prefix) > maxPrefix {
		prefix = strings.TrimRight(prefix[:maxPrefix], "-")
	}
	return prefix + "-" + suffix
}

func safe(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func newRunID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err == nil {
		return time.Now().UTC().Format("20060102t150405") + "-" + hex.EncodeToString(b)
	}
	return time.Now().UTC().Format("20060102t150405000000000")
}
