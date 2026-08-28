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

func (e *Engine) Run(ctx context.Context) (report.Report, int) {
	run := report.Report{ToolVersion: e.Options.ToolVersion, RunID: newRunID(), StartedAt: time.Now().UTC(), ConfigPath: e.Options.ConfigPath, OverallStatus: "passed"}
	paths, err := e.Config.SelectPath(e.Options.SelectedPath)
	if err != nil {
		run.OverallStatus = "preflight_failed"
		run.FinishedAt = time.Now().UTC()
		return run, ExitPreflight
	}
	if err := e.Compose.Validate(ctx, paths[0].From); err != nil {
		run.OverallStatus = "infrastructure_failed"
		pathStatus := "infrastructure_failed"
		if ctx.Err() != nil {
			run.OverallStatus = "interrupted"
			pathStatus = "interrupted"
		}
		for _, p := range paths {
			project := GenerateProjectName(filepath.Base(e.Options.RootDir), p.Name, run.RunID)
			failed := failedBeforePath(p.Name, project, e.pathArtifactDir(run.RunID, p.Name), "PREFLIGHT", err)
			failed.Status = pathStatus
			run.Paths = append(run.Paths, failed)
		}
		run.FinishedAt = time.Now().UTC()
		e.writeReports(run)
		return run, ExitInfrastructure
	}
	targetImages := map[string]string{}
	for _, p := range paths {
		if p.To.Build == nil {
			continue
		}
		key := p.To.Build.Service
		if _, ok := targetImages[key]; ok {
			continue
		}
		tag := "upgradeproof-target:" + run.RunID
		buildProject := GenerateProjectName(filepath.Base(e.Options.RootDir), "target-build", run.RunID)
		if err := e.Compose.Build(ctx, buildProject, tag); err != nil {
			run.OverallStatus = "infrastructure_failed"
			failed := failedBeforePath(p.Name, buildProject, e.pathArtifactDir(run.RunID, p.Name), "RESOLVE_IMAGES", err)
			if ctx.Err() != nil {
				run.OverallStatus = "interrupted"
				failed.Status = "interrupted"
			}
			run.Paths = append(run.Paths, failed)
			run.FinishedAt = time.Now().UTC()
			e.writeReports(run)
			return run, ExitInfrastructure
		}
		targetImages[key] = tag
	}

	exitCode := ExitPassed
	for _, path := range paths {
		result, code := e.runPath(ctx, run.RunID, path, targetImages)
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

func (e *Engine) runPath(ctx context.Context, runID string, path config.Path, targetImages map[string]string) (result report.PathResult, exitCode int) {
	artifactDir := e.pathArtifactDir(runID, path.Name)
	project := GenerateProjectName(filepath.Base(e.Options.RootDir), path.Name, runID)
	result = report.PathResult{Name: path.Name, Status: "passed", ProjectName: project, ArtifactDirectory: artifactDir}
	exitCode = ExitPassed
	targetImage := path.To.Image
	targetRequested := path.To.Image
	if path.To.Build != nil {
		targetImage = targetImages[path.To.Build.Service]
		targetRequested = "build:" + path.To.Build.Service
	}
	result.RequestedImages = append([]string{path.From}, path.Via...)
	result.RequestedImages = append(result.RequestedImages, targetRequested)

	stage(&result, "PREFLIGHT", func() error { return nil })
	stage(&result, "RESOLVE_IMAGES", func() error { return nil })
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		stage(&result, "PREPARE_PROJECT", func() error { return err })
		result.Status = "infrastructure_failed"
		return result, ExitInfrastructure
	}
	stage(&result, "PREPARE_PROJECT", func() error { return nil })
	current := path.From
	if err := stage(&result, "START_FROM", func() error { return e.Compose.UpFrom(ctx, project, current) }); err != nil {
		exitCode = ExitInfrastructure
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, exitCode, current)
	}
	if err := stage(&result, "WAIT_FROM_HEALTH", func() error {
		return e.Health.Wait(ctx, e.Config.Health.URL, e.Config.Health.Timeout.Duration, e.Config.Health.Interval.Duration)
	}); err != nil {
		exitCode = ExitTestFailed
		result.Status = "failed"
		e.captureLog(ctx, &result, current, "compose-source.log")
		return e.finishPath(ctx, result, exitCode, current)
	}
	if err := stage(&result, "CAPTURE_SOURCE_EVIDENCE", func() error { return e.captureLog(ctx, &result, current, "compose-source.log") }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, ExitInfrastructure, current)
	}
	if err := stage(&result, "RESOLVE_FROM_IMAGE", func() error { return e.resolve(ctx, &result, current, current) }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, ExitInfrastructure, current)
	}

	seedEnv := e.hookEnvironment(runID, project, "seed", path.Name, path.From, current, targetRequested, artifactDir)
	var seed report.CheckResult
	seedErr := stage(&result, "SEED", func() error {
		seed = e.Hooks.Run(ctx, "seed", e.Config.Seed.Command, e.Config.Seed.Timeout.Duration, seedEnv, filepath.Join(artifactDir, "seed.stdout"), filepath.Join(artifactDir, "seed.stderr"))
		result.Hooks = append(result.Hooks, seed)
		if seed.Status != "passed" {
			return errors.New(seed.Error)
		}
		return nil
	})
	if seedErr != nil {
		exitCode = ExitTestFailed
		result.Status = "failed"
		return e.finishPath(ctx, result, exitCode, current)
	}
	if err := stage(&result, "CAPTURE_SEEDED_STATE", func() error { return e.captureLog(ctx, &result, current, "compose-seeded.log") }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, ExitInfrastructure, current)
	}

	for i, image := range path.Via {
		current = image
		stepName := fmt.Sprintf("UPGRADE_STEP_%d", i+1)
		if err := stage(&result, stepName, func() error { return e.Compose.Upgrade(ctx, project, image) }); err != nil {
			exitCode = ExitInfrastructure
			result.Status = "infrastructure_failed"
			return e.finishPath(ctx, result, exitCode, current)
		}
		waitName := fmt.Sprintf("WAIT_STEP_%d_HEALTH", i+1)
		if err := stage(&result, waitName, func() error {
			return e.Health.Wait(ctx, e.Config.Health.URL, e.Config.Health.Timeout.Duration, e.Config.Health.Interval.Duration)
		}); err != nil {
			exitCode = ExitTestFailed
			result.Status = "failed"
			e.captureLog(ctx, &result, current, fmt.Sprintf("compose-step-%d.log", i+1))
			return e.finishPath(ctx, result, exitCode, current)
		}
		if err := stage(&result, fmt.Sprintf("CAPTURE_STEP_%d_EVIDENCE", i+1), func() error { return e.captureLog(ctx, &result, current, fmt.Sprintf("compose-step-%d.log", i+1)) }); err != nil {
			result.Status = "infrastructure_failed"
			return e.finishPath(ctx, result, ExitInfrastructure, current)
		}
		if err := stage(&result, fmt.Sprintf("RESOLVE_STEP_%d_IMAGE", i+1), func() error { return e.resolve(ctx, &result, current, current) }); err != nil {
			result.Status = "infrastructure_failed"
			return e.finishPath(ctx, result, ExitInfrastructure, current)
		}
	}

	current = targetImage
	if err := stage(&result, "UPGRADE_TARGET", func() error { return e.Compose.Upgrade(ctx, project, targetImage) }); err != nil {
		exitCode = ExitInfrastructure
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, exitCode, current)
	}
	if err := stage(&result, "WAIT_TARGET_HEALTH", func() error {
		return e.Health.Wait(ctx, e.Config.Health.URL, e.Config.Health.Timeout.Duration, e.Config.Health.Interval.Duration)
	}); err != nil {
		exitCode = ExitTestFailed
		result.Status = "failed"
		e.captureLog(ctx, &result, current, "compose-target.log")
		return e.finishPath(ctx, result, exitCode, current)
	}
	if err := stage(&result, "CAPTURE_TARGET_EVIDENCE", func() error { return e.captureLog(ctx, &result, current, "compose-target.log") }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, ExitInfrastructure, current)
	}
	if err := stage(&result, "RESOLVE_TARGET_IMAGE", func() error { return e.resolve(ctx, &result, current, targetRequested) }); err != nil {
		result.Status = "infrastructure_failed"
		return e.finishPath(ctx, result, ExitInfrastructure, current)
	}

	verifyFailed := false
	stage(&result, "VERIFY", func() error {
		for _, check := range e.Config.Verify.Checks {
			name := hooks.SafeName(check.Name)
			hookEnv := e.hookEnvironment(runID, project, "verify", path.Name, path.From, current, targetRequested, artifactDir)
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
		exitCode = ExitTestFailed
		result.Status = "failed"
	}
	return e.finishPath(ctx, result, exitCode, current)
}

func (e *Engine) finishPath(ctx context.Context, result report.PathResult, code int, current string) (report.PathResult, int) {
	if ctx.Err() != nil {
		result.Status = "interrupted"
		code = ExitInfrastructure
	}
	evidenceCtx, evidenceCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer evidenceCancel()
	if err := stage(&result, "CAPTURE_EVIDENCE", func() error { return e.captureLog(evidenceCtx, &result, current, "compose-final.log") }); err != nil && code < ExitInfrastructure {
		code = ExitInfrastructure
		result.Status = "infrastructure_failed"
	}
	stage(&result, "REPORT", func() error { return nil })
	if code != ExitPassed && e.Options.KeepOnFailure && ctx.Err() == nil {
		addSkippedStage(&result, "CLEANUP", "kept because --keep-on-failure was set")
		return result, code
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := stage(&result, "CLEANUP", func() error { return e.Compose.Cleanup(cleanupCtx, result.ProjectName, current) }); err != nil {
		result.Status = "infrastructure_failed"
		return result, ExitInfrastructure
	}
	return result, code
}

func (e *Engine) resolve(ctx context.Context, result *report.PathResult, currentImage, requested string) error {
	resolved, err := e.Compose.ResolveImage(ctx, result.ProjectName, currentImage)
	if err != nil {
		return err
	}
	result.ResolvedImages = append(result.ResolvedImages, report.ImageIdentity{Requested: requested, Resolved: resolved})
	return nil
}

func (e *Engine) captureLog(ctx context.Context, result *report.PathResult, image, filename string) error {
	b, err := e.Compose.Logs(ctx, result.ProjectName, image)
	writeErr := os.WriteFile(filepath.Join(result.ArtifactDirectory, filename), b, 0o644)
	if err != nil {
		return err
	}
	return writeErr
}

func (e *Engine) hookEnvironment(runID, project, phase, pathName, from, current, target, artifactDir string) hooks.Environment {
	return hooks.Environment{RunID: runID, Project: project, Phase: phase, Path: pathName, FromImage: from, CurrentImage: current, TargetImage: target, ComposeFile: e.Compose.ComposeFile, ReportDir: artifactDir}
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

func failedBeforePath(name, project, artifact, stageName string, err error) report.PathResult {
	result := report.PathResult{Name: name, Status: "infrastructure_failed", ProjectName: project, ArtifactDirectory: artifact}
	stage(&result, stageName, func() error { return err })
	return result
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
