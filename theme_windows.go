//go:build windows

package main

// Restrained Unraid-inspired charcoal and orange Win32 theme.
// Only paints native controls; does not handle backup or restore data.
// No external assets, fonts, network calls, or dependencies are required.

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	wmEraseBkgnd        = 0x0014
	wmCtlColorEdit      = 0x0133
	wmCtlColorListbox   = 0x0134
	wmCtlColorBtn       = 0x0135
	wmCtlColorStatic    = 0x0138
	wmDrawItem          = 0x002b
	wmMeasureItem       = 0x002c
	bsOwnerDraw         = 0x000b
	odsSelected         = 0x0001
	odsDisabled         = 0x0004
	odsFocus            = 0x0010
	odtListbox          = 2
	lbGetText           = 0x0189
	lbGetTextLen        = 0x018a
	lbGetCount          = 0x018b
	lbItemFromPoint     = 0x01a9
	wmMouseMove         = 0x0200
	wmMouseLeave        = 0x02a3
	wmNcDestroy         = 0x0082
	tmeLeave            = 0x00000002
	cdDrawPrepaint      = 0x00000001
	cdDrawItemPrepaint  = 0x00010001
	cdDrawItemPostpaint = 0x00010002
	cdrfNotifyItemDraw  = 0x00000020
	cdrfNewFont         = 0x00000002
	cdrfNotifyPostpaint = 0x00000010
	cdisSelected        = 0x0001
	cdisHot             = 0x0040
	nmCustomDraw        = -12
	bkModeTransparent   = 1
	dtCenter            = 0x0001
	dtVCenter           = 0x0004
	dtSingleLine        = 0x0020
	dtEndEllipsis       = 0x8000
	odtButton           = 4
	odtStatic           = 5
	ssOwnerDraw         = 0x0000000d
	dtNoPrefix          = 0x0800
	rdwInvalidate       = 0x0001
	rdwErase            = 0x0004
	rdwAllChildren      = 0x0080
	tvmSetBkColor       = tvFirst + 29
	tvmSetTextColor     = tvFirst + 30
	tvmSetLineColor     = tvFirst + 40
	lvmSetBkColor       = lvmFirst + 1
	lvmSetTextColor     = lvmFirst + 36
	lvmSetTextBkColor   = lvmFirst + 38
)

var (
	dllMsimg32                = syscall.NewLazyDLL("msimg32.dll")
	dllDwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	dllUxTheme                = syscall.NewLazyDLL("uxtheme.dll")
	procGradientFill          = dllMsimg32.NewProc("GradientFill")
	procDwmSetWindowAttribute = dllDwmapi.NewProc("DwmSetWindowAttribute")
	procSetWindowTheme        = dllUxTheme.NewProc("SetWindowTheme")
	procCreateSolidBrush      = dllGdi32.NewProc("CreateSolidBrush")
	procSetBkMode             = dllGdi32.NewProc("SetBkMode")
	procSetTextColor          = dllGdi32.NewProc("SetTextColor")
	procSetBkColor            = dllGdi32.NewProc("SetBkColor")
	procFillRect              = dllUser32.NewProc("FillRect")
	procFrameRect             = dllUser32.NewProc("FrameRect")
	procDrawText              = dllUser32.NewProc("DrawTextW")
	procDrawFocusRect         = dllUser32.NewProc("DrawFocusRect")
	procRedrawWindow          = dllUser32.NewProc("RedrawWindow")
	procGetWindowRect         = dllUser32.NewProc("GetWindowRect")
	procGetParent             = dllUser32.NewProc("GetParent")
	procSelectObject          = dllGdi32.NewProc("SelectObject")
	procGetClientRect         = dllUser32.NewProc("GetClientRect")
	procGetStockObject        = dllGdi32.NewProc("GetStockObject")
	procInvalidateRect        = dllUser32.NewProc("InvalidateRect")
	procTrackMouseEvent       = dllUser32.NewProc("TrackMouseEvent")
	procSetWindowSubclass     = dllComctl32.NewProc("SetWindowSubclass")
	procRemoveWindowSubclass  = dllComctl32.NewProc("RemoveWindowSubclass")
	procDefSubclassProc       = dllComctl32.NewProc("DefSubclassProc")
	themeButtonCallback       uintptr
	themeListCallback         uintptr
	hoveredButtons            = make(map[uintptr]bool)
	hoveredLists              = make(map[uintptr]int)
)

type themeRect struct{ Left, Top, Right, Bottom int32 }
type themeVertex struct {
	X, Y                    int32
	Red, Green, Blue, Alpha uint16
}
type themeGradientRect struct{ First, Second uint32 }
type themeDrawItem struct {
	ControlType, ControlID, ItemID, ItemAction, ItemState uint32
	HwndItem, HDC                                         uintptr
	Rect                                                  themeRect
	ItemData                                              uintptr
}

// Native common-control custom draw structures (pointer-sized fields keep
// the Windows x64 layout; messages are copied from native memory via RtlMoveMemory).
type themeMeasureItem struct {
	ControlType uint32
	ControlID   uint32
	ItemID      uint32
	ItemWidth   uint32
	ItemHeight  uint32
	ItemData    uintptr
}

type themeCustomDraw struct {
	Header    notifyHeader
	DrawStage uint32
	HDC       uintptr
	Rect      themeRect
	ItemSpec  uintptr
	ItemState uint32
	ItemParam uintptr
}
type themeTreeCustomDraw struct {
	Base        themeCustomDraw
	TextColor   uint32
	TextBkColor uint32
	Level       int32
}
type themeListCustomDraw struct {
	Base        themeCustomDraw
	TextColor   uint32
	TextBkColor uint32
	SubItem     int32
}
type themeTrackMouseEvent struct {
	Size      uint32
	Flags     uint32
	HwndTrack uintptr
	HoverTime uint32
}

// The visual states are intentionally independent of recovery state. They
// never read a backup, change selected paths, or launch the Vault executable.
type visualRowState uint8

const (
	rowNormal visualRowState = iota
	rowHover
	rowSelected
)

func visualRowColor(state visualRowState) uintptr {
	switch state {
	case rowSelected:
		return shadeSelection
	case rowHover:
		return shadeHover
	default:
		return shadeInput
	}
}

func themeRGB(r, g, b uint8) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }

// Warm accent colors on near-neutral charcoal; orange is reserved for actions
// and section headings rather than large high-saturation panels.
var (
	shadeBackground = themeRGB(19, 20, 23)
	shadePanel      = themeRGB(30, 32, 36)
	shadeInput      = themeRGB(39, 41, 46)
	shadeText       = themeRGB(245, 246, 247)
	shadeInk        = themeRGB(251, 251, 252)
	shadeMuted      = themeRGB(182, 186, 192)
	shadeAccent     = themeRGB(255, 175, 107)
	shadeBorder     = themeRGB(80, 82, 88)
	shadeSelection  = themeRGB(111, 65, 36) // muted burnt amber, not Windows blue
	shadeHover      = themeRGB(56, 52, 48)  // softer than selection
	shadeFocus      = themeRGB(245, 147, 62)
	shadeDisabled   = themeRGB(139, 142, 147)
	brushBackground uintptr
	brushPanel      uintptr
	brushInput      uintptr
	brushBorder     uintptr
	brushAccent     uintptr
	brushSelection  uintptr
	brushHover      uintptr
	brushFocus      uintptr
)

var (
	headerStart = [3]uint8{50, 52, 57}
	headerEnd   = [3]uint8{32, 33, 37}
	bodyStart   = [3]uint8{34, 36, 40}
	bodyEnd     = [3]uint8{18, 19, 22}
)

func setupTheme() {
	themeButtonCallback = syscall.NewCallback(themeButtonSubclass)
	themeListCallback = syscall.NewCallback(themeListSubclass)
	makeBrush := func(c uintptr) uintptr { b, _, _ := procCreateSolidBrush.Call(c); return b }
	brushBackground = makeBrush(shadeBackground)
	brushPanel = makeBrush(shadePanel)
	brushInput = makeBrush(shadeInput)
	brushBorder = makeBrush(shadeBorder)
	brushAccent = makeBrush(shadeAccent)
	brushSelection = makeBrush(shadeSelection)
	brushHover = makeBrush(shadeHover)
	brushFocus = makeBrush(shadeFocus)
}
func releaseTheme() {
	for _, b := range []uintptr{brushBackground, brushPanel, brushInput, brushBorder, brushAccent, brushSelection, brushHover, brushFocus} {
		if b != 0 {
			procDeleteObject.Call(b)
		}
	}
}

func fillGradient(hdc uintptr, r themeRect, start, finish [3]uint8, vertical bool) {
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	cv := func(x uint8) uint16 { return uint16(x) * 257 }
	v := [2]themeVertex{
		{X: r.Left, Y: r.Top, Red: cv(start[0]), Green: cv(start[1]), Blue: cv(start[2])},
		{X: r.Right, Y: r.Bottom, Red: cv(finish[0]), Green: cv(finish[1]), Blue: cv(finish[2])},
	}
	mesh := themeGradientRect{First: 0, Second: 1}
	var mode uintptr
	if vertical {
		mode = 1
	}
	ok, _, _ := procGradientFill.Call(hdc, uintptr(unsafe.Pointer(&v[0])), 2, uintptr(unsafe.Pointer(&mesh)), 1, mode)
	if ok == 0 {
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brushPanel)
	}
	runtime.KeepAlive(v)
}

func fillThemeRect(hdc uintptr, r themeRect, brush uintptr) {
	if r.Right > r.Left && r.Bottom > r.Top {
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	}
}
func themeBackdrop(hwnd, hdc uintptr, browser bool) {
	var bounds themeRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
	if bounds.Right <= 0 || bounds.Bottom <= 0 {
		return
	}
	fillGradient(hdc, bounds, bodyStart, bodyEnd, true)
	const top int32 = 76
	fillGradient(hdc, themeRect{0, 0, bounds.Right, top}, headerStart, headerEnd, false)
	fillGradient(hdc, themeRect{0, 0, bounds.Right, 4}, [3]uint8{255, 157, 65}, [3]uint8{233, 84, 47}, false)
	fillThemeRect(hdc, themeRect{0, top, bounds.Right, top + 2}, brushAccent)
	if !browser {
		// Only the settings area is narrowed. Draw the larger mascot beside
		// those five fields; the title bar is now free of portrait artwork.
		paintAvatar(hdc, bounds.Right)
	}
	if browser {
		fillThemeRect(hdc, themeRect{18, 117, bounds.Right - 18, 118}, brushBorder)
	} else {
		fillThemeRect(hdc, themeRect{18, 332, bounds.Right - 18, 333}, brushBorder)
		fillThemeRect(hdc, themeRect{18, 514, bounds.Right - 18, 515}, brushBorder)
	}
}

// Dialogs have their own charcoal-and-amber backdrop. Do not call
// themeBackdrop here: it also paints the large mascot and the main window's
// section dividers, which caused the blue-ring "photobomb" inside the
// recovery-summary window.
func themeDialogBackdrop(hwnd, hdc uintptr) {
	var bounds themeRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
	if bounds.Right <= 0 || bounds.Bottom <= 0 {
		return
	}
	fillGradient(hdc, bounds, bodyStart, bodyEnd, true)
	const top int32 = 76
	fillGradient(hdc, themeRect{0, 0, bounds.Right, top}, headerStart, headerEnd, false)
	fillGradient(hdc, themeRect{0, 0, bounds.Right, 4}, [3]uint8{255, 157, 65}, [3]uint8{233, 84, 47}, false)
	fillThemeRect(hdc, themeRect{0, top, bounds.Right, top + 2}, brushAccent)
}

// Win32 paints owner-drawn STATICs with their own HDC. Paint the same gradient
// beneath each label in screen-aligned coordinates to avoid translucent ghosts
// and white label rectangles after maximize, restore, or rapid resizing.
func mixChannel(a, b uint8, position, extent int) uint8 {
	if extent <= 0 {
		return a
	}
	position = max(0, min(position, extent))
	return uint8((int(a)*(extent-position) + int(b)*position + extent/2) / extent)
}
func mixColor(a, b [3]uint8, position, extent int) [3]uint8 {
	return [3]uint8{mixChannel(a[0], b[0], position, extent), mixChannel(a[1], b[1], position, extent), mixChannel(a[2], b[2], position, extent)}
}
func labelBackground(hdc, child, parent uintptr, r themeRect) {
	var bounds, screen themeRect
	procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&bounds)))
	if bounds.Right <= 0 || bounds.Bottom <= 0 {
		fillThemeRect(hdc, r, brushPanel)
		return
	}
	procGetWindowRect.Call(child, uintptr(unsafe.Pointer(&screen)))
	pt := nativePoint{X: screen.Left, Y: screen.Top}
	procScreenToClient.Call(parent, uintptr(unsafe.Pointer(&pt)))
	x, y := int(pt.X), int(pt.Y)
	if y < 76 {
		// Header uses horizontal charcoal gradient; no input labels belong here.
		fillGradient(hdc, r, mixColor(headerStart, headerEnd, x, int(bounds.Right)), mixColor(headerStart, headerEnd, x+int(r.Right-r.Left), int(bounds.Right)), false)
		return
	}
	fillGradient(hdc, r, mixColor(bodyStart, bodyEnd, y, int(bounds.Bottom)), mixColor(bodyStart, bodyEnd, y+int(r.Bottom-r.Top), int(bounds.Bottom)), true)
}
func drawThemedLabel(lp uintptr, labels map[string]uintptr, parent, font, titleFont uintptr) bool {
	if lp == 0 {
		return false
	}
	var d themeDrawItem
	procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&d)), lp, unsafe.Sizeof(d))
	if d.ControlType != odtStatic {
		return false
	}
	key := ""
	for k, h := range labels {
		if d.HwndItem == h {
			key = k
			break
		}
	}
	if key == "" {
		return false
	}
	labelBackground(d.HDC, d.HwndItem, parent, d.Rect)
	fg := shadeText
	switch key {
	case "points", "items", "entries", "log", "status", "confirm", "tree", "files", "search", "summary":
		fg = shadeAccent
	case "hash", "mode":
		fg = shadeMuted
	}
	procSetBkMode.Call(d.HDC, bkModeTransparent)
	procSetTextColor.Call(d.HDC, fg)
	choose := font
	if key == "title" {
		choose = titleFont
	}
	if choose != 0 {
		old, _, _ := procSelectObject.Call(d.HDC, choose)
		defer procSelectObject.Call(d.HDC, old)
	}
	s := getText(d.HwndItem)
	chars := syscall.StringToUTF16(s)
	if len(chars) == 0 {
		return true
	}
	inner := d.Rect
	procDrawText.Call(d.HDC, uintptr(unsafe.Pointer(&chars[0])), ^uintptr(0), uintptr(unsafe.Pointer(&inner)), dtVCenter|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	runtime.KeepAlive(chars)
	return true
}
func requestThemeRedraw(hwnd uintptr) {
	if hwnd != 0 {
		procRedrawWindow.Call(hwnd, 0, 0, rdwInvalidate|rdwErase|rdwAllChildren)
	}
}

func activateDarkTitle(hwnd uintptr) {
	// Supported by recent Windows versions; failure is harmless on older PCs.
	use := uint32(1)
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&use)), uintptr(unsafe.Sizeof(use)))
}
func themeTreeAndFiles(tree, files uintptr) {
	// Disable OS theme-based blue selection painting on TreeView/ListView;
	// custom-draw now owns selected/hover foreground and background colors.
	// Otherwise, DarkMode_Explorer may override clrTextBk with system blue.
	empty := wide("")
	procSetWindowTheme.Call(tree, uintptr(unsafe.Pointer(empty)), uintptr(unsafe.Pointer(empty)))
	procSetWindowTheme.Call(files, uintptr(unsafe.Pointer(empty)), uintptr(unsafe.Pointer(empty)))
	send(tree, tvmSetBkColor, 0, shadeInput)
	send(tree, tvmSetTextColor, 0, shadeText)
	send(tree, tvmSetLineColor, 0, shadeBorder)
	send(files, lvmSetBkColor, 0, shadeInput)
	send(files, lvmSetTextColor, 0, shadeText)
	send(files, lvmSetTextBkColor, 0, shadeInput)
}

func themeCtlColor(hdc uintptr, kind uint32, foreground uintptr, disabled bool) uintptr {
	if disabled {
		foreground = shadeMuted
	}
	procSetTextColor.Call(hdc, foreground)
	switch kind {
	case wmCtlColorEdit, wmCtlColorListbox:
		procSetBkMode.Call(hdc, 2) // OPAQUE
		procSetBkColor.Call(hdc, shadeInput)
		return brushInput
	case wmCtlColorStatic:
		procSetBkMode.Call(hdc, bkModeTransparent)
		return brushPanel
	default:
		return brushPanel
	}
}
func mainControlColor(a *appState, kind uint32, hdc, child uintptr) uintptr {
	if kind == wmCtlColorEdit || kind == wmCtlColorListbox {
		return themeCtlColor(hdc, kind, shadeText, false)
	}
	// A disabled EDIT is drawn using WM_CTLCOLORSTATIC, not CTLCOLOREDIT.
	for _, id := range []int{idExecutable, idBackup, idConfig, idKey, idPass, idDestination, idConfirm, 500} {
		if a.fields[id] == child {
			procSetBkMode.Call(hdc, 2)
			procSetTextColor.Call(hdc, shadeMuted)
			procSetBkColor.Call(hdc, shadeInput)
			return brushInput
		}
	}
	fg := shadeMuted
	switch child {
	case a.labels["title"], a.labels["intro"]:
		fg = shadeInk
	case a.labels["points"], a.labels["items"], a.labels["entries"], a.labels["status"], a.labels["confirm"]:
		fg = shadeAccent
	case a.labels["hash"]:
		fg = shadeMuted
	default:
		fg = shadeText
	}
	return themeCtlColor(hdc, wmCtlColorStatic, fg, false)
}
func browserControlColor(b *fileBrowser, kind uint32, hdc, child uintptr) uintptr {
	if kind == wmCtlColorEdit || kind == wmCtlColorListbox {
		return themeCtlColor(hdc, kind, shadeText, false)
	}
	if child == b.controls[browserSearchID] {
		procSetBkMode.Call(hdc, 2)
		procSetBkColor.Call(hdc, shadeInput)
		procSetTextColor.Call(hdc, shadeMuted)
		return brushInput
	}
	fg := shadeText
	switch child {
	case b.labels["title"], b.labels["intro"]:
		fg = shadeInk
	case b.labels["tree"], b.labels["files"], b.labels["search"], b.labels["summary"]:
		fg = shadeAccent
	case b.labels["mode"]:
		fg = shadeMuted
	}
	return themeCtlColor(hdc, wmCtlColorStatic, fg, false)
}

// Fixed-height owner-drawn lists must supply a row height; otherwise Windows
// may choose a pre-font default that clips the Segoe UI row text.
func measureThemedListbox(lp uintptr) bool {
	if lp == 0 {
		return false
	}
	var m themeMeasureItem
	procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&m)), lp, unsafe.Sizeof(m))
	if m.ControlType != odtListbox {
		return false
	}
	m.ItemHeight = 23
	procCopyNativeMemory.Call(lp, uintptr(unsafe.Pointer(&m)), unsafe.Sizeof(m))
	return true
}

// LBS_OWNERDRAWFIXED | LBS_HASSTRINGS ensures list boxes retain their exact
// existing Windows listbox APIs, selection behavior, and item strings.
// Only the painting changes; we never modify item or recovery state here.
func drawThemedListbox(lp uintptr, matching func(uintptr) bool) bool {
	if lp == 0 {
		return false
	}
	var d themeDrawItem
	procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&d)), lp, unsafe.Sizeof(d))
	if d.ControlType != odtListbox || !matching(d.HwndItem) {
		return false
	}
	selected := d.ItemState&odsSelected != 0
	bg := brushInput
	if selected {
		bg = brushSelection
	} else if hoveredRow, tracked := hoveredLists[d.HwndItem]; tracked && hoveredRow == int(d.ItemID) && d.ItemID != ^uint32(0) {
		bg = brushHover
	}
	fillThemeRect(d.HDC, d.Rect, bg)
	if d.ItemID == ^uint32(0) { // empty listbox: no item text to draw
		return true
	}
	length := int(int32(send(d.HwndItem, lbGetTextLen, uintptr(d.ItemID), 0)))
	if length < 0 || length > 32768 {
		return true
	}
	buf := make([]uint16, length+1)
	if int32(send(d.HwndItem, lbGetText, uintptr(d.ItemID), uintptr(unsafe.Pointer(&buf[0])))) < 0 {
		return true
	}
	text := syscall.UTF16ToString(buf)
	chars := syscall.StringToUTF16(text)
	if len(chars) > 0 {
		procSetBkMode.Call(d.HDC, bkModeTransparent)
		if selected {
			procSetTextColor.Call(d.HDC, shadeInk) // white on warm selection
		} else {
			procSetTextColor.Call(d.HDC, shadeText)
		}
		r := d.Rect
		r.Left += 9
		r.Right -= 5
		procDrawText.Call(d.HDC, uintptr(unsafe.Pointer(&chars[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), dtVCenter|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		runtime.KeepAlive(chars)
	}
	if selected {
		// Bright orange left-hand focus rail gives a high-contrast selection
		// marker without turning the entire interface saturated orange.
		rail := d.Rect
		rail.Right = min(rail.Left+3, rail.Right)
		fillThemeRect(d.HDC, rail, brushFocus)
	}
	if d.ItemState&odsFocus != 0 {
		// The focus indication remains visible for keyboard navigation.
		r := d.Rect
		r.Left += 3
		r.Top++
		r.Right--
		r.Bottom--
		procDrawFocusRect.Call(d.HDC, uintptr(unsafe.Pointer(&r)))
	}
	return true
}

// Native Explorer tree/list controls request NM_CUSTOMDRAW before painting.
// Set BOTH foreground and background; Windows' system blue is avoided while
// listview checkbox selection and TreeView expand/collapse stay native.
func themeBrowserCustomDraw(lp uintptr, tree bool) (uintptr, bool) {
	if lp == 0 {
		return 0, false
	}
	var base themeCustomDraw
	procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&base)), lp, unsafe.Sizeof(base))
	if int32(base.Header.Code) != nmCustomDraw {
		return 0, false
	}
	switch base.DrawStage {
	case cdDrawPrepaint:
		return cdrfNotifyItemDraw, true
	case cdDrawItemPrepaint:
		// TreeView reports its own selection reliably. With checkbox ListView,
		// CDIS_SELECTED can be repeated during owner/custom painting on some
		// Windows themes, making every row look selected (even when every box
		// is UNCHECKED). Ask ListView which ONE row is actually highlighted.
		// The recovery checkbox state remains independent from this highlight.
		state := rowNormal
		selected := base.ItemState&cdisSelected != 0
		if !tree {
			index := int32(base.ItemSpec)
			selectedIndex := int32(send(base.Header.From, lvmGetNextItem, ^uintptr(0), lvniSelected))
			selected = selectedIndex >= 0 && index == selectedIndex
		}
		if selected {
			state = rowSelected
		} else if base.ItemState&cdisHot != 0 {
			state = rowHover
		}
		fg := uint32(shadeText)
		if state == rowSelected {
			fg = uint32(shadeInk)
		}
		bg := uint32(visualRowColor(state))
		if tree {
			var d themeTreeCustomDraw
			procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&d)), lp, unsafe.Sizeof(d))
			d.TextColor, d.TextBkColor = fg, bg
			procCopyNativeMemory.Call(lp, uintptr(unsafe.Pointer(&d)), unsafe.Sizeof(d))
		} else {
			var d themeListCustomDraw
			procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&d)), lp, unsafe.Sizeof(d))
			d.TextColor, d.TextBkColor = fg, bg
			procCopyNativeMemory.Call(lp, uintptr(unsafe.Pointer(&d)), unsafe.Sizeof(d))
		}
		return cdrfNewFont | cdrfNotifyPostpaint, true
	case cdDrawItemPostpaint:
		selected := base.ItemState&cdisSelected != 0
		if !tree {
			selectedIndex := int32(send(base.Header.From, lvmGetNextItem, ^uintptr(0), lvniSelected))
			selected = selectedIndex >= 0 && int32(base.ItemSpec) == selectedIndex
		}
		if selected {
			// Only selected rows receive the narrow orange focus edge;
			// hovering changes the fill gently without a selection rail.
			stripe := base.Rect
			stripe.Left += 1
			stripe.Right = min(stripe.Left+3, stripe.Right)
			fillThemeRect(base.HDC, stripe, brushFocus)
		}
		return 0, true
	}
	return 0, true
}

func installThemedButton(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	// If the platform lacks the optional subclass API, owner draw still works
	// with normal/pressed/disabled states. Hover is only decorative.
	procSetWindowSubclass.Call(hwnd, themeButtonCallback, 1, 0)
}
func themeButtonSubclass(hwnd uintptr, msg uint32, wp, lp, subclassID, data uintptr) uintptr {
	switch msg {
	case wmMouseMove:
		if !hoveredButtons[hwnd] {
			hoveredButtons[hwnd] = true
			track := themeTrackMouseEvent{Size: uint32(unsafe.Sizeof(themeTrackMouseEvent{})), Flags: tmeLeave, HwndTrack: hwnd}
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
			procInvalidateRect.Call(hwnd, 0, 0)
		}
	case wmMouseLeave:
		delete(hoveredButtons, hwnd)
		procInvalidateRect.Call(hwnd, 0, 0)
	case wmNcDestroy:
		delete(hoveredButtons, hwnd)
		procRemoveWindowSubclass.Call(hwnd, themeButtonCallback, subclassID)
	}
	ret, _, _ := procDefSubclassProc.Call(hwnd, uintptr(msg), wp, lp)
	return ret
}

// A listbox is still a native LISTBOX: this subclass does no selection or
// item editing. It only repaints the hovered row with a subdued amber tint.
func installThemedListbox(hwnd uintptr) {
	if hwnd != 0 {
		procSetWindowSubclass.Call(hwnd, themeListCallback, 2, 0)
	}
}
func themeListSubclass(hwnd uintptr, msg uint32, wp, lp, subclassID, data uintptr) uintptr {
	switch msg {
	case wmMouseMove:
		// LB_ITEMFROMPOINT returns the row in LOWORD, outside flag in HIWORD.
		hit := send(hwnd, lbItemFromPoint, 0, lp)
		idx := int(uint16(hit))
		if uint16(hit>>16) != 0 || idx >= int(send(hwnd, lbGetCount, 0, 0)) {
			idx = -1
		}
		old, tracked := hoveredLists[hwnd]
		if !tracked || old != idx {
			hoveredLists[hwnd] = idx
			if !tracked {
				track := themeTrackMouseEvent{Size: uint32(unsafe.Sizeof(themeTrackMouseEvent{})), Flags: tmeLeave, HwndTrack: hwnd}
				procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
			}
			procInvalidateRect.Call(hwnd, 0, 0)
		}
	case wmMouseLeave:
		delete(hoveredLists, hwnd)
		procInvalidateRect.Call(hwnd, 0, 0)
	case wmNcDestroy:
		delete(hoveredLists, hwnd)
		procRemoveWindowSubclass.Call(hwnd, themeListCallback, subclassID)
	}
	ret, _, _ := procDefSubclassProc.Call(hwnd, uintptr(msg), wp, lp)
	return ret
}

func themeDrawButton(lp uintptr) bool {
	if lp == 0 {
		return false
	}
	var d themeDrawItem
	procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&d)), lp, unsafe.Sizeof(d))
	if d.ControlType != odtButton {
		return false
	}
	id := int(d.ControlID)
	primary := id == idRestore || id == idScan || id == idInspect || id == idOpenBrowser || id == browserDoneID || id == idPreflightYes
	disabled := d.ItemState&odsDisabled != 0
	pressed := d.ItemState&odsSelected != 0
	hover := !disabled && hoveredButtons[d.HwndItem]
	var from, to [3]uint8
	switch {
	case disabled:
		from, to = [3]uint8{49, 50, 54}, [3]uint8{42, 43, 47}
	case primary && pressed:
		from, to = [3]uint8{192, 99, 46}, [3]uint8{160, 71, 37}
	case primary && hover:
		from, to = [3]uint8{244, 159, 79}, [3]uint8{218, 103, 49}
	case primary:
		from, to = [3]uint8{226, 143, 67}, [3]uint8{195, 91, 44}
	case pressed:
		from, to = [3]uint8{66, 55, 49}, [3]uint8{52, 47, 46}
	case hover:
		from, to = [3]uint8{70, 65, 60}, [3]uint8{58, 56, 54}
	default:
		from, to = [3]uint8{58, 60, 64}, [3]uint8{45, 47, 51}
	}
	r := d.Rect
	fillGradient(d.HDC, r, from, to, false)
	frame := brushBorder
	if !disabled && (hover || primary || pressed) {
		frame = brushFocus
	}
	procFrameRect.Call(d.HDC, uintptr(unsafe.Pointer(&r)), frame)
	text := getText(d.HwndItem)
	chars := syscall.StringToUTF16(text)
	if len(chars) > 0 {
		procSetBkMode.Call(d.HDC, bkModeTransparent)
		switch {
		case disabled:
			procSetTextColor.Call(d.HDC, shadeDisabled)
		case primary:
			procSetTextColor.Call(d.HDC, shadeInk)
		default:
			procSetTextColor.Call(d.HDC, shadeText)
		}
		inner := r
		inner.Left += 5
		inner.Right -= 5
		if pressed {
			inner.Top++
			inner.Bottom++
		}
		procDrawText.Call(d.HDC, uintptr(unsafe.Pointer(&chars[0])), ^uintptr(0), uintptr(unsafe.Pointer(&inner)), dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		runtime.KeepAlive(chars)
	}
	if d.ItemState&odsFocus != 0 && !disabled {
		focus := r
		focus.Left += 3
		focus.Top += 3
		focus.Right -= 3
		focus.Bottom -= 3
		procDrawFocusRect.Call(d.HDC, uintptr(unsafe.Pointer(&focus)))
	}
	return true
}
