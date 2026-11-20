package main

import "testing"

func TestPortraitDoesNotOverlapSettingsOrActions(t *testing.T) {
	for _, width := range []int{780, 800, 900, 1100, 1366, 1651, 1920, 2560} {
		g := calculateTopFormGeometry(width)
		if g.FieldWidth < 235 {
			t.Errorf("width %d: field too narrow: %+v", width, g)
		}
		if g.FieldX+g.FieldWidth+8 != g.BrowseX {
			t.Errorf("width %d: form field and Browse not aligned: %+v", width, g)
		}
		if g.BrowseX+g.BrowseWidth+22 > g.PortraitX {
			t.Errorf("width %d: browse button and portrait overlap: %+v", width, g)
		}
		if g.PortraitX+g.PortraitSize > width-18 {
			t.Errorf("width %d: portrait clipped on right: %+v", width, g)
		}
		if g.PortraitSize < 96 || g.PortraitSize > 192 || g.PortraitY < 100 || g.PortraitY+g.PortraitSize >= 301 {
			t.Errorf("width %d: portrait overlaps header or lower controls: %+v", width, g)
		}
	}
}

func TestWideWindowHasLargePortraitAndShorterPaths(t *testing.T) {
	const width = 1651
	g := calculateTopFormGeometry(width)
	originalFieldWidth := width - g.FieldX - 94 - 2*12 - 18
	if g.PortraitSize != 192 || g.FieldWidth >= originalFieldWidth-160 {
		t.Fatalf("portrait isn't larger and paths aren't shortened: %+v, original %d", g, originalFieldWidth)
	}
	if g.PortraitY+g.PortraitSize > 295 {
		t.Fatalf("portrait too low: %+v", g)
	}
}
