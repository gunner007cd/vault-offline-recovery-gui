package main

import (
	"os"
	"strings"
	"testing"
)

// Ensure that the more decorative portrait layout can't accidentally narrow
// recovery destination inputs or delete restore controls on future edits.
func TestPortraitEditionOnlyShortensConnectionRows(t *testing.T) {
	b, err := os.ReadFile("windows_gui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"settings := calculateTopFormGeometry(w)",
		"move(a.fields[r.id], settings.FieldX, r.y, settings.FieldWidth, 26)",
		"move(a.fields[r.browse], settings.BrowseX, r.y, settings.BrowseWidth, 26)",
		"move(a.fields[idDestination], destination.FieldX, geo.DestY, destination.FieldWidth, 26)",
		"move(a.fields[idOpenBrowser]",
		"a.showBrowser()",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing expected top-form or recovery UI structure: %s", want)
		}
	}
	a, err := os.ReadFile("avatar_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a), "calculateTopFormGeometry(int(clientWidth))") {
		t.Fatal("portrait must follow same layout calculations as path fields")
	}
}
