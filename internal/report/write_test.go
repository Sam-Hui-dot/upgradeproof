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

func TestJUnitReportsPathFailureBeforeVerification(t *testing.T) {
	now := time.Now().UTC()
	value := Report{RunID: "run", OverallStatus: "failed", Paths: []PathResult{{
		Name: "health-failure", Status: "failed", ArtifactDirectory: "artifacts/path",
		Steps: []Stage{{Name: "WAIT_FROM_HEALTH", Status: "failed", Error: "health timeout", StartedAt: now, FinishedAt: now}},
	}}}
	path := filepath.Join(t.TempDir(), "junit.xml")
	if err := WriteJUnit(path, value); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	text := string(b)
	if !strings.Contains(text, `tests="1" failures="1"`) || !strings.Contains(text, `name="path-lifecycle"`) || !strings.Contains(text, `stage=WAIT_FROM_HEALTH`) {
		t.Fatalf("path failure missing from JUnit: %s", text)
	}
}

func TestReportsDoNotContainInheritedEnvironment(t *testing.T) {
	t.Setenv("UPGRADEPROOF_REPORT_SECRET_TEST", "do-not-report-this-secret")
	value := Report{RunID: "run", OverallStatus: "passed"}
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
	combined := string(jsonData) + string(junitData)
	if strings.Contains(combined, "do-not-report-this-secret") || strings.Contains(combined, "UPGRADEPROOF_REPORT_SECRET_TEST") {
		t.Fatal("inherited environment leaked into report")
	}
}
