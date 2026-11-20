//go:build windows

package main

// Native Win32 GUI written against the Windows API. There is deliberately no
// webview, network listener, daemon, embedded Python runtime, or restore code.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	csHredraw           = 0x0002
	csVredraw           = 0x0001
	wsOverlappedWindow  = 0x00CF0000
	wsClipChildren      = 0x02000000
	wsVisible           = 0x10000000
	wsChild             = 0x40000000
	wsTabstop           = 0x00010000
	wsBorder            = 0x00800000
	wsVscroll           = 0x00200000
	esAutoHscroll       = 0x0080
	esMultiline         = 0x0004
	esAutoVscroll       = 0x0040
	esReadOnly          = 0x0800
	lbsNotify           = 0x0001
	lbsMultiple         = 0x0008
	lbsExtended         = 0x0800
	lbsNoIntegral       = 0x0100
	lbsOwnerDrawFixed   = 0x0010
	lbsHasStrings       = 0x0040
	cbsDropdownList     = 0x0003
	swShow              = 5
	wmCreate            = 0x0001
	wmDestroy           = 0x0002
	wmSize              = 0x0005
	wmClose             = 0x0010
	wmCommand           = 0x0111
	wmSetFont           = 0x0030
	wmAppEvent          = 0x8002
	wmSetText           = 0x000C
	enChange            = 0x0300
	lbnSelChange        = 1
	cbnSelChange        = 1
	lbAddString         = 0x0180
	lbResetContent      = 0x0184
	lbGetCurSel         = 0x0188
	lbGetSelCount       = 0x0190
	lbGetSelItems       = 0x0191
	lbGetSel            = 0x0187
	cbAddString         = 0x0143
	cbGetCurSel         = 0x0147
	cbSetCurSel         = 0x014E
	emSetSel            = 0x00B1
	emReplaceSel        = 0x00C2
	emLimitText         = 0x00C5
	mbOK                = 0
	mbIconError         = 0x10
	mbIconInfo          = 0x40
	mbYesNo             = 0x04
	mbIconWarning       = 0x30
	mbDefButton2        = 0x100
	idYes               = 6
	wmEnable            = 0x000A
	ofnExplorer         = 0x00080000
	ofnFileMustExist    = 0x00001000
	ofnPathMustExist    = 0x00000800
	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
	defaultGUIFont      = 17
	idExecutable        = 101
	idBackup            = 102
	idConfig            = 103
	idKey               = 104
	idPass              = 105
	idDestination       = 106
	idConfirm           = 107
	idStorage           = 120
	idPoints            = 121
	idItems             = 122
	idEntries           = 123
	idCheck             = 201
	idScan              = 202
	idInspect           = 203
	idRestore           = 204
	idStop              = 205
	idOpenBrowser       = 206
	idRemember          = 207
	idBrowseExe         = 301
	idBrowseBackup      = 302
	idBrowseConfig      = 303
	idBrowseKey         = 304
	idBrowsePass        = 305
	idBrowseDestination = 306
)

var (
	dllUser32        = syscall.NewLazyDLL("user32.dll")
	dllKernel32      = syscall.NewLazyDLL("kernel32.dll")
	dllGdi32         = syscall.NewLazyDLL("gdi32.dll")
	dllComdlg32      = syscall.NewLazyDLL("comdlg32.dll")
	dllShell32       = syscall.NewLazyDLL("shell32.dll")
	dllOle32         = syscall.NewLazyDLL("ole32.dll")
	procRegClass     = dllUser32.NewProc("RegisterClassExW")
	procCreate       = dllUser32.NewProc("CreateWindowExW")
	procDefWin       = dllUser32.NewProc("DefWindowProcW")
	procDestroy      = dllUser32.NewProc("DestroyWindow")
	procGetMsg       = dllUser32.NewProc("GetMessageW")
	procTranslate    = dllUser32.NewProc("TranslateMessage")
	procDispatch     = dllUser32.NewProc("DispatchMessageW")
	procPostQuit     = dllUser32.NewProc("PostQuitMessage")
	procPostMsg      = dllUser32.NewProc("PostMessageW")
	procSend         = dllUser32.NewProc("SendMessageW")
	procSetText      = dllUser32.NewProc("SetWindowTextW")
	procGetText      = dllUser32.NewProc("GetWindowTextW")
	procGetLen       = dllUser32.NewProc("GetWindowTextLengthW")
	procMove         = dllUser32.NewProc("MoveWindow")
	procEnable       = dllUser32.NewProc("EnableWindow")
	procMessageBox   = dllUser32.NewProc("MessageBoxW")
	procShow         = dllUser32.NewProc("ShowWindow")
	procUpdate       = dllUser32.NewProc("UpdateWindow")
	procLoadCursor   = dllUser32.NewProc("LoadCursorW")
	procLoadImage    = dllUser32.NewProc("LoadImageW")
	procCreateFont   = dllGdi32.NewProc("CreateFontW")
	procDeleteObject = dllGdi32.NewProc("DeleteObject")
	procGetOpenFile  = dllComdlg32.NewProc("GetOpenFileNameW")
	procBrowseFolder = dllShell32.NewProc("SHBrowseForFolderW")
	procFolderPath   = dllShell32.NewProc("SHGetPathFromIDListW")
	procCoTaskFree   = dllOle32.NewProc("CoTaskMemFree")
	procGetModule    = dllKernel32.NewProc("GetModuleHandleW")
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
	Private uint32
}
type openFileName struct {
	Size          uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustom     uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxTitle      uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustomData    uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	ReservedNum   uint32
	FlagsEx       uint32
}
type browseInfo struct {
	Owner       uintptr
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	Param       uintptr
	Image       int32
}

// These icons are embedded as native RT_GROUP_ICON / RT_ICON PE resources.
// Windows Explorer reads the same resources for the EXE's file icon.
// Using LR_SHARED avoids owning or manually destroying the class icons.
func appWindowIcons(instance uintptr) (large uintptr, small uintptr) {
	const imageIcon = 1
	const lrShared = 0x8000
	large, _, _ = procLoadImage.Call(instance, 1, imageIcon, 32, 32, lrShared)
	small, _, _ = procLoadImage.Call(instance, 1, imageIcon, 16, 16, lrShared)
	return large, small
}

func wide(s string) *uint16 { return syscall.StringToUTF16Ptr(s) }
func send(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	v, _, _ := procSend.Call(hwnd, uintptr(msg), wp, lp)
	return v
}
func setText(hwnd uintptr, s string) { procSetText.Call(hwnd, uintptr(unsafe.Pointer(wide(s)))) }
func getText(hwnd uintptr) string {
	n, _, _ := procGetLen.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, int(n)+1)
	procGetText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func msgBox(parent uintptr, text, title string, flags uintptr) uintptr {
	ret, _, _ := procMessageBox.Call(parent, uintptr(unsafe.Pointer(wide(text))), uintptr(unsafe.Pointer(wide(title))), flags)
	return ret
}
func move(hwnd uintptr, x, y, w, h int) {
	if hwnd != 0 {
		procMove.Call(hwnd, uintptr(x), uintptr(y), uintptr(max(w, 1)), uintptr(max(h, 1)), 1)
	}
}
func enabled(hwnd uintptr, on bool) {
	if hwnd != 0 {
		var v uintptr
		if on {
			v = 1
		}
		procEnable.Call(hwnd, v)
	}
}
func displayError(hwnd uintptr, msg string) {
	msgBox(hwnd, msg, "Vault recovery error", mbOK|mbIconError)
}

type event struct {
	mode      string
	cancelled bool // the GUI cancelled the command context; not an engine failure
	signature string
	points    []Point
	entries   []Entry
	report    Report
	details   string
	elapsed   time.Duration
	hash      string
	err       error
}

type appState struct {
	hwnd            uintptr
	fields          map[int]uintptr
	labels          map[string]uintptr
	font            uintptr
	titleFont       uintptr
	points          []Point
	entries         []Entry
	browserModel    *browserModel
	inspectedSig    string
	scanSig         string
	events          chan event
	busy            bool
	restoring       bool
	cancel          context.CancelFunc
	closing         bool
	suppress        bool
	width           int
	height          int
	mascot          mascotTracker // display-only workflow indicator
	rememberEnabled bool
	settingsPath    string
	summaryHwnd     uintptr
}

var app = &appState{fields: map[int]uintptr{}, labels: map[string]uintptr{}, events: make(chan event, 500)}

func classCreate(class, title string, style uint32, x, y, w, h int, parent uintptr, id int) uintptr {
	cls := wide(class)
	txt := wide(title)
	v, _, _ := procCreate.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(txt)), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, uintptr(id), 0, 0)
	return v
}
func (a *appState) control(id int, class, title string, style uint32) uintptr {
	v := classCreate(class, title, style|wsChild|wsVisible, 0, 0, 30, 26, a.hwnd, id)
	a.fields[id] = v
	if a.font != 0 {
		send(v, wmSetFont, a.font, 1)
	}
	if class == "BUTTON" {
		installThemedButton(v)
	} else if class == "LISTBOX" {
		installThemedListbox(v)
	}
	return v
}
func (a *appState) label(name, title string) uintptr {
	v := classCreate("STATIC", title, wsChild|wsVisible|ssOwnerDraw, 0, 0, 10, 20, a.hwnd, 0)
	a.labels[name] = v
	if a.font != 0 {
		send(v, wmSetFont, a.font, 1)
	}
	return v
}
func (a *appState) field(id int) string  { return strings.TrimSpace(getText(a.fields[id])) }
func (a *appState) setStatus(msg string) { setText(a.labels["status"], msg) }

// Opt-in path history; never write a key, a passphrase, or remote config.
func (a *appState) persistPaths() {
	if !a.rememberEnabled || a.settingsPath == "" || a.suppress {
		return
	}
	_ = saveRememberedPaths(a.settingsPath, rememberedPaths{
		Executable: a.field(idExecutable), Backup: a.field(idBackup), Destination: a.field(idDestination),
	})
}
func (a *appState) toggleRememberPaths() {
	a.rememberEnabled = !a.rememberEnabled
	if a.rememberEnabled {
		a.persistPaths()
	} else if a.settingsPath != "" {
		if err := deleteRememberedPaths(a.settingsPath); err != nil {
			a.showInputError("Could not remove remembered paths: " + err.Error())
		}
	}
	a.refreshRememberButton()
}
func (a *appState) refreshRememberButton() {
	label := "[ ] Remember paths on this PC"
	if a.rememberEnabled {
		label = "[x] Remember paths on this PC"
	}
	setText(a.fields[idRemember], label)
}

// Update only the decorative image. No backup, key, or restore state changes.
func (a *appState) setMascot(phase mascotPhase) {
	if a.mascot.phase == phase {
		return
	}
	a.mascot.phase = phase
	if a.hwnd != 0 {
		requestThemeRedraw(a.hwnd)
	}
}

// Local validation errors use the same one-line status area as CLI failures.
// A dialog always shows the complete error, even when the status line clips.
func (a *appState) showInputError(message string) {
	a.setStatus("ERROR — " + statusErrorSummary(fmt.Errorf("%s", message)))
	displayError(a.hwnd, message)
}

func (a *appState) storageType() string {
	i := int(send(a.fields[idStorage], cbGetCurSel, 0, 0))
	choices := []string{"local", "s3", "sftp", "smb", "webdav"}
	if i < 0 || i >= len(choices) {
		return "local"
	}
	return choices[i]
}
func (a *appState) connection() Connection {
	return Connection{Type: a.storageType(), BackupFolder: a.field(idBackup), ConfigFile: a.field(idConfig), KeyFile: a.field(idKey), PassphraseFile: a.field(idPass)}
}
func (a *appState) signature() string {
	c := a.connection()
	return strings.Join([]string{a.field(idExecutable), c.Type, c.BackupFolder, c.ConfigFile, c.KeyFile, c.PassphraseFile}, "\x00")
}
func (a *appState) selectedPoint() *Point {
	index := int(send(a.fields[idPoints], lbGetCurSel, 0, 0))
	if index < 0 || index >= len(a.points) {
		return nil
	}
	return &a.points[index]
}
func (a *appState) selectedItem() *Item {
	p := a.selectedPoint()
	if p == nil {
		return nil
	}
	index := int(send(a.fields[idItems], lbGetCurSel, 0, 0))
	if index < 0 || index >= len(p.Items) {
		return nil
	}
	return &p.Items[index]
}
func (a *appState) inspectionSignature() string {
	p := a.selectedPoint()
	item := a.selectedItem()
	if p == nil || item == nil {
		return ""
	}
	return a.signature() + "\x00" + p.StoragePath + "\x00" + item.Name
}
func (a *appState) resetContents() {
	a.closeBrowser()
	a.entries = nil
	a.browserModel = nil
	a.inspectedSig = ""
	send(a.fields[idEntries], lbResetContent, 0, 0)
	send(a.fields[idEntries], lbAddString, 0, uintptr(unsafe.Pointer(wide("Inspect an item to open the separate file browser."))))
	setText(a.fields[idConfirm], "")
}

// Browsing never sets a restore selection by itself. A checked root
// explicitly opts into whole-item recovery; no checked paths disables restore.
func (a *appState) refreshMainSelection() {
	send(a.fields[idEntries], lbResetContent, 0, 0)
	if a.browserModel == nil {
		send(a.fields[idEntries], lbAddString, 0, uintptr(unsafe.Pointer(wide("Inspect an item to see its files in the separate browser."))))
		return
	}
	paths := a.browserModel.includes()
	if a.browserModel.wholeItem {
		send(a.fields[idEntries], lbAddString, 0, uintptr(unsafe.Pointer(wide("ENTIRE ITEM explicitly checked for recovery."))))
	} else if len(paths) == 0 {
		send(a.fields[idEntries], lbAddString, 0, uintptr(unsafe.Pointer(wide("No paths checked. Restore is disabled until you check something in the file browser."))))
	} else {
		for _, p := range paths {
			send(a.fields[idEntries], lbAddString, 0, uintptr(unsafe.Pointer(wide("Selected for recovery: "+p))))
		}
	}
	// Any change in scope demands the explicit RESTORE acknowledgement again.
	setText(a.fields[idConfirm], "")
	a.syncButtons()
}
func (a *appState) invalidate() {
	if a.suppress {
		return
	}
	a.points = nil
	a.scanSig = ""
	send(a.fields[idPoints], lbResetContent, 0, 0)
	send(a.fields[idItems], lbResetContent, 0, 0)
	a.resetContents()
	a.setMascot(mascotIdle)
	a.setStatus("Connection changed. Scan again; no files have been restored.")
	a.syncButtons()
}
func (a *appState) syncButtons() {
	available := !a.busy
	p := a.selectedPoint()
	it := a.selectedItem()
	canInspect := !a.busy && a.signature() == a.scanSig && p != nil && it != nil && !p.Locked
	canRestore := canInspect && a.inspectedSig == a.inspectionSignature() && a.browserModel != nil && a.browserModel.ready() && a.field(idDestination) != "" && a.field(idConfirm) == "RESTORE"
	enabled(a.fields[idScan], available)
	enabled(a.fields[idCheck], available)
	enabled(a.fields[idInspect], canInspect)
	enabled(a.fields[idOpenBrowser], canInspect && a.inspectedSig == a.inspectionSignature())
	enabled(a.fields[idRestore], canRestore)
	enabled(a.fields[idStop], a.busy)
	for _, id := range []int{idExecutable, idBackup, idConfig, idKey, idPass, idDestination, idConfirm, idStorage, idPoints, idItems, idEntries} {
		enabled(a.fields[id], available)
	}
	for _, id := range []int{idBrowseExe, idBrowseBackup, idBrowseConfig, idBrowseKey, idBrowsePass, idBrowseDestination} {
		enabled(a.fields[id], available)
	}
}
func (a *appState) sendEvent(ev event) {
	select {
	case a.events <- ev:
	default:
		if ev.mode != "progress" {
			a.events <- ev
		} else {
			return
		}
	}
	procPostMsg.Call(a.hwnd, wmAppEvent, 0, 0)
}
func (a *appState) launch(mode string, fn func(context.Context) event) {
	if a.busy {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.busy = true
	a.restoring = mode == "restore"
	a.mascot.begin(mode)
	requestThemeRedraw(a.hwnd)
	a.cancel = cancel
	a.syncButtons()
	intro := "Running original Vault executable — " + mode + " ..."
	if mode == "scan" {
		intro = "Read-only: scanning backup restore points. No restore is being performed."
	}
	if mode == "inspect" {
		intro = "Read-only: Vault is indexing the selected item's file listing. No restore is being performed."
	}
	if mode == "restore" {
		intro = "Vault restore started — preparing repository data. Waiting for actual extraction output; no percentage is available."
	}
	a.setStatus(intro)
	sig := a.signature()
	started := time.Now()
	finished := make(chan struct{})
	// `recover contents --json` returns a completed JSON listing, not live file
	// progress. Show elapsed time only (not a fabricated percent complete).
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-finished:
				return
			case <-ticker.C:
				a.sendEvent(event{mode: "heartbeat", elapsed: time.Since(started)})
			}
		}
	}()
	go func() {
		ev := fn(ctx)
		// Only a cancelled GUI command context produces the cancelled mascot.
		// Vault CLI failures without cancellation use the failed mascot.
		ev.cancelled = ctx.Err() != nil
		close(finished)
		ev.elapsed = time.Since(started)
		ev.mode = mode
		ev.signature = sig
		a.sendEvent(ev)
	}()
}
func (a *appState) checkExecutable() {
	exe := a.field(idExecutable)
	a.launch("check", func(ctx context.Context) event {
		hash, err := checkExecutable(ctx, exe)
		return event{hash: hash, err: err}
	})
}
func (a *appState) scan() {
	conn := a.connection()
	exe := a.field(idExecutable)
	if _, err := conn.Flags(); err != nil {
		a.showInputError(err.Error())
		return
	}
	a.points = nil
	a.scanSig = ""
	send(a.fields[idPoints], lbResetContent, 0, 0)
	send(a.fields[idItems], lbResetContent, 0, 0)
	a.resetContents()
	a.launch("scan", func(ctx context.Context) event {
		points, _, err := scanPoints(ctx, exe, conn)
		return event{points: points, err: err}
	})
}
func (a *appState) selectPoint() {
	a.resetContents()
	send(a.fields[idItems], lbResetContent, 0, 0)
	if p := a.selectedPoint(); p != nil {
		for _, it := range p.Items {
			send(a.fields[idItems], lbAddString, 0, uintptr(unsafe.Pointer(wide(it.Name+"   ["+it.Type+"]"))))
		}
		// Locked manifests still count as a successful read-only scan, but
		// they cannot be inspected or restored without valid credentials.
		a.mascot.selectedLockedPoint(p.Locked)
		requestThemeRedraw(a.hwnd)
		if p.Locked {
			a.setStatus("LOCKED / KEY NEEDED — backup found, but cannot be opened. Check original vault.key or backup passphrase, then scan again.")
		} else {
			a.setStatus("READ-ONLY: selected restore point is readable. Select an item to inspect.")
		}
	} else {
		// Preserve the post-scan expression if selection is cleared.
		// An entirely locked repository stays amber; readable points keep
		// the original thinking/ready-to-browse image.
		a.mascot.scanned(a.points)
		requestThemeRedraw(a.hwnd)
	}
	a.syncButtons()
}
func (a *appState) inspect() {
	if a.inspectionSignature() == "" || a.scanSig != a.signature() {
		return
	}
	conn := a.connection()
	exe := a.field(idExecutable)
	p := *a.selectedPoint()
	it := *a.selectedItem()
	target := a.inspectionSignature()
	a.resetContents()
	a.launch("inspect", func(ctx context.Context) event {
		entries, _, err := inspectContents(ctx, exe, conn, p.StoragePath, it.Name)
		return event{entries: entries, hash: target, err: err}
	})
}
func (a *appState) selectedIncludes() []string {
	if a.browserModel == nil {
		return nil
	}
	return a.browserModel.includes()
}

func (a *appState) restore() {
	if a.field(idConfirm) != "RESTORE" || a.inspectedSig == "" || a.inspectedSig != a.inspectionSignature() || a.scanSig != a.signature() || a.browserModel == nil || !a.browserModel.ready() {
		a.showInputError("Inspect the item, check the files or folders to recover (or explicitly check the entire item), then type RESTORE to authorize this operation.")
		return
	}
	p := a.selectedPoint()
	it := a.selectedItem()
	if p == nil || it == nil || p.Locked {
		return
	}
	req := RestoreRequest{Connection: a.connection(), Point: p.StoragePath, Item: it.Name, Destination: a.field(idDestination), Includes: a.selectedIncludes()}
	if _, err := req.Args(); err != nil {
		a.showInputError(err.Error())
		return
	}
	what := "Entire item"
	if len(req.Includes) > 0 {
		what = fmt.Sprintf("%d selected file/folder path(s)", len(req.Includes))
	}
	estimate := estimateSelection(a.entries, a.browserModel.wholeItem, req.Includes)
	free, freeErr := availableSpace(req.Destination)
	spaceText := preflightSpaceMessage(estimate, free, freeErr)
	conflict := destinationItemConflict(req.Destination, req.Item)
	scopeDetails := ""
	if len(req.Includes) > 0 {
		shown := req.Includes
		if len(shown) > 8 {
			shown = shown[:8]
		}
		scopeDetails = "\n" + strings.Join(shown, "\n")
		if len(req.Includes) > len(shown) {
			scopeDetails += fmt.Sprintf("\n... and %d more", len(req.Includes)-len(shown))
		}
	}
	backupDate := p.CreatedAt
	if backupDate == "" {
		backupDate = "Not supplied by Vault"
	}
	if len(scopeDetails) > 1600 {
		scopeDetails = scopeDetails[:1600] + "..."
	}
	summary := fmt.Sprintf("PRE-RESTORE REVIEW — files WILL be written.\n\nRestore point: %s\nBackup date (from Vault): %s\nItem: %s\nSelection: %s%s\n\nSelected file estimate: %d file(s), %s\nFree space check: %s\n\nDestination: %s\n\nOverwrite is DISABLED. The original Vault CLI performs the restore.\n\nProceed?", req.Point, backupDate, req.Item, what, scopeDetails, estimate.Files, readableBytes(estimate.Bytes), spaceText, req.Destination)
	if conflict != "" {
		summary = strings.Replace(summary, "\n\nProceed?", "\n\nDESTINATION WARNING: "+conflict+"\n\nProceed?", 1)
	}
	if !showPreflightReview(a.hwnd, summary) {
		return
	}
	setText(a.fields[idConfirm], "")
	exe := a.field(idExecutable)
	// Keep the inspected file index so another restore can be selected after
	// this command finishes. A fresh CLI process still performs every restore.
	// Do not let the browser change scope while a restore is in flight.
	a.closeBrowser()
	a.launch("restore", func(ctx context.Context) event {
		// Observe the existing CLI progress milestone on stderr; do not
		// implement extraction, infer file counts, or change the engine.
		var detector extractorMilestone
		rep, _, err := restore(ctx, exe, req, true, func(stream, data string) {
			if stream == "stderr" && detector.feed(data) {
				a.sendEvent(event{mode: "extracting"})
			}
		})
		return event{report: rep, err: err}
	})
}
func (a *appState) handleEvent(ev event) {
	if ev.mode == "heartbeat" {
		if a.busy {
			before := a.mascot.phase
			if a.restoring {
				a.mascot.elapsed(ev.elapsed)
			}
			if before != a.mascot.phase {
				requestThemeRedraw(a.hwnd)
			}
			switch a.mascot.phase {
			case mascotPreparing:
				a.setStatus(fmt.Sprintf("Vault restore: preparing repository data — %s elapsed. No engine percentage available.", elapsedLabel(ev.elapsed)))
			case mascotWaiting:
				a.setStatus(fmt.Sprintf("Vault restore: still preparing — %s elapsed. Vault hasn't reported extraction starting yet.", elapsedLabel(ev.elapsed)))
			case mascotExtracting:
				a.setStatus(fmt.Sprintf("Vault reports extraction started — %s elapsed. Recovering selected paths; no per-file progress emitted.", elapsedLabel(ev.elapsed)))
			case mascotScanning:
				a.setStatus(fmt.Sprintf("Read-only Vault scan/inspection — %s elapsed. Waiting for Vault's file listing.", elapsedLabel(ev.elapsed)))
			default:
				a.setStatus(fmt.Sprintf("Vault command running — %s elapsed.", elapsedLabel(ev.elapsed)))
			}
		}
		return
	}
	if ev.mode == "extracting" {
		if a.busy && a.restoring {
			before := a.mascot.phase
			a.mascot.extractionStarted()
			if before != a.mascot.phase {
				requestThemeRedraw(a.hwnd)
			}
			if a.mascot.phase == mascotExtracting {
				a.setStatus("Vault reports extraction started — recovering selected files. Final counts will come from the verified report.")
			}
		}
		return
	}
	a.busy = false
	a.restoring = false
	a.cancel = nil
	if ev.cancelled {
		// Cancellation is a user action, not a recovery success or an
		// engine failure. An interrupted restore may leave partial files.
		a.mascot.cancelled()
		requestThemeRedraw(a.hwnd)
		if ev.mode == "restore" {
			a.setStatus("CANCELLED / NOT CONFIRMED — Vault restore stopped. Partially extracted files may remain.")
		} else {
			a.setStatus("CANCELLED — Vault command stopped. No restore was confirmed.")
		}
	} else if ev.err != nil {
		// Includes failed scans/inspection, nonzero Vault exit, or a restore
		// that exited zero without a trustworthy successful recovery report.
		a.mascot.fail()
		requestThemeRedraw(a.hwnd)
		a.setStatus("FAILED / NOT CONFIRMED — " + statusErrorSummary(ev.err))
		displayError(a.hwnd, ev.err.Error())
	} else if ev.signature != a.signature() {
		a.setMascot(mascotIdle)
		a.setStatus("Settings changed during execution. Results discarded; scan again.")
	} else {
		switch ev.mode {
		case "check":
			a.setStatus("CLI commands verified; compare this SHA-256 with a trusted official release checksum: " + ev.hash)
		case "scan":
			a.points = ev.points
			a.scanSig = ev.signature
			send(a.fields[idPoints], lbResetContent, 0, 0)
			for _, p := range a.points {
				label := p.CreatedAt
				if len(label) > 19 {
					label = label[:19]
				}
				status := "readable"
				if p.Locked {
					status = "LOCKED " + p.LockedBy
				}
				row := fmt.Sprintf("%s   %s   [%s; %s]", label, p.StoragePath, p.BackupType, status)
				send(a.fields[idPoints], lbAddString, 0, uintptr(unsafe.Pointer(wide(row))))
			}
			locked, readable := a.mascot.scanned(a.points)
			requestThemeRedraw(a.hwnd)
			switch {
			case locked > 0 && readable == 0:
				a.setStatus(fmt.Sprintf("LOCKED / KEY NEEDED — %d restore point(s) found, all locked. Verify original vault.key or backup passphrase and scan again.", locked))
			case locked > 0:
				a.setStatus(fmt.Sprintf("READ-ONLY: %d restore point(s) found (%d readable, %d locked). Select a point; locked backups need the correct key or passphrase.", len(a.points), readable, locked))
			default:
				a.setStatus(fmt.Sprintf("READ-ONLY inspection: %d restore point(s) found. Select a point and an item.", len(a.points)))
			}
		case "inspect":
			if ev.hash != a.inspectionSignature() {
				a.setStatus("Selection changed; inspect again.")
				break
			}
			a.entries = ev.entries
			a.browserModel = newBrowserModel(ev.entries)
			a.inspectedSig = ev.hash
			a.refreshMainSelection()
			a.setStatus(fmt.Sprintf("READ-ONLY: %d paths indexed. Browse folders or search in the separate file browser; no extra repository scan needed.", len(ev.entries)))
			a.showBrowser()
		case "restore":
			// restore() validates both original CLI exit status and report before
			// returning nil; only this branch can display success artwork.
			a.mascot.complete("restore", true)
			requestThemeRedraw(a.hwnd)
			r := ev.report.Items[0]
			// The underlying restore was confirmed by Vault. Clear the previous
			// selection so a second restore is always a NEW, explicit choice.
			// Preserve the inspected paths and their signature: browsing again
			// is instant, with no redundant dedup index rebuild.
			if a.browserModel != nil {
				a.browserModel.afterConfirmedRestore()
				a.refreshMainSelection()
			}
			a.setStatus(fmt.Sprintf("CONFIRMED by original Vault report: %s — %d files, %s. Open file browser to choose more files.", r.Item, r.Files, readableBytes(r.Bytes)))
			a.showRecoverySummary(ev.report)
		}
	}
	a.syncButtons()
	if a.closing {
		procDestroy.Call(a.hwnd)
	}
}
func (a *appState) browseFile(id int) {
	buf := make([]uint16, 32768)
	existing := a.field(id)
	if existing != "" {
		copy(buf, syscall.StringToUTF16(existing))
	}
	filter := []uint16{'A', 'l', 'l', ' ', 'f', 'i', 'l', 'e', 's', 0, '*', '.', '*', 0, 0}
	title := wide("Select a file (the GUI never changes this file)")
	ofn := openFileName{Owner: a.hwnd, Filter: &filter[0], File: &buf[0], MaxFile: uint32(len(buf)), Title: title, Flags: ofnExplorer | ofnFileMustExist | ofnPathMustExist}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	ok, _, _ := procGetOpenFile.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok != 0 {
		setText(a.fields[id], syscall.UTF16ToString(buf))
		if id == idExecutable {
			a.persistPaths()
		}
	}
}
func (a *appState) browseFolder(id int) {
	var display [260]uint16
	title := wide("Choose an existing folder (the GUI only writes during confirmed recovery)")
	info := browseInfo{Owner: a.hwnd, DisplayName: &display[0], Title: title, Flags: bifReturnOnlyFSDirs | bifNewDialogStyle}
	pidl, _, _ := procBrowseFolder.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return
	}
	defer procCoTaskFree.Call(pidl)
	var buf [32768]uint16
	ok, _, _ := procFolderPath.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if ok != 0 {
		setText(a.fields[id], syscall.UTF16ToString(buf[:]))
		if id == idBackup || id == idDestination {
			a.persistPaths()
		}
	}
}

func (a *appState) createContents() {
	a.suppress = true
	defer func() { a.suppress = false }()
	// Segoe UI and system widgets; all strings are local. Nothing is served.
	f, _, _ := procCreateFont.Call(uintptr(^uintptr(15)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
	// A negative LF height requests a normal 16px text font.
	a.font = f
	ft, _, _ := procCreateFont.Call(uintptr(^uintptr(23)), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
	a.titleFont = ft
	a.label("title", "Vault Offline Recovery")
	send(a.labels["title"], wmSetFont, a.titleFont, 1)
	a.label("intro", "READ-ONLY BY DEFAULT   |   All operations use the original, independent vault recover executable")
	for _, label := range []struct{ key, text string }{
		{"exe", "Vault executable"}, {"type", "Storage type"}, {"backup", "Local backup folder"},
		{"config", "Remote settings JSON"}, {"key", "Original vault.key (optional)"}, {"pass", "Passphrase file (optional)"},
		{"hash", "To confirm authenticity, compare the SHA-256 in the status to an official release checksum."},
		{"points", "2. Restore points (choose one)"}, {"items", "3. Items (choose one)"},
		{"entries", "4. Checked recovery paths (nothing checked = no restore)"},
		{"dest", "Restore destination"}, {"confirm", "Type RESTORE"},
		{"status", "Read-only inspection mode. No files have been restored."},
	} {
		a.label(label.key, label.text)
	}
	for _, id := range []int{idExecutable, idBackup, idConfig, idKey, idPass, idDestination, idConfirm} {
		a.control(id, "EDIT", "", wsBorder|wsTabstop|esAutoHscroll)
	}
	a.control(idStorage, "COMBOBOX", "", wsTabstop|cbsDropdownList)
	for _, typ := range []string{"local", "s3", "sftp", "smb", "webdav"} {
		send(a.fields[idStorage], cbAddString, 0, uintptr(unsafe.Pointer(wide(typ))))
	}
	send(a.fields[idStorage], cbSetCurSel, 0, 0)
	for _, b := range []struct {
		id   int
		text string
	}{
		{idBrowseExe, "Browse..."}, {idBrowseBackup, "Browse..."}, {idBrowseConfig, "Browse..."}, {idBrowseKey, "Browse..."}, {idBrowsePass, "Browse..."}, {idBrowseDestination, "Browse..."},
		{idCheck, "Check executable + SHA-256"}, {idScan, "Scan backups (read only)"}, {idInspect, "Inspect item files (read only)"},
		{idRestore, "Restore files (confirmed)"}, {idStop, "Stop command"}, {idOpenBrowser, "Open file browser..."},
		{idRemember, "[ ] Remember paths on this PC"},
	} {
		a.control(b.id, "BUTTON", b.text, wsTabstop|bsOwnerDraw)
	}
	a.control(idPoints, "LISTBOX", "", wsBorder|wsVscroll|wsTabstop|lbsNotify|lbsNoIntegral|lbsOwnerDrawFixed|lbsHasStrings)
	a.control(idItems, "LISTBOX", "", wsBorder|wsVscroll|wsTabstop|lbsNotify|lbsNoIntegral|lbsOwnerDrawFixed|lbsHasStrings)
	a.control(idEntries, "LISTBOX", "", wsBorder|wsVscroll|wsTabstop|lbsNoIntegral|lbsOwnerDrawFixed|lbsHasStrings)
	exe, err := os.Executable()
	if err == nil {
		for _, name := range []string{"vault-windows-amd64.exe", "vault.exe"} {
			candidate := filepath.Join(filepath.Dir(exe), name)
			if _, e := os.Stat(candidate); e == nil {
				setText(a.fields[idExecutable], candidate)
				break
			}
		}
	}
	a.settingsPath, _ = settingsFilePath()
	if a.settingsPath != "" {
		if v, e := loadRememberedPaths(a.settingsPath); e == nil {
			a.rememberEnabled = true
			if v.Executable != "" {
				setText(a.fields[idExecutable], v.Executable)
			}
			if v.Backup != "" {
				setText(a.fields[idBackup], v.Backup)
			}
			if v.Destination != "" {
				setText(a.fields[idDestination], v.Destination)
			}
		}
	}
	a.refreshRememberButton()
	a.width = 1100
	a.height = 880
	a.layout()
	a.syncButtons()
}
func (a *appState) layout() {
	w := a.width
	h := a.height
	if w < 780 {
		w = 780
	}
	left := 18
	labelW := 256
	fieldX := left + labelW
	// The portrait only shortens the upper connection rows. The restore
	// destination remains full-width, but starts nearer its shorter label.
	destination := calculateDestinationRowGeometry(w)
	settings := calculateTopFormGeometry(w)
	move(a.labels["title"], left, 10, w-36, 33)
	move(a.labels["intro"], left, 43, w-36, 20)
	rows := []struct {
		key    string
		id     int
		browse int
		y      int
	}{
		{"exe", idExecutable, idBrowseExe, 94},
		{"backup", idBackup, idBrowseBackup, 160}, {"config", idConfig, idBrowseConfig, 193},
		{"key", idKey, idBrowseKey, 226}, {"pass", idPass, idBrowsePass, 259},
	}
	for _, r := range rows {
		move(a.labels[r.key], left, r.y+5, labelW-5, 20)
		move(a.fields[r.id], settings.FieldX, r.y, settings.FieldWidth, 26)
		move(a.fields[r.browse], settings.BrowseX, r.y, settings.BrowseWidth, 26)
	}
	move(a.labels["type"], left, 132, labelW-5, 20)
	move(a.fields[idStorage], fieldX, 128, 145, 220)
	move(a.fields[idCheck], left, 301, 225, 30)
	move(a.fields[idScan], left+235, 301, 210, 30)
	move(a.fields[idRemember], left+465, 301, 270, 30)
	move(a.labels["hash"], left, 337, w-36, 20)
	const listY = 383
	move(a.labels["points"], left, 362, (w-46)*2/3, 20)
	itemX := left + (w-40)*2/3 + 8
	itemW := w - itemX - 18
	move(a.labels["items"], itemX, 362, itemW, 20)
	geo := calculateMainLayout(w, h, left, itemX, itemW)
	move(a.fields[idPoints], left, listY, itemX-left-10, geo.ListHeight)
	move(a.fields[idItems], itemX, listY, itemW, geo.ListHeight)
	// Same action row, like Check executable / Scan backups.
	move(a.fields[idInspect], geo.InspectX, geo.ActionsY, geo.InspectWidth, 29)
	move(a.fields[idOpenBrowser], geo.BrowserX, geo.ActionsY, geo.BrowserWidth, 29)
	// Both actions share one row, giving more space to file selection.
	move(a.labels["entries"], left, geo.CheckedLabelY, geo.CheckedLabelW, 20)
	move(a.fields[idEntries], left, geo.EntriesY, w-36, geo.EntriesHeight)
	move(a.labels["dest"], left, geo.DestY+4, destination.LabelWidth, 20)
	move(a.fields[idDestination], destination.FieldX, geo.DestY, destination.FieldWidth, 26)
	move(a.fields[idBrowseDestination], destination.BrowseX, geo.DestY, destination.BrowseWidth, 26)
	move(a.labels["confirm"], left, geo.ConfirmY+5, 102, 20)
	move(a.fields[idConfirm], left+108, geo.ConfirmY, 143, 26)
	move(a.fields[idRestore], left+263, geo.ConfirmY, 250, 29)
	move(a.fields[idStop], left+523, geo.ConfirmY, 140, 29)
	move(a.labels["status"], left, geo.StatusY, w-36, 29)
}

func windowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	a := app
	switch msg {
	case wmCreate:
		a.hwnd = hwnd
		activateDarkTitle(hwnd)
		a.createContents()
		return 0
	case wmEraseBkgnd:
		themeBackdrop(hwnd, wp, false)
		return 1
	case wmCtlColorEdit, wmCtlColorListbox, wmCtlColorStatic, wmCtlColorBtn:
		return mainControlColor(a, msg, wp, lp)
	case wmMeasureItem:
		if measureThemedListbox(lp) {
			return 1
		}
	case wmDrawItem:
		if drawThemedLabel(lp, a.labels, hwnd, a.font, a.titleFont) ||
			drawThemedListbox(lp, func(h uintptr) bool {
				return h == a.fields[idPoints] || h == a.fields[idItems] || h == a.fields[idEntries]
			}) || themeDrawButton(lp) {
			return 1
		}
	case wmSize:
		a.width = int(uint16(lp & 0xffff))
		a.height = int(uint16((lp >> 16) & 0xffff))
		a.layout()
		requestThemeRedraw(hwnd)
		return 0
	case wmAppEvent:
		for {
			select {
			case ev := <-a.events:
				a.handleEvent(ev)
			default:
				return 0
			}
		}
	case wmCommand:
		id := int(wp & 0xffff)
		notify := int((wp >> 16) & 0xffff)
		switch id {
		case idCheck:
			if notify == 0 {
				a.checkExecutable()
			}
		case idScan:
			if notify == 0 {
				a.scan()
			}
		case idInspect:
			if notify == 0 {
				a.inspect()
			}
		case idRestore:
			if notify == 0 {
				a.restore()
			}
		case idOpenBrowser:
			if notify == 0 {
				a.showBrowser()
			}
		case idRemember:
			if notify == 0 {
				a.toggleRememberPaths()
			}
		case idStop:
			if notify == 0 && a.cancel != nil {
				warning := "Stop this read-only Vault command?\nNo files have been restored by inspection."
				status := "Stopping read-only Vault command; no files restored."
				if a.restoring {
					warning = "Stop the Vault restore?\nAny partially extracted files may remain."
					status = "Stopping Vault restore; partial recovered files may remain."
				}
				if msgBox(hwnd, warning, "Stop Vault command?", mbYesNo|mbIconWarning|mbDefButton2) == idYes {
					a.cancel()
					a.setStatus(status)
				}
			}
		case idBrowseExe:
			if notify == 0 {
				a.browseFile(idExecutable)
			}
		case idBrowseBackup:
			if notify == 0 {
				a.browseFolder(idBackup)
			}
		case idBrowseConfig:
			if notify == 0 {
				a.browseFile(idConfig)
			}
		case idBrowseKey:
			if notify == 0 {
				a.browseFile(idKey)
			}
		case idBrowsePass:
			if notify == 0 {
				a.browseFile(idPass)
			}
		case idBrowseDestination:
			if notify == 0 {
				a.browseFolder(idDestination)
			}
		case idPoints:
			if notify == lbnSelChange {
				a.selectPoint()
			}
		case idItems:
			if notify == lbnSelChange {
				a.resetContents()
				a.syncButtons()
			}
		case idStorage:
			if notify == cbnSelChange {
				a.invalidate()
			}
		case idExecutable, idBackup, idConfig, idKey, idPass:
			if notify == enChange {
				a.invalidate()
			}
		case idDestination, idConfirm:
			if notify == enChange {
				a.syncButtons()
			}
		}
		return 0
	case wmClose:
		if a.busy {
			if msgBox(hwnd, "A Vault command is still running.\n\nStop it and close? Any partial extracted files may remain.", "Close Vault Recovery?", mbYesNo|mbIconWarning|mbDefButton2) != idYes {
				return 0
			}
			a.closing = true
			if a.cancel != nil {
				a.cancel()
			}
			a.setStatus("Stopping Vault executable before closing.")
			return 0
		}
		procDestroy.Call(hwnd)
		return 0
	case wmDestroy:
		a.persistPaths()
		a.closeBrowser()
		if a.font != 0 {
			procDeleteObject.Call(a.font)
		}
		if a.titleFont != 0 {
			procDeleteObject.Call(a.titleFont)
		}
		procPostQuit.Call(0)
		return 0
	}
	ret, _, _ := procDefWin.Call(hwnd, uintptr(msg), wp, lp)
	return ret
}

func main() {
	runtime.LockOSThread()
	// The classic Windows folder dialog requires a COM apartment on the UI thread.
	initCOM := dllOle32.NewProc("CoInitializeEx")
	uninitCOM := dllOle32.NewProc("CoUninitialize")
	status, _, _ := initCOM.Call(0, 2) // COINIT_APARTMENTTHREADED
	if status == 0 || status == 1 {
		defer uninitCOM.Call()
	}
	setupTheme()
	defer releaseTheme()
	setupAvatar()
	defer releaseAvatar()
	instance, _, _ := procGetModule.Call(0)
	cls := wide("VaultOfflineRecoveryGuiNative")
	cursor, _, _ := procLoadCursor.Call(0, 32512) // IDC_ARROW
	largeIcon, smallIcon := appWindowIcons(instance)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: csHredraw | csVredraw, WndProc: syscall.NewCallback(windowProc), Instance: instance, Icon: largeIcon, IconSm: smallIcon, Cursor: cursor, Background: 0, ClassName: cls}
	atom, _, err := procRegClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		msgBox(0, "Could not register window class: "+err.Error(), "Vault GUI error", mbIconError)
		return
	}
	v, _, e := procCreate.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(wide("Vault Offline Recovery"))), wsOverlappedWindow|wsClipChildren, 80, 60, 1120, 900, 0, 0, instance, 0)
	if v == 0 {
		msgBox(0, "Could not create window: "+e.Error(), "Vault GUI error", mbIconError)
		return
	}
	procShow.Call(v, swShow)
	procUpdate.Call(v)
	var m winMsg
	for {
		r, _, _ := procGetMsg.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslate.Call(uintptr(unsafe.Pointer(&m)))
		procDispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
}
