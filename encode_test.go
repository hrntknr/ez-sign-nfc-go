package ezsignnfc

import (
	"image"
	"image/color"
	"testing"
)

func TestProfileBlockMath(t *testing.T) {
	tests := []struct {
		p      Product
		blocks int
		bpr    int
	}{
		{Product29Mono, 7, 37},
		{Product29Quad, 7, 74},
		{Product42Mono, 15, 50},
		{Product42Quad, 15, 100},
	}

	for _, tc := range tests {
		prof, err := ProfileByProduct(tc.p)
		if err != nil {
			t.Fatal(err)
		}
		if got := prof.BlockCount(); got != tc.blocks {
			t.Fatalf("%s block count: got %d want %d", tc.p, got, tc.blocks)
		}
		if got := prof.BytesPerRow(); got != tc.bpr {
			t.Fatalf("%s bytes per row: got %d want %d", tc.p, got, tc.bpr)
		}
	}
}

func TestEncodePixelsToAPDUs(t *testing.T) {
	prof, err := ProfileByProduct(Product42Quad)
	if err != nil {
		t.Fatal(err)
	}

	pixels := make([]uint8, prof.Width*prof.Height)
	for i := range pixels {
		pixels[i] = uint8(i % prof.Colors())
	}

	apdus, err := EncodePixelsToAPDUs(prof, pixels, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(apdus) == 0 {
		t.Fatal("expected apdus")
	}
	for i, apdu := range apdus {
		if len(apdu) < 7 {
			t.Fatalf("apdu %d too short", i)
		}
		if apdu[0] != 0xF0 || apdu[1] != 0xD3 {
			t.Fatalf("apdu %d bad header: %X", i, apdu[:2])
		}
		if int(apdu[4]) != len(apdu)-5 {
			t.Fatalf("apdu %d lc mismatch", i)
		}
	}
}

func TestQuantizeImageToPixelsWithOptionsDither(t *testing.T) {
	profile, err := ProfileByProduct(Product42Mono)
	if err != nil {
		t.Fatal(err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, profile.Width, profile.Height))
	for y := 0; y < profile.Height; y++ {
		for x := 0; x < profile.Width; x++ {
			img.Set(x, y, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}

	noDither := QuantizeImageToPixelsWithOptions(profile, img, ImageEncodeOptions{Dither: false})
	withDither := QuantizeImageToPixelsWithOptions(profile, img, ImageEncodeOptions{Dither: true})
	if len(noDither) != len(withDither) {
		t.Fatalf("length mismatch: %d != %d", len(noDither), len(withDither))
	}

	blackNoDither := 0
	blackWithDither := 0
	whiteWithDither := 0
	for i := range noDither {
		if noDither[i] == ColorBlack {
			blackNoDither++
		}
		if withDither[i] == ColorBlack {
			blackWithDither++
		}
		if withDither[i] == ColorWhite {
			whiteWithDither++
		}
	}

	if blackNoDither != 0 {
		t.Fatalf("expected no-dither gray to resolve to white only, black count=%d", blackNoDither)
	}
	if blackWithDither == 0 || whiteWithDither == 0 {
		t.Fatalf("expected dither output to contain both black and white, black=%d white=%d", blackWithDither, whiteWithDither)
	}
}
