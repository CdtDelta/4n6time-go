// Package textutil holds small text-decoding helpers shared by the parsers.
package textutil

import (
	"io"
)

// utf8BOM is the UTF-8 byte order mark that some Windows tools prepend to
// text exports.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// bomStrippingReader wraps an io.Reader and drops a leading UTF-8 BOM.
// Modeled on eztoolparser's reader, but it keeps the peeked bytes in a
// pending buffer so a caller reading with a buffer smaller than three bytes
// still receives every byte.
type bomStrippingReader struct {
	r       io.Reader
	checked bool
	pending []byte
}

// NewBOMStrippingReader returns a reader that yields r's content without a
// leading UTF-8 BOM. Content without a BOM passes through unchanged.
func NewBOMStrippingReader(r io.Reader) io.Reader {
	return &bomStrippingReader{r: r}
}

func (b *bomStrippingReader) Read(p []byte) (int, error) {
	if !b.checked {
		b.checked = true
		buf := make([]byte, len(utf8BOM))
		n, err := io.ReadFull(b.r, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return 0, err
		}
		buf = buf[:n]
		if n == len(utf8BOM) && buf[0] == utf8BOM[0] && buf[1] == utf8BOM[1] && buf[2] == utf8BOM[2] {
			buf = buf[:0]
		}
		b.pending = buf
	}

	if len(b.pending) > 0 {
		n := copy(p, b.pending)
		b.pending = b.pending[n:]
		return n, nil
	}
	return b.r.Read(p)
}

// TrimBOM returns s without a leading UTF-8 BOM.
func TrimBOM(s string) string {
	if len(s) >= len(utf8BOM) && s[:len(utf8BOM)] == string(utf8BOM) {
		return s[len(utf8BOM):]
	}
	return s
}
