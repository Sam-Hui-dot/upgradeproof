package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
			if _, err := client.Validate(ctx, state.Env); err != nil {
				if safety.IsViolation(err) {
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
