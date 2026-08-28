package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/Sam-Hui-dot/upgradeproof/internal/command"
	"github.com/Sam-Hui-dot/upgradeproof/internal/compose"
	"github.com/Sam-Hui-dot/upgradeproof/internal/config"
	"github.com/Sam-Hui-dot/upgradeproof/internal/engine"
	"github.com/Sam-Hui-dot/upgradeproof/internal/health"
	"github.com/Sam-Hui-dot/upgradeproof/internal/hooks"
	"github.com/Sam-Hui-dot/upgradeproof/internal/safety"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	version, commit, date = metadataFromBuildInfo(version, commit, date, info)
}

func metadataFromBuildInfo(currentVersion, currentCommit, currentDate string, info *debug.BuildInfo) (string, string, string) {
	if currentVersion == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		currentVersion = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if currentCommit == "unknown" && setting.Value != "" {
				currentCommit = setting.Value
			}
		case "vcs.time":
			if currentDate == "unknown" && setting.Value != "" {
				currentDate = setting.Value
			}
		}
	}
	return currentVersion, currentCommit, currentDate
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return engine.ExitPreflight
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:])
	case "test":
		return runTest(args[1:])
	case "version":
		fmt.Printf("upgradeproof %s (commit=%s date=%s)\n", version, commit, date)
		return engine.ExitPassed
	case "help", "-h", "--help":
		usage()
		return engine.ExitPassed
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		return engine.ExitPreflight
	}
}

func runValidate(args []string) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("c", "upgradeproof.yml", "configuration file")
	if err := flags.Parse(args); err != nil {
		return engine.ExitPreflight
	}
	cfg, client, _, err := loadProject(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		return engine.ExitPreflight
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Health.Timeout.Duration)
	defer cancel()
	for _, path := range cfg.Paths {
		states := append([]config.ReleaseState{path.From}, path.Via...)
		states = append(states, path.To)
		for i, state := range states {
			if state.Build != nil {
				state.Env = cloneEnvironment(state.Env)
				state.Env[state.Build.TagEnv] = "upgradeproof-target-validation"
			}
			model, validateErr := client.Validate(ctx, state.Env)
			if validateErr == nil {
				validateErr = engine.ValidateBuildOwnership(state, model, "validation")
			}
			if validateErr != nil {
				err := validateErr
				if safety.IsViolation(err) || errors.Is(err, engine.ErrInvalidBuildTarget) {
					fmt.Fprintf(os.Stderr, "validation failed for path %q state %d: %v\n", path.Name, i, err)
					return engine.ExitPreflight
				}
				fmt.Fprintf(os.Stderr, "validation infrastructure failed for path %q state %d: %v\n", path.Name, i, err)
				return engine.ExitInfrastructure
			}
		}
	}
	fmt.Printf("configuration valid: %d path(s), Compose release states resolved and audited\n", len(cfg.Paths))
	return engine.ExitPassed
}

func cloneEnvironment(env map[string]string) map[string]string {
	copyEnv := make(map[string]string, len(env))
	for key, value := range env {
		copyEnv[key] = value
	}
	return copyEnv
}

func runTest(args []string) int {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("c", "upgradeproof.yml", "configuration file")
	selectedPath := flags.String("path", "", "run one named path")
	keepOnFailure := flags.Bool("keep-on-failure", false, "keep the generated Compose project after failure")
	reportDir := flags.String("report-dir", ".upgradeproof", "report and evidence directory")
	if err := flags.Parse(args); err != nil {
		return engine.ExitPreflight
	}
	cfg, client, rootDir, err := loadProject(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
		return engine.ExitPreflight
	}
	if _, err := cfg.SelectPath(*selectedPath); err != nil {
		fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
		return engine.ExitPreflight
	}
	reports := *reportDir
	if !filepath.IsAbs(reports) {
		reports = filepath.Join(rootDir, reports)
	}
	runner := command.ExecRunner{}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	executor := engine.Engine{
		Config:  cfg,
		Compose: client,
		Hooks:   hooks.Runner{CommandRunner: runner, RootDir: rootDir},
		Health:  health.HTTPWaiter{},
		Options: engine.Options{ConfigPath: absolute(*configPath), RootDir: rootDir, ReportDir: reports, SelectedPath: *selectedPath, KeepOnFailure: *keepOnFailure, ToolVersion: version},
	}
	result, code := executor.Run(ctx)
	fmt.Printf("run %s: %s (%d path(s)); reports: %s\n", result.RunID, result.OverallStatus, len(result.Paths), filepath.Join(reports, result.RunID))
	for _, path := range result.Paths {
		fmt.Printf("  %s: %s\n", path.Name, path.Status)
		for _, check := range path.Checks {
			fmt.Printf("    %s: %s\n", check.Name, check.Status)
		}
	}
	return code
}

func loadProject(configPath string) (*config.Config, compose.Client, string, error) {
	absConfig := absolute(configPath)
	cfg, err := config.Load(absConfig)
	if err != nil {
		return nil, compose.Client{}, "", err
	}
	rootDir := filepath.Dir(absConfig)
	composePath := cfg.Compose.File
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Join(rootDir, composePath)
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, compose.Client{}, "", err
	}
	if err := safety.CheckCompose(data); err != nil {
		return nil, compose.Client{}, "", fmt.Errorf("unsafe Compose configuration: %w", err)
	}
	runner := command.ExecRunner{}
	return cfg, compose.New(rootDir, composePath, runner), rootDir, nil
}

func absolute(path string) string {
	value, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return value
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: upgradeproof <validate|test|version> [options]")
}
