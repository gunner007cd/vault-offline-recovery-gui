package main

import (
	"os"
	"strings"
	"testing"
)

// Source regression: ListView must query the native *single* selected row
// rather than applying amber to every row flagged by the custom draw DC.
func TestBrowserHighlightUsesActualSelectedListViewIndex(t *testing.T) {
	b, err := os.ReadFile("theme_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if n := strings.Count(s, "lvmGetNextItem, ^uintptr(0), lvniSelected"); n != 2 {
		t.Fatalf("expected native selected index check in prepaint and postpaint, got %d", n)
	}
	if !strings.Contains(s, "selectedIndex >= 0 && index == selectedIndex") {
		t.Fatal("non-selected ListView rows must keep the charcoal background")
	}
}
