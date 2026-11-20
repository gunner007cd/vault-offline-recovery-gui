package main

// mainLayout calculates the lower half of the native Windows window from the
// actual client height, rather than assuming its nominal creation size.
// Keeping this pure lets it be tested on non-Windows build hosts.
type mainLayout struct {
	ListHeight     int
	ActionsY       int
	CheckedLabelY  int
	CheckedLabelW  int
	EntriesY       int
	EntriesHeight  int
	DestY          int
	ConfirmY       int
	StatusY        int
	InspectX       int
	InspectWidth   int
	BrowserX       int
	BrowserWidth   int
	CompactButtons bool
}

// calculateMainLayout gives the room formerly used by the diagnostics pane
// back to the restore point lists and checked recovery paths. The status
// remains a single line anchored at the bottom of the window.
func calculateMainLayout(w, h, left, itemX, itemW int) mainLayout {
	const (
		listY            = 383
		bottomMargin     = 16
		buttonGap        = 10
		minListHeight    = 80
		minEntriesHeight = 58
		statusHeight     = 29
	)
	compact := itemW < 480
	additionalRow := 0
	if compact {
		additionalRow = 22
	}
	// The rows below the lists consume 5px spacing, 34px for actions,
	// 10px before destination, 35px before confirmation, 35px before status,
	// and the status line itself. The rest is available for the two lists.
	available := h - (listY + 5 + 34 + additionalRow + 10 + 35 + 35 + statusHeight + bottomMargin)
	listHeight := max(minListHeight, available*57/100)
	entriesHeight := max(minEntriesHeight, available-listHeight)
	actionsY := listY + listHeight + 5
	entriesY := actionsY + 34 + additionalRow
	destY := entriesY + entriesHeight + 10
	confirmY := destY + 35
	statusY := confirmY + 35
	result := mainLayout{
		ListHeight: listHeight, ActionsY: actionsY,
		CheckedLabelY: actionsY + 5, CheckedLabelW: itemX - left - 10,
		EntriesY: entriesY, EntriesHeight: entriesHeight,
		DestY: destY, ConfirmY: confirmY, StatusY: statusY,
		CompactButtons: compact,
	}
	if !compact {
		result.InspectX = itemX
		result.InspectWidth = min(255, (itemW-buttonGap)/2)
		result.BrowserX = result.InspectX + result.InspectWidth + buttonGap
		result.BrowserWidth = itemW - result.InspectWidth - buttonGap
	} else {
		result.InspectX = left
		result.InspectWidth = 250
		result.BrowserX = left + result.InspectWidth + buttonGap
		result.BrowserWidth = min(220, max(160, w-result.BrowserX-18))
		result.CheckedLabelY = actionsY + 36
		result.CheckedLabelW = w - 36
	}
	return result
}

// destinationRowGeometry keeps the restore destination near its label while
// retaining a full-width path field and the original right-hand Browse edge.
// It intentionally does not use the narrower portrait connection geometry.
type destinationRowGeometry struct {
	LabelWidth  int
	FieldX      int
	FieldWidth  int
	BrowseX     int
	BrowseWidth int
}

func calculateDestinationRowGeometry(clientWidth int) destinationRowGeometry {
	if clientWidth < 780 {
		clientWidth = 780
	}
	const (
		left             = 18
		labelWidth       = 160
		labelToFieldGap  = 10
		browseWidth      = 94
		fieldToBrowseGap = 8
		rightMargin      = 34
	)
	fieldX := left + labelWidth + labelToFieldGap
	browseX := clientWidth - rightMargin - browseWidth
	return destinationRowGeometry{
		LabelWidth:  labelWidth,
		FieldX:      fieldX,
		FieldWidth:  browseX - fieldToBrowseGap - fieldX,
		BrowseX:     browseX,
		BrowseWidth: browseWidth,
	}
}
