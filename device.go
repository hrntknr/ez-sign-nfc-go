package ezsignnfc

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/ebfe/scard"
)

// Device is an active PC/SC connection to EZ-Sign.
type Device struct {
	ctx            *scard.Context
	card           *scard.Card
	reader         string
	profile        Profile
	maxFragment    int
	pollInterval   time.Duration
	maxPollAttempt int
}

// ReaderSelector chooses one reader from detected PC/SC readers.
type ReaderSelector interface {
	selectReader(readers []string) (string, error)
}

type readerIndexSelector int
type readerNameSelector string

// ReaderIndex selects a reader by zero-based index from ListReaders.
func ReaderIndex(index int) ReaderSelector {
	return readerIndexSelector(index)
}

// ReaderName selects a reader by exact PC/SC reader name.
func ReaderName(name string) ReaderSelector {
	return readerNameSelector(name)
}

func (s readerIndexSelector) selectReader(readers []string) (string, error) {
	idx := int(s)
	if idx < 0 || idx >= len(readers) {
		return "", fmt.Errorf("reader index out of range: %d (readers=%d)", idx, len(readers))
	}
	return readers[idx], nil
}

func (s readerNameSelector) selectReader(readers []string) (string, error) {
	name := string(s)
	if name == "" {
		return "", fmt.Errorf("reader name must not be empty")
	}
	for _, r := range readers {
		if r == name {
			return r, nil
		}
	}
	return "", fmt.Errorf("reader name not found: %q", name)
}

// ListReaders returns currently available PC/SC reader names.
func ListReaders() ([]string, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, fmt.Errorf("establish pc/sc context: %w", err)
	}
	defer ctx.Release()

	readers, err := ctx.ListReaders()
	if err != nil {
		return nil, fmt.Errorf("list readers: %w", err)
	}
	return readers, nil
}

// Open opens a device for a preset product profile.
// When no selector is provided, it opens the first available reader.
func Open(product Product, selectors ...ReaderSelector) (*Device, error) {
	profile, err := ProfileByProduct(product)
	if err != nil {
		return nil, err
	}
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, fmt.Errorf("establish pc/sc context: %w", err)
	}

	readers, err := ctx.ListReaders()
	if err != nil {
		ctx.Release()
		return nil, fmt.Errorf("list readers: %w", err)
	}
	if len(readers) == 0 {
		ctx.Release()
		return nil, fmt.Errorf("no pc/sc readers found")
	}

	reader, err := resolveReader(readers, selectors)
	if err != nil {
		ctx.Release()
		return nil, err
	}

	card, err := ctx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		ctx.Release()
		return nil, fmt.Errorf("connect reader %q: %w", reader, err)
	}

	return &Device{
		ctx:            ctx,
		card:           card,
		reader:         reader,
		profile:        profile,
		maxFragment:    250,
		pollInterval:   500 * time.Millisecond,
		maxPollAttempt: 60,
	}, nil
}

func resolveReader(readers []string, selectors []ReaderSelector) (string, error) {
	if len(selectors) == 0 {
		return readers[0], nil
	}
	if len(selectors) != 1 {
		return "", fmt.Errorf("open accepts at most one reader selector")
	}
	if selectors[0] == nil {
		return "", fmt.Errorf("reader selector must not be nil")
	}
	return selectors[0].selectReader(readers)
}

func (d *Device) ReaderName() string {
	return d.reader
}

func (d *Device) SetMaxFragment(n int) error {
	if n <= 0 || n > 250 {
		return fmt.Errorf("max fragment must be 1..250")
	}
	d.maxFragment = n
	return nil
}

func (d *Device) SetPolling(interval time.Duration, attempts int) error {
	if interval <= 0 {
		return fmt.Errorf("poll interval must be > 0")
	}
	if attempts <= 0 {
		return fmt.Errorf("poll attempts must be > 0")
	}
	d.pollInterval = interval
	d.maxPollAttempt = attempts
	return nil
}

func (d *Device) Close() error {
	var firstErr error
	if d.card != nil {
		if err := d.card.Disconnect(scard.ResetCard); err != nil {
			firstErr = err
		}
		d.card = nil
	}
	if d.ctx != nil {
		if err := d.ctx.Release(); err != nil && firstErr == nil {
			firstErr = err
		}
		d.ctx = nil
	}
	return firstErr
}

func (d *Device) WriteImage(ctx context.Context, img image.Image) error {
	return d.WriteImageWithOptions(ctx, img, ImageEncodeOptions{})
}

func (d *Device) WriteImageWithOptions(ctx context.Context, img image.Image, opts ImageEncodeOptions) error {
	apdus, err := EncodeImageToAPDUsWithOptions(d.profile, img, d.maxFragment, opts)
	if err != nil {
		return err
	}
	return d.writeAPDUs(ctx, apdus)
}

func (d *Device) WritePixels(ctx context.Context, pixels []uint8) error {
	apdus, err := EncodePixelsToAPDUs(d.profile, pixels, d.maxFragment)
	if err != nil {
		return err
	}
	return d.writeAPDUs(ctx, apdus)
}

func (d *Device) writeAPDUs(ctx context.Context, imageDataAPDUs [][]byte) error {
	if err := d.bootstrap(ctx); err != nil {
		return err
	}
	for i, apdu := range imageDataAPDUs {
		if err := d.checkContext(ctx); err != nil {
			return err
		}
		if _, err := d.transmitExpect9000(apdu); err != nil {
			return fmt.Errorf("send image apdu %d/%d: %w", i+1, len(imageDataAPDUs), err)
		}
	}
	if _, err := d.transmitExpect9000(apduStartRefresh); err != nil {
		return fmt.Errorf("start refresh: %w", err)
	}
	return d.pollRefreshDone(ctx)
}

func (d *Device) bootstrap(ctx context.Context) error {
	if err := d.checkContext(ctx); err != nil {
		return err
	}
	if _, err := d.transmitExpect9000(apduAuthenticate); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	return nil
}

func (d *Device) pollRefreshDone(ctx context.Context) error {
	for i := 0; i < d.maxPollAttempt; i++ {
		if err := d.checkContext(ctx); err != nil {
			return err
		}
		data, err := d.transmitExpect9000(apduPollStatus)
		if err != nil {
			return fmt.Errorf("poll status #%d: %w", i+1, err)
		}
		if len(data) > 0 {
			switch data[0] {
			case 0x00:
				return nil
			case 0x01:
				// still refreshing
			default:
				return fmt.Errorf("unexpected refresh status 0x%02X", data[0])
			}
		}
		time.Sleep(d.pollInterval)
	}
	return fmt.Errorf("refresh timeout")
}

func (d *Device) transmitExpect9000(apdu []byte) ([]byte, error) {
	data, sw1, sw2, err := d.transmit(apdu)
	if err != nil {
		return nil, err
	}
	if sw1 != 0x90 || sw2 != 0x00 {
		return nil, fmt.Errorf("status %02X%02X", sw1, sw2)
	}
	return data, nil
}

func (d *Device) transmit(apdu []byte) ([]byte, byte, byte, error) {
	resp, err := d.card.Transmit(apdu)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(resp) < 2 {
		return nil, 0, 0, fmt.Errorf("short response: %X", resp)
	}
	sw1 := resp[len(resp)-2]
	sw2 := resp[len(resp)-1]
	return resp[:len(resp)-2], sw1, sw2, nil
}

func (d *Device) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
