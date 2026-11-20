//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	idSummaryOpenFolder = 1101
	idSummaryViewReport = 1102
	idSummaryClose      = 1103
	idSummaryText       = 1104
)

var (
	procShellExecute  = dllShell32.NewProc("ShellExecuteW")
	summaryClass      = "VaultOfflineRecoverySummaryV1"
	summaryRegistered = false
	summaryWin        = struct {
		hwnd, text, folderButton, reportButton, closeButton uintptr
		folder, report                                      string
	}{}
)

func shellOpenVerifiedPath(owner uintptr, p string) {
	if p == "" {
		return
	}
	result, _, _ := procShellExecute.Call(owner, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(p))), 0, 0, uintptr(swShow))
	if result <= 32 {
		msgBox(owner, "Windows could not open the selected file or folder.", "Open failed", mbOK|mbIconError)
	}
}

func summaryWindowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	switch msg {
	case wmCreate:
		summaryWin.hwnd = hwnd
		activateDarkTitle(hwnd)
		summaryWin.text = classCreate("EDIT", "", wsChild|wsVisible|wsBorder|wsVscroll|esMultiline|esAutoVscroll|esReadOnly, 14, 14, 720, 370, hwnd, idSummaryText)
		summaryWin.folderButton = classCreate("BUTTON", "Open recovered folder", wsChild|wsVisible|wsTabstop|bsOwnerDraw, 14, 400, 210, 34, hwnd, idSummaryOpenFolder)
		summaryWin.reportButton = classCreate("BUTTON", "View Vault JSON report", wsChild|wsVisible|wsTabstop|bsOwnerDraw, 240, 400, 210, 34, hwnd, idSummaryViewReport)
		summaryWin.closeButton = classCreate("BUTTON", "Close", wsChild|wsVisible|wsTabstop|bsOwnerDraw, 620, 400, 115, 34, hwnd, idSummaryClose)
		for _, v := range []uintptr{summaryWin.folderButton, summaryWin.reportButton, summaryWin.closeButton} {
			installThemedButton(v)
		}
		send(summaryWin.text, wmSetFont, app.font, 1)
		for _, v := range []uintptr{summaryWin.folderButton, summaryWin.reportButton, summaryWin.closeButton} {
			send(v, wmSetFont, app.font, 1)
		}
		return 0
	case wmEraseBkgnd:
		themeDialogBackdrop(hwnd, wp)
		return 1
	case wmCtlColorEdit, wmCtlColorStatic:
		return themeCtlColor(wp, msg, shadeText, false)
	case wmDrawItem:
		if themeDrawButton(lp) {
			return 1
		}
	case wmSize:
		w := int(uint16(lp & 0xffff))
		h := int(uint16((lp >> 16) & 0xffff))
		if w >= 400 && h >= 200 {
			move(summaryWin.text, 14, 14, w-28, h-84)
			move(summaryWin.folderButton, 14, h-55, 205, 32)
			move(summaryWin.reportButton, 231, h-55, 205, 32)
			move(summaryWin.closeButton, w-134, h-55, 120, 32)
		}
		return 0
	case wmCommand:
		switch int(wp & 0xffff) {
		case idSummaryOpenFolder:
			shellOpenVerifiedPath(hwnd, summaryWin.folder)
		case idSummaryViewReport:
			shellOpenVerifiedPath(hwnd, summaryWin.report)
		case idSummaryClose:
			procDestroy.Call(hwnd)
		}
		return 0
	case wmClose:
		procDestroy.Call(hwnd)
		return 0
	case wmDestroy:
		summaryWin.hwnd = 0
		return 0
	}
	v, _, _ := procDefWin.Call(hwnd, uintptr(msg), wp, lp)
	return v
}

// Dedicated read-only report viewer. The recovery engine has already
// confirmed the report path; the viewer never edits it or interprets JSON as
// recovery instructions.
func (a *appState) showRecoverySummary(r Report) {
	if summaryWin.hwnd != 0 {
		procDestroy.Call(summaryWin.hwnd)
	}
	dir := ""
	if len(r.Items) > 0 {
		candidate, err := canonicalPath(r.Items[0].Dir)
		base, e := canonicalPath(filepath.Dir(r.ReportPath))
		if err == nil && e == nil && pathIsWithin(base, candidate) {
			if fi, e := os.Stat(candidate); e == nil && fi.IsDir() {
				dir = candidate
			}
		}
	}
	summaryWin.folder = dir
	summaryWin.report = r.ReportPath
	instance, _, _ := procGetModule.Call(0)
	if !summaryRegistered {
		cls := wide(summaryClass)
		cursor, _, _ := procLoadCursor.Call(0, 32512)
		lg, sm := appWindowIcons(instance)
		wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: csHredraw | csVredraw, WndProc: syscall.NewCallback(summaryWindowProc), Instance: instance, Icon: lg, IconSm: sm, Cursor: cursor, ClassName: cls}
		ok, _, _ := procRegClass.Call(uintptr(unsafe.Pointer(&wc)))
		if ok == 0 {
			msgBox(a.hwnd, recoverySummaryText(r), "Vault recovery report confirmed", mbOK|mbIconInfo)
			return
		}
		summaryRegistered = true
	}
	cls := wide(summaryClass)
	hwnd, _, _ := procCreate.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(wide("Vault Recovery — Confirmed Summary"))), wsOverlappedWindow|wsClipChildren, 190, 140, 780, 520, a.hwnd, 0, instance, 0)
	if hwnd == 0 {
		msgBox(a.hwnd, recoverySummaryText(r), "Vault recovery report confirmed", mbOK|mbIconInfo)
		return
	}
	setText(summaryWin.text, recoverySummaryText(r))
	enabled(summaryWin.folderButton, dir != "")
	enabled(summaryWin.reportButton, r.ReportPath != "")
	procShow.Call(hwnd, swShow)
	procUpdate.Call(hwnd)
	runtime.KeepAlive(cls)
}
