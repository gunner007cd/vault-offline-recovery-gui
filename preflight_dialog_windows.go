//go:build windows

package main

// Native, explicitly modal pre-restore review. Presentation only: it never
// touches the repository or recovery engine. A failed window creation,
// ESC/close, or any non-affirmative result means NO restore.

import (
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	idPreflightYes  = 1211
	idPreflightNo   = 1212
	idPreflightText = 1213
	wmKeyDown       = 0x0100
	vkEscape        = 0x1b
	wsCaption       = 0x00c00000
	wsSysMenu       = 0x00080000
)

var (
	preflightClass           = "VaultRecoveryPreflightDarkV1"
	preflightClassRegistered bool
	procSetFocus             = dllUser32.NewProc("SetFocus")
	procSetActiveWindow      = dllUser32.NewProc("SetActiveWindow")
	procIsDialogMessage      = dllUser32.NewProc("IsDialogMessageW")
	preflightWindow          = struct {
		hwnd, owner, body, yesButton, noButton uintptr
		title, subtitle                        uintptr
		accepted                               bool
		text                                   string
	}{}
)

func preflightWindowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	p := &preflightWindow
	switch msg {
	case wmCreate:
		p.hwnd = hwnd
		activateDarkTitle(hwnd)
		p.title = classCreate("STATIC", "Review recovery", wsChild|wsVisible|ssOwnerDraw, 18, 13, 680, 34, hwnd, 0)
		p.subtitle = classCreate("STATIC", "Files will be written only after you approve", wsChild|wsVisible|ssOwnerDraw, 18, 46, 680, 22, hwnd, 0)
		p.body = classCreate("EDIT", "", wsChild|wsVisible|wsBorder|wsVscroll|esMultiline|esAutoVscroll|esReadOnly, 18, 92, 704, 342, hwnd, idPreflightText)
		p.yesButton = classCreate("BUTTON", "Start restore", wsChild|wsVisible|wsTabstop|bsOwnerDraw, 416, 450, 145, 35, hwnd, idPreflightYes)
		p.noButton = classCreate("BUTTON", "Cancel", wsChild|wsVisible|wsTabstop|bsOwnerDraw, 573, 450, 150, 35, hwnd, idPreflightNo)
		for _, h := range []uintptr{p.yesButton, p.noButton} {
			installThemedButton(h)
			send(h, wmSetFont, app.font, 1)
		}
		for _, h := range []uintptr{p.title, p.subtitle, p.body} {
			send(h, wmSetFont, app.font, 1)
		}
		return 0
	case wmEraseBkgnd:
		themeDialogBackdrop(hwnd, wp)
		return 1
	case wmCtlColorEdit, wmCtlColorStatic:
		if lp == p.body {
			// Windows may send WM_CTLCOLORSTATIC for read-only edit controls.
			return themeCtlColor(wp, wmCtlColorEdit, shadeText, false)
		}
		return themeCtlColor(wp, msg, shadeText, false)
	case wmDrawItem:
		labels := map[string]uintptr{"title": p.title, "mode": p.subtitle}
		if drawThemedLabel(lp, labels, hwnd, app.font, app.titleFont) || themeDrawButton(lp) {
			return 1
		}
	case wmSize:
		w := int(uint16(lp & 0xffff))
		h := int(uint16((lp >> 16) & 0xffff))
		if w >= 450 && h >= 220 {
			move(p.title, 18, 12, w-36, 35)
			move(p.subtitle, 18, 48, w-36, 21)
			move(p.body, 18, 94, w-36, h-169)
			move(p.yesButton, w-324, h-59, 144, 34)
			move(p.noButton, w-169, h-59, 150, 34)
		}
		return 0
	case wmCommand:
		if int(wp&0xffff) == idPreflightYes {
			p.accepted = true
			procDestroy.Call(hwnd)
		} else if int(wp&0xffff) == idPreflightNo {
			procDestroy.Call(hwnd)
		}
		return 0
	case wmKeyDown:
		if wp == vkEscape {
			procDestroy.Call(hwnd)
			return 0
		}
	case wmClose:
		procDestroy.Call(hwnd) // closing always means NO
		return 0
	case wmDestroy:
		p.hwnd = 0 // nested modal message loop exits
		return 0
	}
	ret, _, _ := procDefWin.Call(hwnd, uintptr(msg), wp, lp)
	return ret
}

// showPreflightReview is fail-closed. The caller has already checked the
// command arguments, selection and space estimates; this only asks for a
// further explicit Yes before calling the original Vault executable.
func showPreflightReview(owner uintptr, review string) bool {
	if owner == 0 || preflightWindow.hwnd != 0 {
		return false
	}
	p := &preflightWindow
	p.accepted = false
	p.owner = owner
	p.text = review
	inst, _, _ := procGetModule.Call(0)
	if !preflightClassRegistered {
		name := wide(preflightClass)
		cursor, _, _ := procLoadCursor.Call(0, 32512)
		large, small := appWindowIcons(inst)
		wc := wndClassEx{
			Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: csHredraw | csVredraw,
			WndProc: syscall.NewCallback(preflightWindowProc), Instance: inst,
			Icon: large, IconSm: small, Cursor: cursor, ClassName: name,
		}
		ok, _, _ := procRegClass.Call(uintptr(unsafe.Pointer(&wc)))
		if ok == 0 {
			return false
		}
		preflightClassRegistered = true
	}
	const width, height int32 = 760, 575
	// Center on the parent; do not take over the entire desktop.
	var ownerRect themeRect
	procGetWindowRect.Call(owner, uintptr(unsafe.Pointer(&ownerRect)))
	x := ownerRect.Left + ((ownerRect.Right-ownerRect.Left)-width)/2
	y := ownerRect.Top + ((ownerRect.Bottom-ownerRect.Top)-height)/2
	name := wide(preflightClass)
	title := wide("Vault Recovery — Pre-Restore Review")
	hwnd, _, _ := procCreate.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(title)), wsCaption|wsSysMenu|wsClipChildren, uintptr(x), uintptr(y), uintptr(width), uintptr(height), owner, 0, inst, 0)
	if hwnd == 0 {
		return false
	}
	review = strings.TrimPrefix(review, "PRE-RESTORE REVIEW — files WILL be written.\n\n")
	setText(p.body, strings.ReplaceAll(review, "\n", "\r\n"))
	// The owner is blocked while this dialog is displayed, just like a
	// standard Windows confirmation box. Focus defaults to CANCEL, never Yes.
	procEnable.Call(owner, 0)
	procShow.Call(hwnd, swShow)
	procUpdate.Call(hwnd)
	procSetFocus.Call(p.noButton)
	for p.hwnd != 0 {
		var msg winMsg
		got, _, _ := procGetMsg.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(got) <= 0 {
			if int32(got) == 0 {
				procPostQuit.Call(msg.WParam)
			}
			// Error or WM_QUIT; definitely do not authorize a restore.
			p.accepted = false
			if p.hwnd != 0 {
				procDestroy.Call(p.hwnd)
			}
			break
		}
		if msg.Message == wmKeyDown && msg.WParam == vkEscape && (msg.Hwnd == hwnd || msg.Hwnd == p.body || msg.Hwnd == p.yesButton || msg.Hwnd == p.noButton) {
			procDestroy.Call(hwnd)
			continue
		}
		handled, _, _ := procIsDialogMessage.Call(hwnd, uintptr(unsafe.Pointer(&msg)))
		if handled == 0 {
			procTranslate.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatch.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
	procEnable.Call(owner, 1)
	procSetActiveWindow.Call(owner)
	approved := p.accepted
	p.accepted = false
	p.owner = 0
	p.body, p.yesButton, p.noButton, p.title, p.subtitle = 0, 0, 0, 0, 0
	p.text = ""
	runtime.KeepAlive(name)
	runtime.KeepAlive(title)
	return approved
}
