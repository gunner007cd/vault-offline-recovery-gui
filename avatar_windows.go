//go:build windows

package main

// Nine embedded, offline mascot images are purely presentational.
// The original Vault executable alone decides recovery success.

import (
	"bytes"
	_ "embed"
	"image/png"
	"runtime"
	"unsafe"
)

//go:embed mascot_idle.png
var mascotIdlePNG []byte

//go:embed mascot_scan.png
var mascotScanPNG []byte

//go:embed mascot_restore.png
var mascotRestorePNG []byte

//go:embed mascot_waiting.png
var mascotWaitingPNG []byte

//go:embed mascot_extracting.png
var mascotExtractingPNG []byte

//go:embed mascot_success.png
var mascotSuccessPNG []byte

//go:embed mascot_failed.png
var mascotFailedPNG []byte

//go:embed mascot_cancelled.png
var mascotCancelledPNG []byte

//go:embed mascot_locked.png
var mascotLockedPNG []byte

const avatarPx = 192

var (
	procCreateDIBSection   = dllGdi32.NewProc("CreateDIBSection")
	procCreateCompatibleDC = dllGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = dllGdi32.NewProc("DeleteDC")
	procAlphaBlend         = dllMsimg32.NewProc("AlphaBlend")
	procGetDC              = dllUser32.NewProc("GetDC")
	procReleaseDC          = dllUser32.NewProc("ReleaseDC")
	avatarImages           [9]avatarNativeImage
)

type avatarNativeImage struct{ dc, bitmap, previous uintptr }

// Native Windows BITMAPINFOHEADER and one RGBQUAD (40+4 bytes).
// A negative height creates a top-down bitmap.
type avatarBitmapInfo struct {
	HeaderSize      uint32
	Width           int32
	Height          int32
	Planes          uint16
	BitCount        uint16
	Compression     uint32
	ImageSize       uint32
	XPelsPerMeter   int32
	YPelsPerMeter   int32
	UsedColors      uint32
	ImportantColors uint32
	UnusedRGBQuad   uint32
}

func setupAvatar() {
	for i, payload := range [][]byte{mascotIdlePNG, mascotScanPNG, mascotRestorePNG, mascotWaitingPNG, mascotExtractingPNG, mascotSuccessPNG, mascotFailedPNG, mascotCancelledPNG, mascotLockedPNG} {
		avatarImages[i] = createAvatarImage(payload)
	}
}

func createAvatarImage(pngBytes []byte) avatarNativeImage {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil || img.Bounds().Dx() != avatarPx || img.Bounds().Dy() != avatarPx {
		return avatarNativeImage{}
	}
	bmi := avatarBitmapInfo{HeaderSize: 40, Width: avatarPx, Height: -avatarPx, Planes: 1,
		BitCount: 32, Compression: 0, ImageSize: avatarPx * avatarPx * 4}
	var dibPixels uintptr
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return avatarNativeImage{}
	}
	defer procReleaseDC.Call(0, screenDC)
	bmp, _, _ := procCreateDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&dibPixels)), 0, 0)
	if bmp == 0 || dibPixels == 0 {
		return avatarNativeImage{}
	}

	// AlphaBlend expects premultiplied BGRA. image.Color.RGBA() supplies
	// premultiplied 16-bit channel values, converted here to 8-bit.
	pixels := make([]byte, avatarPx*avatarPx*4)
	b := img.Bounds()
	for y := 0; y < avatarPx; y++ {
		for x := 0; x < avatarPx; x++ {
			r, g, blue, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			off := (y*avatarPx + x) * 4
			pixels[off] = uint8(blue >> 8)
			pixels[off+1] = uint8(g >> 8)
			pixels[off+2] = uint8(r >> 8)
			pixels[off+3] = uint8(a >> 8)
		}
	}
	procCopyNativeMemory.Call(dibPixels, uintptr(unsafe.Pointer(&pixels[0])), uintptr(len(pixels)))
	runtime.KeepAlive(pixels)
	dc, _, _ := procCreateCompatibleDC.Call(screenDC)
	if dc == 0 {
		procDeleteObject.Call(bmp)
		return avatarNativeImage{}
	}
	previous, _, _ := procSelectObject.Call(dc, bmp)
	if previous == 0 {
		procDeleteDC.Call(dc)
		procDeleteObject.Call(bmp)
		return avatarNativeImage{}
	}
	return avatarNativeImage{dc: dc, bitmap: bmp, previous: previous}
}

// paintAvatar paints the current mascot next to the five top connection rows.
// The layout is unchanged; the image swap never moves or overlaps controls.
func paintAvatar(hdc uintptr, clientWidth int32) {
	state := app.mascot.phase
	if int(state) >= len(avatarImages) {
		state = mascotIdle
	}
	img := avatarImages[state]
	if img.dc == 0 {
		return
	}
	g := calculateTopFormGeometry(int(clientWidth))
	const blendFunction uintptr = (1 << 24) | (255 << 16)
	procAlphaBlend.Call(hdc, uintptr(g.PortraitX), uintptr(g.PortraitY), uintptr(g.PortraitSize), uintptr(g.PortraitSize),
		img.dc, 0, 0, avatarPx, avatarPx, blendFunction)
}

func releaseAvatar() {
	for i, img := range avatarImages {
		if img.dc != 0 {
			if img.previous != 0 {
				procSelectObject.Call(img.dc, img.previous)
			}
			procDeleteDC.Call(img.dc)
		}
		if img.bitmap != 0 {
			procDeleteObject.Call(img.bitmap)
		}
		avatarImages[i] = avatarNativeImage{}
	}
}
