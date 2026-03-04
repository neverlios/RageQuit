package display

import "testing"

func TestSourceHash_returns64CharHex(t *testing.T) {
	h := sourceHash()
	if len(h) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars: %q", len(h), h)
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in hash", c)
		}
	}
}

func TestSourceHash_matchesKnownValue(t *testing.T) {
	// If this test fails, swiftSource was changed.
	// Update this constant after an intentional source change.
	const want = "41c8e86164a393a31d2097bb34e0eae4ffb410bd5f9b17934f5ab6ac37bc7d69"
	got := sourceHash()
	if got != want {
		t.Errorf("sourceHash changed: got %q\n\nIf swiftSource was intentionally changed, update the want constant to %q", got, got)
	}
}
