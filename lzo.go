package ezsignnfc

import "fmt"

// compressLZO1XLiteral emits a valid LZO1X stream using literal runs only.
// It is compatible with LZO1X decoders, while not aiming for compression ratio.
func compressLZO1XLiteral(src []byte) ([]byte, error) {
	if len(src) < 4 {
		return nil, fmt.Errorf("source too short for literal-only LZO stream: %d", len(src))
	}

	out := make([]byte, 0, len(src)+32)
	t := len(src)

	if t <= 238 {
		out = append(out, byte(17+t))
	} else {
		out = append(out, 0)
		out = appendLZOMulti(out, t-18)
	}
	out = append(out, src...)
	out = append(out, 17, 0, 0)
	return out, nil
}

func appendLZOMulti(out []byte, t int) []byte {
	for t > 255 {
		out = append(out, 0)
		t -= 255
	}
	out = append(out, byte(t))
	return out
}
