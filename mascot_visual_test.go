package main

import (
	"image/png"
	"os"
	"testing"
)

func TestMascotAssetsAreSameDimensionsAndTransparent(t *testing.T) {
	for _, n := range []string{"idle", "scan", "restore", "waiting", "extracting", "success", "failed", "cancelled", "locked"} {
		f, e := os.Open("mascot_" + n + ".png")
		if e != nil {
			t.Fatal(e)
		}
		img, e := png.Decode(f)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		if img.Bounds().Dx() != 192 || img.Bounds().Dy() != 192 {
			t.Fatalf("%s wrong size", n)
		}
		_, _, _, corner := img.At(0, 0).RGBA()
		_, _, _, center := img.At(96, 96).RGBA()
		if corner != 0 || center == 0 {
			t.Fatalf("%s alpha incorrect", n)
		}
	}
}
