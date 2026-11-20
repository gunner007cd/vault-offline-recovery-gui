package main

import (
	"os"
	"strings"
	"testing"
)

// The diagnostics console should be gone, but the original engine's error
// and verified success contract must remain visible in the main window.
func TestCompactStatusAndErrorHandling(t *testing.T) {
	source, err := os.ReadFile("windows_gui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, want := range []string{
		`a.setStatus("FAILED / NOT CONFIRMED — " + statusErrorSummary(ev.err))`,
		`displayError(a.hwnd, ev.err.Error())`,
		`CONFIRMED by original Vault report:`,
		`ev.elapsed = time.Since(started)`,
		`if ev.mode == "heartbeat"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing status/error safeguard: %q", want)
		}
	}
	for _, forbidden := range []string{`a.control(500,`, `a.labels["log"]`, `a.fields[500]`, `a.log(`, `Actual Vault output / diagnostics`} {
		if strings.Contains(s, forbidden) {
			t.Errorf("diagnostics control still present: %q", forbidden)
		}
	}
}

func TestRecoveryAdapterNotModified(t *testing.T) {
	source, err := os.ReadFile("diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"exec.Command", "aes.", "age.", "os.WriteFile", "Decrypt", "Extract"} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("status helpers must not implement recovery logic %q", forbidden)
		}
	}
}
