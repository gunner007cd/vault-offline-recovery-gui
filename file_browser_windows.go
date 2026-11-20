//go:build windows

package main

// Native Explorer-style checkbox browser. All paths come from the previous
// `vault recover contents --json` response. The browser never reads backups.

import (
	"fmt"
	"path"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	wmNotify                    = 0x004e
	wmAppToggleTree             = 0x8020
	lbSetHorizontalExtent       = 0x0194
	tvFirst                     = 0x1100
	tvmInsertItemW              = tvFirst + 50
	tvmGetNextItem              = tvFirst + 10
	tvmSelectItem               = tvFirst + 11
	tvmExpand                   = tvFirst + 2
	tvmGetItemW                 = tvFirst + 62
	tvmSetItemW                 = tvFirst + 63
	tvmHitTest                  = tvFirst + 17
	tvgnCaret                   = 9
	tveExpand                   = 2
	tvifText                    = 1
	tvifState                   = 8
	tvisStateImageMask          = 0xF000
	tvhtOnItemStateIcon         = 0x0040
	tvsHasButtons               = 1
	tvsHasLines                 = 2
	tvsLinesAtRoot              = 4
	tvsShowSelAlways            = 0x0020
	tvsCheckBoxes               = 0x0100
	iccTreeViewClasses          = 2
	iccListViewClasses          = 1
	lvsReport                   = 1
	lvsSingleSel                = 4
	lvsShowSelAlways            = 8
	lvsExCheckBoxes             = 4
	lvsExFullRowSelect          = 0x20
	lvmFirst                    = 0x1000
	lvmDeleteAllItems           = lvmFirst + 9
	lvmGetNextItem              = lvmFirst + 12
	lvmSetColumnWidth           = lvmFirst + 30
	lvmSetItemState             = lvmFirst + 43
	lvmSetExtendedListViewStyle = lvmFirst + 54
	lvmInsertItemW              = lvmFirst + 77
	lvmInsertColumnW            = lvmFirst + 97
	lvmSetItemTextW             = lvmFirst + 116
	lvisStateImageMask          = 0xF000
	lvifText                    = 1
	lvifState                   = 8
	lvcfFmt                     = 1
	lvcfWidth                   = 2
	lvcfText                    = 4
	lvniSelected                = 2
	lvnItemChanged              = -101
	tvnSelChangedW              = -451
	tvnKeyDown                  = -412
	nmClick                     = -2
	nmDblClick                  = -3
	browserTreeID               = 1101
	browserFilesID              = 1102
	browserSearchID             = 1103
	browserMarkedID             = 1104
	browserRemoveID             = 1113
	browserClearID              = 1114
	browserDoneID               = 1115
	browserSearchClearID        = 1116
)

var (
	dllComctl32              = syscall.NewLazyDLL("comctl32.dll")
	procInitCommonControlsEx = dllComctl32.NewProc("InitCommonControlsEx")
	procCopyNativeMemory     = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlMoveMemory")
	procGetCursorPos         = dllUser32.NewProc("GetCursorPos")
	procScreenToClient       = dllUser32.NewProc("ScreenToClient")
	activeBrowser            *fileBrowser
	browserRegistered        bool
)

type initCommonControlsEx struct {
	Size    uint32
	Classes uint32
}
type tvItemW struct {
	Mask          uint32
	Handle        uintptr
	State         uint32
	StateMask     uint32
	Text          *uint16
	TextMax       int32
	Image         int32
	SelectedImage int32
	Children      int32
	Param         uintptr
}
type tvInsertStructW struct {
	Parent          uintptr
	After           uintptr
	Item            tvItemW
	ExtendedPadding [48]byte
}
type nativePoint struct{ X, Y int32 }
type tvHitTestInfo struct {
	Point  nativePoint
	Flags  uint32
	Handle uintptr
}
type notifyHeader struct {
	From uintptr
	ID   uintptr
	Code uint32
}
type tvKeyDown struct {
	Header notifyHeader
	Key    uint16
	Flags  uint16
}
type nmListView struct {
	Header   notifyHeader
	Item     int32
	SubItem  int32
	NewState uint32
	OldState uint32
	Changed  uint32
	Action   nativePoint
	Param    uintptr
}
type lvColumnW struct {
	Mask         uint32
	Fmt          int32
	Width        int32
	Text         *uint16
	TextMax      int32
	SubItem      int32
	Image        int32
	Order        int32
	MinWidth     int32
	DefaultWidth int32
	IdealWidth   int32
}
type lvItemW struct {
	Mask      uint32
	Item      int32
	SubItem   int32
	State     uint32
	StateMask uint32
	Text      *uint16
	TextMax   int32
	Image     int32
	Param     uintptr
	Indent    int32
	GroupID   int32
	Columns   uint32
	ColFmt    *int32
	Group     int32
}

type fileBrowser struct {
	hwnd          uintptr
	controls      map[int]uintptr
	labels        map[string]uintptr
	model         *browserModel
	main          *appState
	shown         []browserRow
	chosen        []string
	treePaths     map[uintptr]string
	treeHandles   map[string]uintptr
	folder        string
	pendingToggle string
	syncing       bool
	populating    bool
	w, h          int
}

func registerBrowserClass() error {
	if browserRegistered {
		return nil
	}
	init := initCommonControlsEx{Size: uint32(unsafe.Sizeof(initCommonControlsEx{})), Classes: iccTreeViewClasses | iccListViewClasses}
	ok, _, err := procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&init)))
	if ok == 0 {
		return fmt.Errorf("initialize Windows tree/list controls: %v", err)
	}
	instance, _, _ := procGetModule.Call(0)
	cursor, _, _ := procLoadCursor.Call(0, 32512)
	largeIcon, smallIcon := appWindowIcons(instance)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: csHredraw | csVredraw,
		WndProc: syscall.NewCallback(browserWindowProc), Instance: instance, Icon: largeIcon, IconSm: smallIcon, Cursor: cursor,
		Background: 0, ClassName: wide("VaultOfflineCheckboxBrowser")}
	ok, _, err = procRegClass.Call(uintptr(unsafe.Pointer(&wc)))
	if ok == 0 {
		return fmt.Errorf("register checkbox browser: %v", err)
	}
	browserRegistered = true
	return nil
}

func (a *appState) showBrowser() {
	if a.browserModel == nil || a.inspectedSig != a.inspectionSignature() || a.busy {
		return
	}
	if activeBrowser != nil && activeBrowser.hwnd != 0 {
		procShow.Call(activeBrowser.hwnd, swShow)
		dllUser32.NewProc("SetForegroundWindow").Call(activeBrowser.hwnd)
		return
	}
	if err := registerBrowserClass(); err != nil {
		a.showInputError(err.Error())
		return
	}
	b := &fileBrowser{model: a.browserModel, main: a, controls: make(map[int]uintptr), labels: make(map[string]uintptr),
		treePaths: make(map[uintptr]string), treeHandles: make(map[string]uintptr), w: 1180, h: 810}
	activeBrowser = b
	instance, _, _ := procGetModule.Call(0)
	hwnd, _, err := procCreate.Call(0, uintptr(unsafe.Pointer(wide("VaultOfflineCheckboxBrowser"))),
		uintptr(unsafe.Pointer(wide("Vault Backup File Browser — Check files and folders to restore"))),
		wsOverlappedWindow|wsClipChildren, 140, 95, uintptr(b.w), uintptr(b.h), a.hwnd, 0, instance, 0)
	if hwnd == 0 {
		activeBrowser = nil
		a.showInputError("Cannot open file browser: " + err.Error())
		return
	}
	procShow.Call(hwnd, swShow)
	procUpdate.Call(hwnd)
}
func (a *appState) closeBrowser() {
	if activeBrowser != nil && activeBrowser.hwnd != 0 {
		procDestroy.Call(activeBrowser.hwnd)
	}
}
func (b *fileBrowser) control(id int, class, title string, style uint32) uintptr {
	h := classCreate(class, title, style|wsChild|wsVisible, 0, 0, 60, 24, b.hwnd, id)
	b.controls[id] = h
	if b.main.font != 0 {
		send(h, wmSetFont, b.main.font, 1)
	}
	if class == "BUTTON" {
		installThemedButton(h)
	} else if class == "LISTBOX" {
		installThemedListbox(h)
	}
	return h
}
func (b *fileBrowser) label(name, title string) {
	h := classCreate("STATIC", title, wsChild|wsVisible|ssOwnerDraw, 0, 0, 60, 20, b.hwnd, 0)
	b.labels[name] = h
	if b.main.font != 0 {
		send(h, wmSetFont, b.main.font, 1)
	}
}
func (b *fileBrowser) selectedIndices(h uintptr) []int {
	n := int(int32(send(h, lbGetSelCount, 0, 0)))
	if n <= 0 {
		return nil
	}
	buf := make([]int32, n)
	count := int(int32(send(h, lbGetSelItems, uintptr(n), uintptr(unsafe.Pointer(&buf[0])))))
	out := make([]int, 0, max(0, count))
	for _, idx := range buf[:max(0, min(count, len(buf)))] {
		if idx >= 0 {
			out = append(out, int(idx))
		}
	}
	return out
}
func (b *fileBrowser) addTreeFolder(parent uintptr, folder, label string) uintptr {
	str := wide(label)
	ins := tvInsertStructW{Parent: parent, After: ^uintptr(1), Item: tvItemW{Mask: tvifText, Text: str}}
	h := send(b.controls[browserTreeID], tvmInsertItemW, 0, uintptr(unsafe.Pointer(&ins)))
	runtime.KeepAlive(str)
	if h != 0 {
		b.treePaths[h] = folder
		b.treeHandles[folder] = h
	}
	return h
}
func (b *fileBrowser) appendFolders(folder string, h uintptr) {
	for _, child := range b.model.folderChildren[folder] {
		sub := b.addTreeFolder(h, child, path.Base(child))
		if sub != 0 {
			b.appendFolders(child, sub)
		}
	}
}
func (b *fileBrowser) insertColumn(index int, heading string, width int) {
	c := lvColumnW{Mask: lvcfFmt | lvcfWidth | lvcfText, Width: int32(width), Text: wide(heading)}
	send(b.controls[browserFilesID], lvmInsertColumnW, uintptr(index), uintptr(unsafe.Pointer(&c)))
}
func (b *fileBrowser) createContents() {
	b.label("title", "Browse your backup — tick checkboxes to select recovery paths")
	send(b.labels["title"], wmSetFont, b.main.titleFont, 1)
	b.label("intro", "Checking a folder includes everything inside it. To choose fewer files, uncheck the parent folder first. Browsing remains read-only.")
	b.label("search", "Search every indexed file or folder:")
	b.label("tree", "Folder tree — checkbox = entire folder (root = whole item)")
	b.label("files", "Files and folders — tick boxes to restore")
	b.label("summary", "")
	b.label("mode", "Nothing selected until you check a file, folder, or the entire item.")
	b.control(browserSearchID, "EDIT", "", wsBorder|wsTabstop|esAutoHscroll)
	b.control(browserSearchClearID, "BUTTON", "Clear search", wsTabstop|bsOwnerDraw)
	b.control(browserTreeID, "SysTreeView32", "", wsBorder|wsTabstop|wsVscroll|tvsHasButtons|tvsHasLines|tvsLinesAtRoot|tvsShowSelAlways|tvsCheckBoxes)
	b.control(browserFilesID, "SysListView32", "", wsBorder|wsTabstop|wsVscroll|lvsReport|lvsSingleSel|lvsShowSelAlways)
	themeTreeAndFiles(b.controls[browserTreeID], b.controls[browserFilesID])
	send(b.controls[browserFilesID], lvmSetExtendedListViewStyle, 0, lvsExCheckBoxes|lvsExFullRowSelect)
	b.insertColumn(0, "Name", 660)
	b.insertColumn(1, "Size", 130)
	b.control(browserMarkedID, "LISTBOX", "", wsBorder|wsTabstop|wsVscroll|lbsExtended|lbsNoIntegral|lbsOwnerDrawFixed|lbsHasStrings)
	b.control(browserRemoveID, "BUTTON", "Uncheck highlighted", wsTabstop|bsOwnerDraw)
	b.control(browserClearID, "BUTTON", "Uncheck all", wsTabstop|bsOwnerDraw)
	b.control(browserDoneID, "BUTTON", "Done - keep checked paths", wsTabstop|bsOwnerDraw)
	b.populating = true
	root := b.addTreeFolder(0, "", "Entire backup item")
	if root == 0 {
		setText(b.labels["intro"], "Cannot populate folder tree; close and reopen the browser.")
	} else {
		b.appendFolders("", root)
		send(b.controls[browserTreeID], tvmExpand, tveExpand, root)
		send(b.controls[browserTreeID], tvmSelectItem, tvgnCaret, root)
	}
	b.populating = false
	b.folder = ""
	b.refreshFiles()
	b.refreshChosen()
	b.layout()
}
func (b *fileBrowser) refreshFiles() {
	query := strings.TrimSpace(getText(b.controls[browserSearchID]))
	if query != "" {
		b.shown = b.model.search(query)
	} else {
		b.shown = b.model.children(b.folder)
	}
	hwnd := b.controls[browserFilesID]
	b.syncing = true
	send(hwnd, lvmDeleteAllItems, 0, 0)
	for i, row := range b.shown {
		name := row.Name
		if query != "" {
			name = row.Path
		}
		if row.IsDir {
			name = "[Folder]  " + name
		}
		it := lvItemW{Mask: lvifText, Item: int32(i), Text: wide(name)}
		send(hwnd, lvmInsertItemW, 0, uintptr(unsafe.Pointer(&it)))
		detail := "Folder"
		if !row.IsDir {
			detail = sizeLabel(row.Size)
		}
		sub := lvItemW{SubItem: 1, Text: wide(detail)}
		send(hwnd, lvmSetItemTextW, uintptr(i), uintptr(unsafe.Pointer(&sub)))
	}
	b.syncing = false
	b.refreshChecks()
	if query != "" {
		setText(b.labels["files"], fmt.Sprintf("Search matches: %d paths", len(b.shown)))
	} else {
		where := b.folder
		if where == "" {
			where = "(item root)"
		}
		setText(b.labels["files"], fmt.Sprintf("%s — %d files/folders", where, len(b.shown)))
	}
}
func sizeLabel(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	f := float64(size)
	for _, u := range units {
		f /= 1024
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%.1f PB", f/1024)
}
func (b *fileBrowser) refreshChecks() {
	b.syncing = true
	defer func() { b.syncing = false }()
	for p, h := range b.treeHandles {
		state := uint32(1 << 12)
		if b.model.checked(p) {
			state = 2 << 12
		}
		item := tvItemW{Mask: tvifState, Handle: h, State: state, StateMask: tvisStateImageMask}
		send(b.controls[browserTreeID], tvmSetItemW, 0, uintptr(unsafe.Pointer(&item)))
	}
	hwnd := b.controls[browserFilesID]
	for i, row := range b.shown {
		state := uint32(1 << 12)
		if b.model.checked(row.Path) {
			state = 2 << 12
		}
		item := lvItemW{Mask: lvifState, State: state, StateMask: lvisStateImageMask}
		send(hwnd, lvmSetItemState, uintptr(i), uintptr(unsafe.Pointer(&item)))
	}
}
func (b *fileBrowser) refreshChosen() {
	b.chosen = b.model.includes()
	h := b.controls[browserMarkedID]
	send(h, lbResetContent, 0, 0)
	if b.model.wholeItem {
		send(h, lbAddString, 0, uintptr(unsafe.Pointer(wide("[Whole item] Entire backup item"))))
		setText(b.labels["summary"], "ENTIRE ITEM explicitly checked. All paths will be recovered after separate RESTORE confirmation.")
	} else {
		for _, p := range b.chosen {
			typeLabel := "[File]   "
			if row, ok := b.model.byPath[p]; ok && row.IsDir {
				typeLabel = "[Folder] "
			}
			send(h, lbAddString, 0, uintptr(unsafe.Pointer(wide(typeLabel+p))))
		}
		if len(b.chosen) == 0 {
			setText(b.labels["summary"], "Nothing selected — restore is disabled until you check a file, folder, or entire item.")
		} else {
			setText(b.labels["summary"], fmt.Sprintf("Checked for recovery: %d path(s). Selected folders automatically include descendants.", len(b.chosen)))
		}
	}
	send(h, lbSetHorizontalExtent, 1500, 0)
	if b.model.ready() {
		setText(b.labels["mode"], "Close this window to return to restore controls. Confirmation is still required.")
	} else {
		setText(b.labels["mode"], "Browse freely. Checking boxes only prepares a selection; nothing is restored here.")
	}
	b.main.refreshMainSelection()
}
func (b *fileBrowser) toggle(p string) {
	if err := b.model.toggle(p); err != nil {
		msgBox(b.hwnd, err.Error(), "Folder selection", mbOK|mbIconInfo)
	} else {
		b.refreshChosen()
	}
	b.refreshChecks()
}
func (b *fileBrowser) removeMarked() {
	if b.model.wholeItem {
		if len(b.selectedIndices(b.controls[browserMarkedID])) > 0 {
			b.model.remove("")
		}
	} else {
		for _, i := range b.selectedIndices(b.controls[browserMarkedID]) {
			if i >= 0 && i < len(b.chosen) {
				b.model.remove(b.chosen[i])
			}
		}
	}
	b.refreshChosen()
	b.refreshChecks()
}
func (b *fileBrowser) clearMarked() {
	if b.model.ready() && msgBox(b.hwnd, "Uncheck all recovery paths?\n\nWith no boxes checked, restore is disabled.", "Clear recovery selection?", mbYesNo|mbIconWarning|mbDefButton2) != idYes {
		return
	}
	b.model.clear()
	b.refreshChosen()
	b.refreshChecks()
}
func (b *fileBrowser) navigateSelectedFile() {
	idx := int(int32(send(b.controls[browserFilesID], lvmGetNextItem, ^uintptr(0), lvniSelected)))
	if idx < 0 || idx >= len(b.shown) {
		return
	}
	row := b.shown[idx]
	if !row.IsDir {
		return
	}
	setText(b.controls[browserSearchID], "")
	if h := b.treeHandles[row.Path]; h != 0 {
		send(b.controls[browserTreeID], tvmSelectItem, tvgnCaret, h)
	}
}
func (b *fileBrowser) layout() {
	w, h := b.w, b.h
	if w < 810 {
		w = 810
	}
	if h < 620 {
		h = 620
	}
	margin := 18
	treeW := max(290, min(470, w*34/100))
	rightX := margin + treeW + 14
	rightW := w - rightX - margin
	topY := 154
	baseY := h - 240
	move(b.labels["title"], margin, 10, w-36, 35)
	move(b.labels["intro"], margin, 47, w-36, 22)
	move(b.labels["search"], margin, 79, 320, 22)
	move(b.controls[browserSearchID], margin+320, 75, w-36-320-126, 28)
	move(b.controls[browserSearchClearID], w-132, 75, 114, 28)
	move(b.labels["tree"], margin, 124, treeW, 23)
	move(b.labels["files"], rightX, 124, rightW, 23)
	move(b.controls[browserTreeID], margin, topY, treeW, max(100, baseY-topY))
	move(b.controls[browserFilesID], rightX, topY, rightW, max(100, baseY-topY))
	send(b.controls[browserFilesID], lvmSetColumnWidth, 0, uintptr(max(140, rightW-145)))
	send(b.controls[browserFilesID], lvmSetColumnWidth, 1, 135)
	move(b.labels["summary"], margin, baseY+10, w-36, 22)
	move(b.controls[browserMarkedID], margin, baseY+36, w-36, max(50, h-baseY-103))
	move(b.controls[browserRemoveID], margin, h-49, 166, 30)
	move(b.controls[browserClearID], margin+176, h-49, 130, 30)
	move(b.labels["mode"], margin+315, h-46, max(80, w-315-260), 23)
	move(b.controls[browserDoneID], w-253, h-49, 235, 30)
}
func notificationCode(n notifyHeader) int32 { return int32(n.Code) }
func browserWindowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	b := activeBrowser
	if b == nil {
		r, _, _ := procDefWin.Call(hwnd, uintptr(msg), wp, lp)
		return r
	}
	switch msg {
	case wmCreate:
		b.hwnd = hwnd
		activateDarkTitle(hwnd)
		b.createContents()
		return 0
	case wmEraseBkgnd:
		themeBackdrop(hwnd, wp, true)
		return 1
	case wmCtlColorEdit, wmCtlColorListbox, wmCtlColorStatic, wmCtlColorBtn:
		return browserControlColor(b, msg, wp, lp)
	case wmMeasureItem:
		if measureThemedListbox(lp) {
			return 1
		}
	case wmDrawItem:
		if drawThemedLabel(lp, b.labels, hwnd, b.main.font, b.main.titleFont) ||
			drawThemedListbox(lp, func(h uintptr) bool { return h == b.controls[browserMarkedID] }) ||
			themeDrawButton(lp) {
			return 1
		}
	case wmSize:
		b.w = int(uint16(lp & 0xffff))
		b.h = int(uint16((lp >> 16) & 0xffff))
		b.layout()
		requestThemeRedraw(hwnd)
		return 0
	case wmAppToggleTree:
		p := b.pendingToggle
		b.pendingToggle = ""
		// The native TreeView finishes its own checkbox handling before this
		// posted message. Always reconcile the visual state from the model.
		if p != "\x00" && p != "" {
			b.toggle(p)
		} else if p == "\x00" {
			b.toggle("")
		}
		return 0
	case wmNotify:
		if lp != 0 {
			var nh notifyHeader
			procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&nh)), lp, unsafe.Sizeof(nh))
			code := notificationCode(nh)
			if code == nmCustomDraw {
				if nh.From == b.controls[browserTreeID] {
					if out, handled := themeBrowserCustomDraw(lp, true); handled {
						return out
					}
				} else if nh.From == b.controls[browserFilesID] {
					if out, handled := themeBrowserCustomDraw(lp, false); handled {
						return out
					}
				}
			}
			switch nh.From {
			case b.controls[browserTreeID]:
				if code == tvnSelChangedW && !b.populating {
					selected := send(b.controls[browserTreeID], tvmGetNextItem, tvgnCaret, 0)
					if p, ok := b.treePaths[selected]; ok {
						b.folder = p
						b.refreshFiles()
					}
					return 0
				}
				if code == nmClick || code == tvnKeyDown {
					var selected uintptr
					if code == nmClick {
						var pt nativePoint
						procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
						procScreenToClient.Call(b.controls[browserTreeID], uintptr(unsafe.Pointer(&pt)))
						hit := tvHitTestInfo{Point: pt}
						send(b.controls[browserTreeID], tvmHitTest, 0, uintptr(unsafe.Pointer(&hit)))
						if hit.Flags&tvhtOnItemStateIcon == 0 {
							break
						}
						selected = hit.Handle
					} else {
						var k tvKeyDown
						procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&k)), lp, unsafe.Sizeof(k))
						if k.Key != 32 {
							break
						}
						selected = send(b.controls[browserTreeID], tvmGetNextItem, tvgnCaret, 0)
					}
					if p, ok := b.treePaths[selected]; ok && !b.syncing {
						// Sentinel distinguishes the root's empty path from
						// an absent pending event.
						if p == "" {
							p = "\x00"
						}
						b.pendingToggle = p
						procPostMsg.Call(hwnd, wmAppToggleTree, 0, 0)
					}
				}
			case b.controls[browserFilesID]:
				if code == lvnItemChanged && !b.syncing {
					var n nmListView
					procCopyNativeMemory.Call(uintptr(unsafe.Pointer(&n)), lp, unsafe.Sizeof(n))
					if n.Item >= 0 && int(n.Item) < len(b.shown) && n.Changed&lvifState != 0 && (n.NewState&lvisStateImageMask) != (n.OldState&lvisStateImageMask) {
						b.toggle(b.shown[n.Item].Path)
					}
				}
				if code == nmDblClick {
					b.navigateSelectedFile()
				}
			}
		}
	case wmCommand:
		id := int(wp & 0xffff)
		notify := int((wp >> 16) & 0xffff)
		switch id {
		case browserSearchID:
			if notify == enChange {
				b.refreshFiles()
			}
		case browserSearchClearID:
			if notify == 0 {
				setText(b.controls[browserSearchID], "")
			}
		case browserRemoveID:
			if notify == 0 {
				b.removeMarked()
			}
		case browserClearID:
			if notify == 0 {
				b.clearMarked()
			}
		case browserDoneID:
			if notify == 0 {
				procDestroy.Call(hwnd)
			}
		}
		return 0
	case wmClose:
		procDestroy.Call(hwnd)
		return 0
	case wmDestroy:
		b.hwnd = 0
		if activeBrowser == b {
			activeBrowser = nil
		}
		return 0
	}
	r, _, _ := procDefWin.Call(hwnd, uintptr(msg), wp, lp)
	return r
}
