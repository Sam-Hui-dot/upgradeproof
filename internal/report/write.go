package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJSON(path string, value Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func WriteJUnit(path string, value Report) error {
	root := junitSuites{Name: "upgradeproof"}
	for _, p := range value.Paths {
		suite := junitSuite{Name: p.Name}
		hasFailedCheck := false
		for _, check := range p.Checks {
			caseResult := junitCase{Name: check.Name, Classname: "upgradeproof." + p.Name, Time: seconds(check)}
			if check.Status != "passed" {
				hasFailedCheck = true
				caseResult.Failure = &junitFailure{Message: check.Error, Body: fmt.Sprintf("exit_code=%d stdout=%s stderr=%s", check.ExitCode, check.StdoutPath, check.StderrPath)}
				suite.Failures++
			}
			suite.Tests++
			suite.Cases = append(suite.Cases, caseResult)
		}
		if p.Status != "passed" && !hasFailedCheck {
			message, body := pathFailure(p)
			suite.Cases = append(suite.Cases, junitCase{
				Name:      "path-lifecycle",
				Classname: "upgradeproof." + p.Name,
				Time:      "0.000",
				Failure:   &junitFailure{Message: message, Body: body},
			})
			suite.Tests++
			suite.Failures++
		}
		root.Tests += suite.Tests
		root.Failures += suite.Failures
		root.Suites = append(root.Suites, suite)
	}
	b, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	b = append([]byte(xml.Header), b...)
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func pathFailure(p PathResult) (string, string) {
	for _, stage := range p.Steps {
		if stage.Status == "failed" {
			message := stage.Error
			if message == "" {
				message = "path lifecycle stage failed"
			}
			return message, fmt.Sprintf("path_status=%s stage=%s artifact_directory=%s", p.Status, stage.Name, p.ArtifactDirectory)
		}
	}
	return "path lifecycle failed", fmt.Sprintf("path_status=%s artifact_directory=%s", p.Status, p.ArtifactDirectory)
}

func seconds(check CheckResult) string {
	d := check.FinishedAt.Sub(check.StartedAt).Seconds()
	return fmt.Sprintf("%.3f", d)
}
