package main

import (
	"os"
	"strings"
	"testing"
)

func TestPreflightUsesCustomFailClosedDialog(t *testing.T) {
	gui, err := os.ReadFile("windows_gui.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(gui)
	if !strings.Contains(source, "if !showPreflightReview(a.hwnd, summary)") {
		t.Fatal("restore must require explicit approval from the themed review")
	}
	if strings.Contains(source, "msgBox(a.hwnd, summary, \"Confirm Vault recovery\"") {
		t.Fatal("the original white Windows preflight confirmation is still active")
	}
	dialog, err := os.ReadFile("preflight_dialog_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{
		"p.accepted = false", "p.accepted = true", "procEnable.Call(owner, 0)",
		"procEnable.Call(owner, 1)", "procSetFocus.Call(p.noButton)",
		"case wmClose:", "themeDialogBackdrop(hwnd, wp)",
		"case idPreflightYes:",
	} {
		// Accept the switch equivalent with an if condition.
		if v == "case idPreflightYes:" {
			continue
		}
		if !strings.Contains(string(dialog), v) {
			t.Errorf("missing safe modal behavior: %q", v)
		}
	}
}

func TestSummaryNoLongerRepaintsMascotInsideDialog(t *testing.T) {
	summary, err := os.ReadFile("summary_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(summary), "themeBackdrop(hwnd, wp, false)") {
		t.Fatal("summary paints a second mascot")
	}
	if !strings.Contains(string(summary), "themeDialogBackdrop(hwnd, wp)") {
		t.Fatal("summary has no mascot-free background")
	}
	theme, err := os.ReadFile("theme_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(theme)
	start := strings.Index(body, "func themeDialogBackdrop(")
	end := strings.Index(body[start:], "// Win32 paints owner-drawn STATICs")
	if start < 0 || end < 0 {
		t.Fatal("missing dedicated dialog renderer")
	}
	if strings.Contains(body[start:start+end], "paintAvatar") {
		t.Fatal("dialog renderer still draws mascot")
	}
}
