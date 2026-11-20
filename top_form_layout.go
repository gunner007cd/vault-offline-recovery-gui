package main

// topFormGeometry reserves a right-side portrait column only alongside the
// connection settings. All restore-point, item, checked-path and diagnostic
// panels retain their existing full-width layout.
type topFormGeometry struct {
	FieldX       int
	FieldWidth   int
	BrowseX      int
	BrowseWidth  int
	PortraitX    int
	PortraitY    int
	PortraitSize int
}

func calculateTopFormGeometry(clientWidth int) topFormGeometry {
	// Values are in the same logical pixel units as the existing native UI.
	const (
		left              = 18
		labelWidth        = 256
		fieldToBrowse     = 8
		buttonWidth       = 94
		browseToPortrait  = 22
		rightMargin       = 22
		preferredPortrait = 192
		minimumFieldWidth = 235
		minimumPortrait   = 96
	)
	if clientWidth < 780 {
		clientWidth = 780
	}
	fieldX := left + labelWidth
	maxPortrait := clientWidth - rightMargin - browseToPortrait - buttonWidth - fieldToBrowse - fieldX - minimumFieldWidth
	portraitSize := max(minimumPortrait, min(preferredPortrait, maxPortrait))
	portraitX := clientWidth - rightMargin - portraitSize
	browseX := portraitX - browseToPortrait - buttonWidth
	fieldWidth := browseX - fieldToBrowse - fieldX

	// Vertically center smaller portraits in the settings block. At full
	// size the portrait spans y=100..292, leaving space before y=301 actions.
	portraitY := 100 + (preferredPortrait-portraitSize)/2
	return topFormGeometry{
		FieldX: fieldX, FieldWidth: fieldWidth,
		BrowseX: browseX, BrowseWidth: buttonWidth,
		PortraitX: portraitX, PortraitY: portraitY, PortraitSize: portraitSize,
	}
}
