package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	ezsignnfc "github.com/hrntknr/ez-sign-nfc-go"
)

func main() {
	var (
		mode         = flag.String("mode", "image", "image | random | checker | hstripe | vstripe")
		product      = flag.String("product", string(ezsignnfc.Product42Quad), "2.9-2c | 2.9-4c | 4.2-2c | 4.2-4c")
		reader       = flag.String("reader", "", "PC/SC reader name (default: first reader)")
		inputPath    = flag.String("input", "", "input image path (required in image mode)")
		crop         = flag.String("crop", "", "crop rectangle x,y,w,h before resize")
		dither       = flag.Bool("dither", false, "enable dithering in image mode")
		seed         = flag.Int64("seed", time.Now().UnixNano(), "random seed for random mode")
		pollMs       = flag.Int("poll-ms", 500, "refresh poll interval milliseconds")
		pollAttempts = flag.Int("poll-attempts", 60, "max refresh poll attempts")
	)
	flag.Parse()

	profile, err := ezsignnfc.ProfileByProduct(ezsignnfc.Product(*product))
	if err != nil {
		exitf("invalid product: %v", err)
	}

	var dev *ezsignnfc.Device
	if strings.TrimSpace(*reader) == "" {
		dev, err = ezsignnfc.Open(profile.Product)
	} else {
		dev, err = ezsignnfc.Open(profile.Product, ezsignnfc.ReaderName(*reader))
	}
	if err != nil {
		exitf("open device: %v", err)
	}
	defer dev.Close()

	if err := dev.SetPolling(time.Duration(*pollMs)*time.Millisecond, *pollAttempts); err != nil {
		exitf("invalid polling options: %v", err)
	}

	ctx := context.Background()
	fmt.Printf("reader: %s\n", dev.ReaderName())
	fmt.Printf("profile: %s (%dx%d, %d colors)\n", profile.Product, profile.Width, profile.Height, profile.Colors())

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "image":
		if *inputPath == "" {
			exitf("-input is required for image mode")
		}
		img, err := loadImage(*inputPath)
		if err != nil {
			exitf("load image: %v", err)
		}
		if *crop != "" {
			img, err = cropImage(img, *crop)
			if err != nil {
				exitf("crop image: %v", err)
			}
		}
		if err := dev.WriteImageWithOptions(ctx, img, ezsignnfc.ImageEncodeOptions{Dither: *dither}); err != nil {
			exitf("write image: %v", err)
		}
		fmt.Println("write complete")

	case "random":
		rng := rand.New(rand.NewSource(*seed))
		pixels := generateRandomPixels(profile, rng)
		if err := dev.WritePixels(ctx, pixels); err != nil {
			exitf("write random pixels: %v", err)
		}
		fmt.Println("write complete")

	case "checker":
		pixels := generateCheckerPixels(profile)
		if err := dev.WritePixels(ctx, pixels); err != nil {
			exitf("write checker pixels: %v", err)
		}
		fmt.Println("write complete")

	case "hstripe":
		pixels := generateHStripePixels(profile)
		if err := dev.WritePixels(ctx, pixels); err != nil {
			exitf("write hstripe pixels: %v", err)
		}
		fmt.Println("write complete")

	case "vstripe":
		pixels := generateVStripePixels(profile)
		if err := dev.WritePixels(ctx, pixels); err != nil {
			exitf("write vstripe pixels: %v", err)
		}
		fmt.Println("write complete")

	default:
		exitf("unsupported mode: %s", *mode)
	}
}

func generateRandomPixels(profile ezsignnfc.Profile, rng *rand.Rand) []uint8 {
	pixels := make([]uint8, profile.Width*profile.Height)
	for i := range pixels {
		pixels[i] = uint8(rng.Intn(profile.Colors()))
	}
	return pixels
}

func generateCheckerPixels(profile ezsignnfc.Profile) []uint8 {
	pixels := make([]uint8, profile.Width*profile.Height)
	colors := profile.Colors()
	for y := 0; y < profile.Height; y++ {
		for x := 0; x < profile.Width; x++ {
			c := (x + y) % 2
			if colors > 2 && ((x/16+y/16)%2 == 1) {
				c = 2 + ((x/32 + y/32) % 2)
			}
			pixels[y*profile.Width+x] = uint8(c % colors)
		}
	}
	return pixels
}

func generateHStripePixels(profile ezsignnfc.Profile) []uint8 {
	pixels := make([]uint8, profile.Width*profile.Height)
	colors := profile.Colors()
	band := profile.Height / 16
	if band < 1 {
		band = 1
	}
	for y := 0; y < profile.Height; y++ {
		c := (y / band) % colors
		for x := 0; x < profile.Width; x++ {
			pixels[y*profile.Width+x] = uint8(c)
		}
	}
	return pixels
}

func generateVStripePixels(profile ezsignnfc.Profile) []uint8 {
	pixels := make([]uint8, profile.Width*profile.Height)
	colors := profile.Colors()
	band := profile.Width / 16
	if band < 1 {
		band = 1
	}
	for x := 0; x < profile.Width; x++ {
		c := (x / band) % colors
		for y := 0; y < profile.Height; y++ {
			pixels[y*profile.Width+x] = uint8(c)
		}
	}
	return pixels
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func cropImage(img image.Image, spec string) (image.Image, error) {
	parts := strings.Split(spec, ",")
	if len(parts) != 4 {
		return nil, errors.New("crop format must be x,y,w,h")
	}
	vals := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid crop value %q: %w", p, err)
		}
		vals[i] = v
	}
	x, y, w, h := vals[0], vals[1], vals[2], vals[3]
	if w <= 0 || h <= 0 {
		return nil, errors.New("crop width/height must be > 0")
	}

	b := img.Bounds()
	r := image.Rect(x, y, x+w, y+h).Intersect(b)
	if r.Empty() {
		return nil, errors.New("crop rect outside image bounds")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for yy := 0; yy < r.Dy(); yy++ {
		for xx := 0; xx < r.Dx(); xx++ {
			dst.Set(xx, yy, img.At(r.Min.X+xx, r.Min.Y+yy))
		}
	}
	return dst, nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
