package main

import (
	"os"
	"strings"
	"testing"
)

// These source-contract checks are executed on Linux build hosts too. They
// safeguard the native Windows message routing that Linux cannot exercise.
// They complement, but do not replace, interactive verification on Windows.
func TestAmberSelectionWiredAcrossMainAndBrowser(t *testing.T) {
	for filename, snippets := range map[string][]string{
		"windows_gui.go": {
			"lbsOwnerDrawFixed|lbsHasStrings",
			"drawThemedListbox(lp,",
			"case wmMeasureItem:",
			"installThemedListbox(v)",
		},
		"file_browser_windows.go": {
			"themeBrowserCustomDraw(lp, true)",
			"themeBrowserCustomDraw(lp, false)",
			"drawThemedListbox(lp,",
			"installThemedListbox(h)",
		},
		"theme_windows.go": {
			"shadeSelection  = themeRGB(111, 65, 36)",
			"shadeFocus      = themeRGB(245, 147, 62)",
			"procSetTextColor.Call(d.HDC, shadeInk)",
			"func themeBrowserCustomDraw(",
			"func themeButtonSubclass(",
			"func themeListSubclass(",
			"func measureThemedListbox(",
		},
	} {
		b, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range snippets {
			if !strings.Contains(string(b), needle) {
				t.Errorf("%s is missing theme contract %q", filename, needle)
			}
		}
	}
}

func TestThemeDoesNotChangeRecoveryEngine(t *testing.T) {
	// The GUI must keep delegating inspection and restoration to the CLI.
	b, err := os.ReadFile("core.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"\"recover\"", "\"contents\"", "\"extract\""} {
		if !strings.Contains(string(b), cmd) {
			t.Errorf("missing original CLI argument %s", cmd)
		}
	}
}
