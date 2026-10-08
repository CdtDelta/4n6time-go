package textutil

import (
	"io"
	"strings"
	"testing"
)

func TestBOMStrippingReader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with BOM", "\xEF\xBB\xBFhello", "hello"},
		{"without BOM", "hello", "hello"},
		{"BOM only", "\xEF\xBB\xBF", ""},
		{"empty", "", ""},
		{"short input", "hi", "hi"},
		{"partial BOM prefix kept", "\xEF\xBBx", "\xEF\xBBx"},
		{"BOM not at start kept", "a\xEF\xBB\xBFb", "a\xEF\xBB\xBFb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := io.ReadAll(NewBOMStrippingReader(strings.NewReader(tt.in)))
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBOMStrippingReaderTinyBuffer reads one byte at a time to confirm no
// bytes are lost when the caller's buffer is smaller than the BOM peek.
func TestBOMStrippingReaderTinyBuffer(t *testing.T) {
	for _, in := range []string{"\xEF\xBB\xBFabc", "abcdef"} {
		r := NewBOMStrippingReader(strings.NewReader(in))
		var out []byte
		buf := make([]byte, 1)
		for {
			n, err := r.Read(buf)
			out = append(out, buf[:n]...)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
		}
		want := TrimBOM(in)
		if string(out) != want {
			t.Errorf("input %q: got %q, want %q", in, out, want)
		}
	}
}

func TestTrimBOM(t *testing.T) {
	if got := TrimBOM("\xEF\xBB\xBF{\"a\":1}"); got != "{\"a\":1}" {
		t.Errorf("TrimBOM with BOM = %q", got)
	}
	if got := TrimBOM("{\"a\":1}"); got != "{\"a\":1}" {
		t.Errorf("TrimBOM without BOM = %q", got)
	}
	if got := TrimBOM(""); got != "" {
		t.Errorf("TrimBOM empty = %q", got)
	}
}
