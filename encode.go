package ezsignnfc

import (
	"fmt"
	"image"
	"math"
)

// ImageEncodeOptions configures image quantization behavior.
type ImageEncodeOptions struct {
	Dither bool
}

// EncodeImageToAPDUs quantizes an image to panel colors and returns F0D3 APDUs.
func EncodeImageToAPDUs(profile Profile, img image.Image, maxFragment int) ([][]byte, error) {
	return EncodeImageToAPDUsWithOptions(profile, img, maxFragment, ImageEncodeOptions{})
}

// EncodeImageToAPDUsWithOptions quantizes an image with options and returns F0D3 APDUs.
func EncodeImageToAPDUsWithOptions(profile Profile, img image.Image, maxFragment int, opts ImageEncodeOptions) ([][]byte, error) {
	pixels := QuantizeImageToPixelsWithOptions(profile, img, opts)
	return EncodePixelsToAPDUs(profile, pixels, maxFragment)
}

// EncodePixelsToAPDUs packs indexed pixels into panel blocks and returns F0D3 APDUs.
func EncodePixelsToAPDUs(profile Profile, pixels []uint8, maxFragment int) ([][]byte, error) {
	if maxFragment <= 0 || maxFragment > 250 {
		return nil, fmt.Errorf("maxFragment must be 1..250: %d", maxFragment)
	}
	if err := validatePixels(profile, pixels); err != nil {
		return nil, err
	}

	blocks, err := packPixelsToBlocks(profile, pixels)
	if err != nil {
		return nil, err
	}

	apdus := make([][]byte, 0, len(blocks)*4)
	for blockNo, raw := range blocks {
		compressed, err := compressLZO1XLiteral(raw)
		if err != nil {
			return nil, fmt.Errorf("compress block %d: %w", blockNo, err)
		}
		frags := splitBytes(compressed, maxFragment)
		for fragNo, frag := range frags {
			isLast := fragNo == len(frags)-1
			apdu, err := buildImageDataAPDU(blockNo, fragNo, frag, isLast)
			if err != nil {
				return nil, err
			}
			apdus = append(apdus, apdu)
		}
	}

	return apdus, nil
}

// QuantizeImageToPixels resizes/crops to panel size and quantizes to indexed colors.
func QuantizeImageToPixels(profile Profile, img image.Image) []uint8 {
	return QuantizeImageToPixelsWithOptions(profile, img, ImageEncodeOptions{})
}

// QuantizeImageToPixelsWithOptions resizes/crops and quantizes to indexed colors with options.
func QuantizeImageToPixelsWithOptions(profile Profile, img image.Image, opts ImageEncodeOptions) []uint8 {
	prepared := ResizeCropNearest(img, profile.Width, profile.Height)
	prepared = enhanceForEpaper(profile, prepared)
	if opts.Dither {
		return quantizeImageToPixelsDither(profile, prepared)
	}
	pixels := make([]uint8, profile.Width*profile.Height)
	idx := 0
	for y := 0; y < profile.Height; y++ {
		for x := 0; x < profile.Width; x++ {
			pixels[idx] = quantizeColor(profile, prepared.At(x, y))
			idx++
		}
	}
	return pixels
}

func quantizeImageToPixelsDither(profile Profile, img *image.NRGBA) []uint8 {
	width := profile.Width
	height := profile.Height
	palette := paletteForProfile(profile)
	size := width * height

	rs := make([]float64, size)
	gs := make([]float64, size)
	bs := make([]float64, size)
	for y := 0; y < height; y++ {
		row := img.Pix[y*img.Stride:]
		for x := 0; x < width; x++ {
			pix := x * 4
			i := y*width + x
			rs[i] = float64(row[pix+0])
			gs[i] = float64(row[pix+1])
			bs[i] = float64(row[pix+2])
		}
	}

	pixels := make([]uint8, size)
	for y := 0; y < height; y++ {
		xStart := 0
		xEnd := width
		step := 1
		if y%2 == 1 {
			xStart = width - 1
			xEnd = -1
			step = -1
		}

		for x := xStart; x != xEnd; x += step {
			i := y*width + x
			r := clampByteFloat(rs[i])
			g := clampByteFloat(gs[i])
			b := clampByteFloat(bs[i])
			c := nearestPaletteIndex(profile, r, g, b)
			pixels[i] = c

			pc := palette[int(c)]
			er := float64(r) - float64(pc.R)
			eg := float64(g) - float64(pc.G)
			eb := float64(b) - float64(pc.B)

			diffuse := func(nx, ny int, factor float64) {
				if nx < 0 || nx >= width || ny < 0 || ny >= height {
					return
				}
				ni := ny*width + nx
				rs[ni] += er * factor
				gs[ni] += eg * factor
				bs[ni] += eb * factor
			}

			if step == 1 {
				diffuse(x+1, y, 7.0/16.0)
				diffuse(x-1, y+1, 3.0/16.0)
				diffuse(x, y+1, 5.0/16.0)
				diffuse(x+1, y+1, 1.0/16.0)
			} else {
				diffuse(x-1, y, 7.0/16.0)
				diffuse(x+1, y+1, 3.0/16.0)
				diffuse(x, y+1, 5.0/16.0)
				diffuse(x-1, y+1, 1.0/16.0)
			}
		}
	}
	return pixels
}

func clampByteFloat(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func enhanceForEpaper(profile Profile, src *image.NRGBA) *image.NRGBA {
	const (
		lowPercent  = 0.10
		highPercent = 0.90
	)

	const gamma = 0.90
	satBoost := 1.0
	if profile.BitsPerPixel == 2 {
		satBoost = 1.5
	}

	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w == 0 || h == 0 {
		return src
	}

	hist := [256]int{}
	total := w * h
	for y := 0; y < h; y++ {
		row := src.Pix[y*src.Stride:]
		for x := 0; x < w; x++ {
			i := x * 4
			l := lumaByte(row[i+0], row[i+1], row[i+2])
			hist[int(l)]++
		}
	}

	lowTarget := int(float64(total) * lowPercent)
	highTarget := int(float64(total) * highPercent)
	low := 0
	high := 255
	cum := 0
	for i := 0; i < 256; i++ {
		cum += hist[i]
		if cum >= lowTarget {
			low = i
			break
		}
	}
	cum = 0
	for i := 0; i < 256; i++ {
		cum += hist[i]
		if cum >= highTarget {
			high = i
			break
		}
	}
	if high <= low {
		return src
	}

	dst := image.NewNRGBA(src.Bounds())
	scale := 255.0 / float64(high-low)
	for y := 0; y < h; y++ {
		srcRow := src.Pix[y*src.Stride:]
		dstRow := dst.Pix[y*dst.Stride:]
		for x := 0; x < w; x++ {
			i := x * 4
			r := applyLevels(srcRow[i+0], low, scale, gamma)
			g := applyLevels(srcRow[i+1], low, scale, gamma)
			b := applyLevels(srcRow[i+2], low, scale, gamma)

			if satBoost > 1.0 {
				gray := (float64(r) + float64(g) + float64(b)) / 3.0
				r = clampByteFloat(gray + (float64(r)-gray)*satBoost)
				g = clampByteFloat(gray + (float64(g)-gray)*satBoost)
				b = clampByteFloat(gray + (float64(b)-gray)*satBoost)
			}

			dstRow[i+0] = r
			dstRow[i+1] = g
			dstRow[i+2] = b
			dstRow[i+3] = srcRow[i+3]
		}
	}
	return dst
}

func applyLevels(v uint8, low int, scale float64, gamma float64) uint8 {
	normalized := (float64(int(v)-low) * scale) / 255.0
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}
	return clampByteFloat(255.0 * math.Pow(normalized, gamma))
}

func lumaByte(r, g, b uint8) uint8 {
	y := (77*int(r) + 150*int(g) + 29*int(b)) >> 8
	if y < 0 {
		return 0
	}
	if y > 255 {
		return 255
	}
	return uint8(y)
}

// ResizeCropNearest scales with aspect-ratio preservation and center-crops.
func ResizeCropNearest(img image.Image, targetW, targetH int) *image.NRGBA {
	srcBounds := img.Bounds()
	sw := srcBounds.Dx()
	sh := srcBounds.Dy()
	if sw <= 0 || sh <= 0 || targetW <= 0 || targetH <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	}

	scaleX := float64(targetW) / float64(sw)
	scaleY := float64(targetH) / float64(sh)
	scale := scaleX
	if scaleY > scale {
		scale = scaleY
	}

	scaledW := int(float64(sw)*scale + 0.5)
	scaledH := int(float64(sh)*scale + 0.5)
	if scaledW < targetW {
		scaledW = targetW
	}
	if scaledH < targetH {
		scaledH = targetH
	}
	offsetX := (scaledW - targetW) / 2
	offsetY := (scaledH - targetH) / 2

	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			sx := int(float64(x+offsetX) / scale)
			sy := int(float64(y+offsetY) / scale)
			if sx < 0 {
				sx = 0
			} else if sx >= sw {
				sx = sw - 1
			}
			if sy < 0 {
				sy = 0
			} else if sy >= sh {
				sy = sh - 1
			}
			dst.Set(x, y, img.At(srcBounds.Min.X+sx, srcBounds.Min.Y+sy))
		}
	}
	return dst
}

func packPixelsToBlocks(profile Profile, pixels []uint8) ([][]byte, error) {
	bytesPerRow := profile.BytesPerRow()
	blockSize := bytesPerRow * blockRows
	blocks := make([][]byte, profile.BlockCount())

	for b := 0; b < profile.BlockCount(); b++ {
		block := make([]byte, 0, blockSize)
		for by := 0; by < blockRows; by++ {
			y := b*blockRows + by
			row := make([]byte, profile.Width)
			if y < profile.Height {
				copy(row, pixels[y*profile.Width:(y+1)*profile.Width])
			} else {
				for i := range row {
					row[i] = ColorWhite
				}
			}
			packed := packRowRightToLeft(profile, row)
			block = append(block, packed...)
		}
		blocks[b] = block
	}

	return blocks, nil
}

func packRowRightToLeft(profile Profile, row []uint8) []byte {
	if profile.BitsPerPixel == 1 {
		return packRow1bppRightToLeft(profile, row)
	}
	return packRow2bppRightToLeft(profile, row)
}

func packRow1bppRightToLeft(profile Profile, row []uint8) []byte {
	out := make([]byte, profile.BytesPerRow())
	pixel := 0
	for bi := 0; bi < len(out); bi++ {
		var v byte
		for bit := 0; bit < 8; bit++ {
			x := profile.Width - 1 - pixel
			px := ColorWhite
			if x >= 0 && x < len(row) {
				px = row[x]
			}
			v |= (px & 0x01) << uint(bit)
			pixel++
		}
		out[bi] = v
	}
	return out
}

func packRow2bppRightToLeft(profile Profile, row []uint8) []byte {
	out := make([]byte, profile.BytesPerRow())
	pixel := 0
	for bi := 0; bi < len(out); bi++ {
		var v byte
		for nib := 0; nib < 4; nib++ {
			x := profile.Width - 1 - pixel
			px := ColorWhite
			if x >= 0 && x < len(row) {
				px = row[x]
			}
			v |= (px & 0x03) << uint(6-2*nib)
			pixel++
		}
		out[bi] = v
	}
	return out
}

func splitBytes(src []byte, n int) [][]byte {
	if len(src) == 0 {
		return [][]byte{{}}
	}
	out := make([][]byte, 0, (len(src)+n-1)/n)
	for len(src) > 0 {
		take := n
		if take > len(src) {
			take = len(src)
		}
		frag := make([]byte, take)
		copy(frag, src[:take])
		out = append(out, frag)
		src = src[take:]
	}
	return out
}
