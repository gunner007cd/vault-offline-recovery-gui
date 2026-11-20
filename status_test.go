package main

import (
	"errors"
	"strings"
	"testing"
)

func TestStatusErrorSummaryUsesRealVaultError(t *testing.T) {
	errorMessage := "Vault recovery executable exited with code 1.\n\n--- actual Vault stderr ---\nError: cannot read index for Flash Drive\n\n--- actual Vault stdout ---\nmore info"
	actual := statusErrorSummary(errors.New(errorMessage))
	if actual != "Error: cannot read index for Flash Drive" {
		t.Fatalf("got %q; want actual stderr", actual)
	}
	if s := statusErrorSummary(errors.New("failed to launch Vault executable: access denied")); !strings.Contains(s, "access denied") {
		t.Fatalf("launch failure missing: %q", s)
	}
	if got := statusErrorSummary(nil); got != "" {
		t.Fatalf("nil error: %q", got)
	}
}
