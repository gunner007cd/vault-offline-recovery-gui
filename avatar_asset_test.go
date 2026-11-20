package main

import (
	"image/png"
	"os"
	"testing"
)

func TestPortraitAvatarIsLargeAndTransparent(t *testing.T) {
	f, err := os.Open("header_avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 192 || img.Bounds().Dy() != 192 {
		t.Fatalf("portrait avatar must fit 192px form column: %v", img.Bounds())
	}
	_, _, _, cornerA := img.At(0, 0).RGBA()
	_, _, _, centerA := img.At(96, 96).RGBA()
	if cornerA != 0 || centerA == 0 {
		t.Fatalf("avatar must have transparent exterior and an opaque center: corner %d, center %d", cornerA, centerA)
	}
}
