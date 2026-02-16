package ezsignnfc

import "fmt"

const blockRows = 20

// Product identifies supported EZ-Sign product variants.
type Product string

const (
	Product29Mono Product = "2.9-2c"
	Product29Quad Product = "2.9-4c"
	Product42Mono Product = "4.2-2c"
	Product42Quad Product = "4.2-4c"
)

// Profile describes per-product panel characteristics.
type Profile struct {
	Product      Product
	Width        int
	Height       int
	BitsPerPixel int
}

// PresetProfiles covers the 2x2 product matrix (size x color count).
var PresetProfiles = map[Product]Profile{
	// 2.9-inch panel family (296x128)
	Product29Mono: {Product: Product29Mono, Width: 296, Height: 128, BitsPerPixel: 1},
	Product29Quad: {Product: Product29Quad, Width: 296, Height: 128, BitsPerPixel: 2},
	// 4.2-inch panel family (400x300)
	Product42Mono: {Product: Product42Mono, Width: 400, Height: 300, BitsPerPixel: 1},
	Product42Quad: {Product: Product42Quad, Width: 400, Height: 300, BitsPerPixel: 2},
}

func ProfileByProduct(p Product) (Profile, error) {
	prof, ok := PresetProfiles[p]
	if !ok {
		return Profile{}, fmt.Errorf("unknown product %q", p)
	}
	return prof, nil
}

func (p Profile) Colors() int {
	if p.BitsPerPixel == 1 {
		return 2
	}
	return 4
}

func (p Profile) BlockCount() int {
	return (p.Height + blockRows - 1) / blockRows
}

func (p Profile) PixelsPerByte() int {
	if p.BitsPerPixel == 1 {
		return 8
	}
	return 4
}

func (p Profile) BytesPerRow() int {
	ppb := p.PixelsPerByte()
	return (p.Width + ppb - 1) / ppb
}
