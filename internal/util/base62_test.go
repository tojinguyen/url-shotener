package util

import (
	"strings"
	"testing"
)

// keyLen mirrors model.ShortKeyLength; duplicated here to keep util dependency-free.
const keyLen = 7

func TestBase62Encode_Deterministic(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", []byte{}, "0"},
		{"zero byte", []byte{0x00}, "0"},
		{"one", []byte{0x01}, "1"},
		{"sixty-one", []byte{61}, "z"},
		{"sixty-two", []byte{62}, "10"},
		{"two fifty-five", []byte{0xFF}, "47"}, // 255 = 4*62 + 7
		{"256", []byte{0x01, 0x00}, "48"},      // 256 = 4*62 + 8
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Base62Encode(tt.in); got != tt.want {
				t.Fatalf("Base62Encode(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBase62Encode_StableAcrossCalls(t *testing.T) {
	in := MD5("https://example.com/a/very/long/path?with=query")
	first := Base62Encode(in)
	for i := 0; i < 100; i++ {
		if got := Base62Encode(in); got != first {
			t.Fatalf("encode not deterministic: call %d gave %q, want %q", i, got, first)
		}
	}
}

func TestBase62Encode_AlphabetOnly(t *testing.T) {
	in := MD5("alphabet-check")
	got := Base62Encode(in)
	for _, c := range got {
		if !strings.ContainsRune(base62Alphabet, c) {
			t.Fatalf("output contains non-base62 rune %q", c)
		}
	}
}

func TestBase62Key_FixedLength(t *testing.T) {
	// MD5 of an arbitrary string yields a large integer -> >7 base62 chars,
	// exercising the truncation branch.
	long := Base62Key(MD5("collision-seed"), keyLen)
	if len(long) != keyLen {
		t.Fatalf("len(long key) = %d, want %d", len(long), keyLen)
	}

	// A tiny input exercises the left-padding branch.
	padded := Base62Key([]byte{0x01}, keyLen)
	if len(padded) != keyLen {
		t.Fatalf("len(padded key) = %d, want %d", len(padded), keyLen)
	}
	if !strings.HasSuffix(padded, "1") || strings.Trim(padded, "0") != "1" {
		t.Fatalf("padded key = %q, want zero-padded %q", padded, "0000001")
	}
}

func TestBase62Key_Deterministic(t *testing.T) {
	seed := MD5("same-input")
	a := Base62Key(seed, keyLen)
	b := Base62Key(seed, keyLen)
	if a != b {
		t.Fatalf("Base62Key not deterministic: %q vs %q", a, b)
	}
}
