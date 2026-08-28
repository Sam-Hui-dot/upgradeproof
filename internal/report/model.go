package report

import "time"

type Report struct {
	ToolVersion   string       `json:"tool_version"`
	RunID         string       `json:"run_id"`
	StartedAt     time.Time    `json:"started_at"`
	FinishedAt    time.Time    `json:"finished_at"`
	ConfigPath    string       `json:"config_path"`
	OverallStatus string       `json:"overall_status"`
	Paths         []PathResult `json:"paths"`
}

type PathResult struct {
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Steps             []Stage           `json:"steps"`
	Hooks             []CheckResult     `json:"hooks"`
	Checks            []CheckResult     `json:"checks"`
	ReleaseStates     []ReleaseIdentity `json:"release_states"`
	ProjectName       string            `json:"project_name"`
	ArtifactDirectory string            `json:"artifact_directory"`
}

type Stage struct {
	Name       string    `json:"name"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Duration   string    `json:"duration"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

type ReleaseIdentity struct {
	Step     string          `json:"step"`
	Services []ImageIdentity `json:"services"`
}

type ImageIdentity struct {
	Service   string `json:"service"`
	Container string `json:"container_id"`
	Requested string `json:"requested"`
	Resolved  string `json:"resolved"`
}

type CheckResult struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Duration   string    `json:"duration"`
	ExitCode   int       `json:"exit_code"`
	StdoutPath string    `json:"stdout_path"`
	StderrPath string    `json:"stderr_path"`
	Error      string    `json:"error,omitempty"`
}
