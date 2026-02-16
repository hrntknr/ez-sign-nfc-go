package ezsignnfc

import (
	"fmt"
	"image/color"
	"math"
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
	return nearestPaletteIndex(profile, uint8(r16>>8), uint8(g16>>8), uint8(b16>>8))
}

func nearestPaletteIndex(profile Profile, r, g, b uint8) uint8 {
	return nearestPaletteIndexRGB(profile, paletteForProfile(profile), r, g, b)
}

func nearestPaletteIndexRGB(profile Profile, palette []color.NRGBA, r, g, b uint8) uint8 {
	if profile.BitsPerPixel == 2 {
		return nearestQuadPaletteIndex(r, g, b)
	}

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

func nearestQuadPaletteIndex(r, g, b uint8) uint8 {
	// Base distances to black/white/yellow/red.
	dBlack := colorDistSq(r, g, b, 0, 0, 0)
	dWhite := colorDistSq(r, g, b, 255, 255, 255)
	dYellow := colorDistSq(r, g, b, 255, 255, 0)
	dRed := colorDistSq(r, g, b, 255, 0, 0)

	maxC := float64(max3(int(r), int(g), int(b)))
	minC := float64(min3(int(r), int(g), int(b)))
	sat := 0.0
	if maxC > 0 {
		sat = (maxC - minC) / maxC
	}

	// For saturated colors, avoid collapsing too aggressively into black/white.
	penaltyGray := int(7000.0 * sat * sat)
	dBlack += penaltyGray
	dWhite += penaltyGray

	// Hue-directed bonus so warm tones keep yellow/red better on 4-color panels.
	if sat > 0.12 {
		if int(r) > int(g)+18 && int(r) > int(b)+18 {
			dRed -= int(2200.0 * sat)
		}
		if int(r) > 95 && int(g) > 95 && int(b) < 165 {
			dYellow -= int(2200.0 * sat)
		}
	}

	// Gentle luminance balancing: strong dark tones should remain black,
	// and very bright tones should remain white unless strongly chromatic.
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luma < 58.0 {
		dBlack -= int(1200.0 * (1.0 - luma/58.0))
	}
	if luma > 220.0 {
		dWhite -= int(1400.0 * ((luma - 220.0) / 35.0))
	}

	bestIdx := ColorBlack
	bestDist := dBlack
	if dWhite < bestDist {
		bestDist = dWhite
		bestIdx = ColorWhite
	}
	if dYellow < bestDist {
		bestDist = dYellow
		bestIdx = ColorYellow
	}
	if dRed < bestDist {
		bestIdx = ColorRed
	}
	return bestIdx
}

func colorDistSq(r, g, b uint8, pr, pg, pb uint8) int {
	dr := int(r) - int(pr)
	dg := int(g) - int(pg)
	db := int(b) - int(pb)
	return dr*dr + dg*dg + db*db
}

func max3(a, b, c int) int {
	return int(math.Max(float64(a), math.Max(float64(b), float64(c))))
}

func min3(a, b, c int) int {
	return int(math.Min(float64(a), math.Min(float64(b), float64(c))))
}
