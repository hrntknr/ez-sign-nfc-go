package ezsignnfc

import "testing"

func TestResolveReader(t *testing.T) {
	readers := []string{"Reader A", "Reader B"}

	t.Run("default-first-reader", func(t *testing.T) {
		got, err := resolveReader(readers, nil)
		if err != nil {
			t.Fatalf("resolveReader default: %v", err)
		}
		if got != "Reader A" {
			t.Fatalf("default reader: got %q want %q", got, "Reader A")
		}
	})

	t.Run("reader-index", func(t *testing.T) {
		got, err := resolveReader(readers, []ReaderSelector{ReaderIndex(1)})
		if err != nil {
			t.Fatalf("resolveReader index: %v", err)
		}
		if got != "Reader B" {
			t.Fatalf("reader index: got %q want %q", got, "Reader B")
		}
	})

	t.Run("reader-name", func(t *testing.T) {
		got, err := resolveReader(readers, []ReaderSelector{ReaderName("Reader B")})
		if err != nil {
			t.Fatalf("resolveReader name: %v", err)
		}
		if got != "Reader B" {
			t.Fatalf("reader name: got %q want %q", got, "Reader B")
		}
	})

	t.Run("bad-index", func(t *testing.T) {
		if _, err := resolveReader(readers, []ReaderSelector{ReaderIndex(2)}); err == nil {
			t.Fatal("expected error for out-of-range index")
		}
	})

	t.Run("bad-name", func(t *testing.T) {
		if _, err := resolveReader(readers, []ReaderSelector{ReaderName("Missing")}); err == nil {
			t.Fatal("expected error for missing reader name")
		}
	})

	t.Run("too-many-selectors", func(t *testing.T) {
		_, err := resolveReader(readers, []ReaderSelector{ReaderIndex(0), ReaderName("Reader A")})
		if err == nil {
			t.Fatal("expected error for multiple selectors")
		}
	})

	t.Run("nil-selector", func(t *testing.T) {
		_, err := resolveReader(readers, []ReaderSelector{nil})
		if err == nil {
			t.Fatal("expected error for nil selector")
		}
	})
}
