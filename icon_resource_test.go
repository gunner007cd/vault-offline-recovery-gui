package main

import (
	"os"
	"strings"
	"testing"
)

func TestWindowClassesUseEmbeddedMascotIcons(t *testing.T) {
	if _, err := os.Stat("appicon_windows_amd64.syso"); err != nil {
		t.Fatalf("native icon resource required: %v", err)
	}
	for _, filename := range []string{"windows_gui.go", "file_browser_windows.go"} {
		b, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		source := string(b)
		for _, snippet := range []string{"appWindowIcons(instance)", "Icon: largeIcon", "IconSm: smallIcon"} {
			if !strings.Contains(source, snippet) {
				t.Errorf("%s missing %q", filename, snippet)
			}
		}
	}
}
