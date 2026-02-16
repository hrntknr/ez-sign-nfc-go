package ezsignnfc

import "fmt"

var (
	apduAuthenticate = []byte{0x00, 0x20, 0x00, 0x01, 0x04, 0x20, 0x09, 0x12, 0x10}
	apduStartRefresh = []byte{0xF0, 0xD4, 0x85, 0x80, 0x00}
	apduPollStatus   = []byte{0xF0, 0xDE, 0x00, 0x00, 0x01}
)

func buildImageDataAPDU(blockNo int, fragNo int, payload []byte, isLast bool) ([]byte, error) {
	if blockNo < 0 || blockNo > 0xFF {
		return nil, fmt.Errorf("blockNo out of range: %d", blockNo)
	}
	if fragNo < 0 || fragNo > 0xFF {
		return nil, fmt.Errorf("fragNo out of range: %d", fragNo)
	}
	if len(payload) > 250 {
		return nil, fmt.Errorf("payload too large: %d", len(payload))
	}

	p2 := byte(0x00)
	if isLast {
		p2 = 0x01
	}
	lc := byte(2 + len(payload))
	apdu := make([]byte, 0, 5+int(lc))
	apdu = append(apdu, 0xF0, 0xD3, 0x00, p2, lc, byte(blockNo), byte(fragNo))
	apdu = append(apdu, payload...)
	return apdu, nil
}
