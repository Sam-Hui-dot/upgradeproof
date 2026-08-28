package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJSONAndJUnitReports(t *testing.T) {
	now := time.Now().UTC()
	value := Report{ToolVersion: "test", RunID: "run", StartedAt: now, FinishedAt: now, OverallStatus: "failed", Paths: []PathResult{{Name: "broken", Status: "failed", Checks: []CheckResult{{Name: "users-preserved", Status: "failed", ExitCode: 1, StartedAt: now, FinishedAt: now, Error: "invariant failed", StdoutPath: "out", StderrPath: "err"}}}}}
	dir := t.TempDir()
	jsonPath, junitPath := filepath.Join(dir, "report.json"), filepath.Join(dir, "junit.xml")
	if err := WriteJSON(jsonPath, value); err != nil {
		t.Fatal(err)
	}
	if err := WriteJUnit(junitPath, value); err != nil {
		t.Fatal(err)
	}
	jsonData, _ := os.ReadFile(jsonPath)
	junitData, _ := os.ReadFile(junitPath)
	if !strings.Contains(string(jsonData), `"overall_status": "failed"`) {
		t.Fatal("JSON status missing")
	}
	if !strings.Contains(string(junitData), `<failure message="invariant failed"`) {
		t.Fatal("JUnit failure missing")
	}
}
