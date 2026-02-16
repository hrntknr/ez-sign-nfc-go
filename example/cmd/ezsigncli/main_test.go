package main

import (
	"math/rand"
	"testing"

	ezsignnfc "github.com/hrntknr/ez-sign-nfc-go"
)

func TestGeneratePatternPixelsRandomDeterministic(t *testing.T) {
	profile := ezsignnfc.PresetProfiles[ezsignnfc.Product29Quad]
	rng1 := rand.New(rand.NewSource(123))
	rng2 := rand.New(rand.NewSource(123))

	p1 := generateRandomPixels(profile, rng1)
	p2 := generateRandomPixels(profile, rng2)
	if len(p1) != profile.Width*profile.Height {
		t.Fatalf("pixel length #1: got %d", len(p1))
	}
	if len(p2) != profile.Width*profile.Height {
		t.Fatalf("pixel length #2: got %d", len(p2))
	}
	for i := range p1 {
		if p1[i] != p2[i] {
			t.Fatalf("determinism mismatch at index %d: %d != %d", i, p1[i], p2[i])
		}
		if int(p1[i]) >= profile.Colors() {
			t.Fatalf("color out of range at index %d: %d", i, p1[i])
		}
	}
}

func TestGeneratePatternPixelsModes(t *testing.T) {
	tests := []struct {
		name string
		p    ezsignnfc.Product
		gen  func(ezsignnfc.Profile) []uint8
	}{
		{name: "checker-2c", p: ezsignnfc.Product42Mono, gen: generateCheckerPixels},
		{name: "checker-4c", p: ezsignnfc.Product42Quad, gen: generateCheckerPixels},
		{name: "hstripe", p: ezsignnfc.Product42Quad, gen: generateHStripePixels},
		{name: "vstripe", p: ezsignnfc.Product42Quad, gen: generateVStripePixels},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := ezsignnfc.PresetProfiles[tc.p]
			pixels := tc.gen(profile)
			if len(pixels) != profile.Width*profile.Height {
				t.Fatalf("pixel length: got %d want %d", len(pixels), profile.Width*profile.Height)
			}
			for i, px := range pixels {
				if int(px) >= profile.Colors() {
					t.Fatalf("color out of range at index %d: %d", i, px)
				}
			}
		})
	}
}
