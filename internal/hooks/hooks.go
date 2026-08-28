package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	commandpkg "github.com/Sam-Hui-dot/upgradeproof/internal/command"
	"github.com/Sam-Hui-dot/upgradeproof/internal/report"
)

type Environment struct {
	RunID        string
	Project      string
	Phase        string
	Path         string
	FromImage    string
	CurrentImage string
	TargetImage  string
	ComposeFile  string
	ReportDir    string
}

type Runner struct {
	CommandRunner commandpkg.Runner
	RootDir       string
}

func (r Runner) Run(ctx context.Context, name, command string, timeout time.Duration, env Environment, stdoutPath, stderrPath string) report.CheckResult {
	result := report.CheckResult{Name: name, Status: "failed", ExitCode: -1, StdoutPath: stdoutPath, StderrPath: stderrPath, StartedAt: time.Now().UTC()}
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	shell, args := shellCommand(command)
	runResult, err := r.CommandRunner.Run(hookCtx, r.RootDir, shell, args, controlledEnvironment(env))
	_ = os.MkdirAll(filepath.Dir(stdoutPath), 0o755)
	stdoutErr := os.WriteFile(stdoutPath, runResult.Stdout, 0o644)
	stderrErr := os.WriteFile(stderrPath, runResult.Stderr, 0o644)
	result.FinishedAt = time.Now().UTC()
	result.Duration = result.FinishedAt.Sub(result.StartedAt).String()
	result.ExitCode = runResult.ExitCode
	if err == nil && stdoutErr == nil && stderrErr == nil {
		result.Status = "passed"
		return result
	}
	if hookCtx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("hook timed out after %s", timeout)
	} else if err != nil {
		result.Error = err.Error()
	} else if stdoutErr != nil {
		result.Error = stdoutErr.Error()
	} else {
		result.Error = stderrErr.Error()
	}
	return result
}

func controlledEnvironment(e Environment) []string {
	return []string{
		"UPGRADEPROOF_RUN_ID=" + e.RunID,
		"UPGRADEPROOF_PROJECT=" + e.Project,
		"UPGRADEPROOF_PHASE=" + e.Phase,
		"UPGRADEPROOF_PATH=" + e.Path,
		"UPGRADEPROOF_FROM_IMAGE=" + e.FromImage,
		"UPGRADEPROOF_CURRENT_IMAGE=" + e.CurrentImage,
		"UPGRADEPROOF_TARGET_IMAGE=" + e.TargetImage,
		"UPGRADEPROOF_COMPOSE_FILE=" + e.ComposeFile,
		"UPGRADEPROOF_REPORT_DIR=" + e.ReportDir,
	}
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/D", "/S", "/C", command}
	}
	return "sh", []string{"-c", command}
}

func SafeName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
