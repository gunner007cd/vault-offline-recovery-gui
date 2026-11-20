package main

import (
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsSummaries(t *testing.T) {
	p := []Point{{Items: []Item{{Name: "one"}, {Name: "two"}}}, {Locked: true, Items: []Item{{Name: "three"}}}}
	if got := pointSummary(p); got != "2 restore points, 3 listed items, 1 locked" {
		t.Fatal(got)
	}
	e := []Entry{{Path: "docs", IsDir: true}, {Path: "docs/a", Size: 1024}, {Path: "b", Size: 1048576}}
	if got := fileSummary(e); got != "3 paths (2 files, 1 folder), 1.0 MiB indexed file sizes" {
		t.Fatal(got)
	}
}
func TestElapsedLabels(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0m 00s", 90 * time.Second: "1m 30s", time.Hour + 5*time.Minute + 3*time.Second: "1h 05m 03s"} {
		if got := elapsedLabel(d); got != want {
			t.Fatalf("elapsed %v: %s != %s", d, got, want)
		}
	}
}
func TestDiagnosticsNeverEmitUnknownProgress(t *testing.T) {
	output := diagLine(time.Date(2026, 11, 19, 14, 15, 16, 0, time.UTC), "inspect", "Vault still running; elapsed 1m 30s")
	if !strings.Contains(output, "[14:15:16] INSPECT") || strings.Contains(output, "%") {
		t.Fatalf("invalid diagnostic %q", output)
	}
}
