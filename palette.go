package ezsignnfc

import (
	"fmt"
	"image/color"
)

const (
	ColorBlack  uint8 = 0
	ColorWhite  uint8 = 1
	ColorYellow uint8 = 2
	ColorRed    uint8 = 3
)

var monoPalette = []color.NRGBA{
	{R: 0, G: 0, B: 0, A: 255},
	{R: 255, G: 255, B: 255, A: 255},
}

var quadPalette = []color.NRGBA{
	{R: 0, G: 0, B: 0, A: 255},
	{R: 255, G: 255, B: 255, A: 255},
	{R: 255, G: 255, B: 0, A: 255},
	{R: 255, G: 0, B: 0, A: 255},
}

func paletteForProfile(profile Profile) []color.NRGBA {
	if profile.BitsPerPixel == 1 {
		return monoPalette
	}
	return quadPalette
}

func validatePixels(profile Profile, pixels []uint8) error {
	if len(pixels) != profile.Width*profile.Height {
		return fmt.Errorf("invalid pixel length: got %d, want %d", len(pixels), profile.Width*profile.Height)
	}
	limit := uint8(profile.Colors() - 1)
	for i, px := range pixels {
		if px > limit {
			return fmt.Errorf("invalid pixel index at %d: got %d, max %d", i, px, limit)
		}
	}
	return nil
}

func quantizeColor(profile Profile, c color.Color) uint8 {
	r16, g16, b16, _ := c.RGBA()
	return nearestPaletteIndexRGB(paletteForProfile(profile), uint8(r16>>8), uint8(g16>>8), uint8(b16>>8))
}

func nearestPaletteIndexRGB(palette []color.NRGBA, r, g, b uint8) uint8 {
	best := 0
	bestDist := int(^uint(0) >> 1)
	for i, p := range palette {
		dr := int(r) - int(p.R)
		dg := int(g) - int(p.G)
		db := int(b) - int(p.B)
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return uint8(best)
}
