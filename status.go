package main

import (
	"strings"
)

// statusErrorSummary gives the operator an actual CLI error on the status line
// instead of a generic "see diagnostics" message. Full, unmodified error
// details are shown in a native error dialog; the status is intentionally
// single-line and may be visually clipped in small windows.
func statusErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	// RunResult.Details contains the CLI exit code before a stderr section.
	// Prefer the engine's concrete error text when it's available.
	if i := strings.Index(raw, "--- actual Vault stderr ---"); i >= 0 {
		raw = raw[i+len("--- actual Vault stderr ---"):]
		if j := strings.Index(raw, "--- actual Vault stdout ---"); j >= 0 {
			raw = raw[:j]
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return strings.Join(strings.Fields(line), " ")
		}
	}
	return "Vault returned an unspecified error"
}
