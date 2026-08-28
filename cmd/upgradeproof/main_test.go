package main

import (
	"runtime/debug"
	"testing"
)

func TestMetadataFallsBackToGoModuleBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.1.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-08-29T00:00:00Z"},
		},
	}
	gotVersion, gotCommit, gotDate := metadataFromBuildInfo("dev", "unknown", "unknown", info)
	if gotVersion != "v0.1.0" || gotCommit != "abc123" || gotDate != "2026-08-29T00:00:00Z" {
		t.Fatalf("unexpected module metadata: version=%q commit=%q date=%q", gotVersion, gotCommit, gotDate)
	}

	gotVersion, gotCommit, gotDate = metadataFromBuildInfo("v0.1.0", "release-commit", "release-date", info)
	if gotVersion != "v0.1.0" || gotCommit != "release-commit" || gotDate != "release-date" {
		t.Fatalf("linker metadata was overwritten: version=%q commit=%q date=%q", gotVersion, gotCommit, gotDate)
	}
}
