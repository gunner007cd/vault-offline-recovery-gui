package main

import "testing"

func TestMainLayoutReclaimsConsoleSpace(t *testing.T) {
	for _, tc := range []struct{ w, h int }{
		{1650, 750}, {1650, 800}, {1650, 830}, {1650, 850}, {1650, 900},
		{1100, 800}, {1100, 850}, {900, 800}, {780, 800}, {780, 850},
		{1920, 1020},
	} {
		left := 18
		itemX := left + (tc.w-40)*2/3 + 8
		itemW := tc.w - itemX - 18
		g := calculateMainLayout(tc.w, tc.h, left, itemX, itemW)
		if g.BrowserX < g.InspectX+g.InspectWidth+8 || g.BrowserX+g.BrowserWidth > tc.w-16 {
			t.Errorf("%dx%d: buttons overlap or clip: %+v", tc.w, tc.h, g)
		}
		if g.EntriesY < g.ActionsY+29 || g.DestY < g.EntriesY+g.EntriesHeight || g.ConfirmY < g.DestY+26 {
			t.Errorf("%dx%d: controls overlap: %+v", tc.w, tc.h, g)
		}
		if g.StatusY+29 > tc.h-12 {
			t.Errorf("%dx%d: status offscreen: %+v", tc.w, tc.h, g)
		}
		if g.EntriesHeight < 58 || g.ListHeight < 80 {
			t.Errorf("%dx%d: lists too short: %+v", tc.w, tc.h, g)
		}
		if !g.CompactButtons && g.CheckedLabelY+20 > g.EntriesY {
			t.Errorf("%dx%d: checked-path heading overlaps selection", tc.w, tc.h)
		}
	}
}

func TestExtraHeightGoesToActualLists(t *testing.T) {
	const w, left = 1650, 18
	itemX := left + (w-40)*2/3 + 8
	itemW := w - itemX - 18
	small := calculateMainLayout(w, 800, left, itemX, itemW)
	large := calculateMainLayout(w, 900, left, itemX, itemW)
	if large.ListHeight <= small.ListHeight || large.EntriesHeight <= small.EntriesHeight {
		t.Fatalf("additional height not allocated to lists: small=%+v large=%+v", small, large)
	}
	if large.EntriesHeight < 100 || large.ListHeight < 150 {
		t.Fatalf("list areas should be more spacious without console: %+v", large)
	}
}

func TestDestinationRowIsNearLabelAndFullWidth(t *testing.T) {
	for _, width := range []int{780, 900, 1100, 1650, 1920} {
		g := calculateDestinationRowGeometry(width)
		if gap := g.FieldX - (18 + g.LabelWidth); gap < 6 || gap > 15 {
			t.Errorf("width %d: destination field label gap = %d, want 6..15", width, gap)
		}
		if g.FieldWidth < 300 {
			t.Errorf("width %d: destination field too narrow: %+v", width, g)
		}
		if g.FieldX+g.FieldWidth+8 != g.BrowseX || g.BrowseX+g.BrowseWidth != width-34 {
			t.Errorf("width %d: destination field / Browse button misaligned: %+v", width, g)
		}
	}
}
