// Package brotlihack implements a wrapper to workaround a non-fatal error
// returned by the Brotli package.
//
// We need to handle Brotli a little specially because the decompresser we're
// using doesn't like Brave's response for some reason.
// While this is specific to *just* Brave right now, we might as well make it
// available for all engines in the off chance they need it too.
//
// The error from the decompressor isn't fatal but it is treated as such:
// https://github.com/andybalholm/brotli/blob/57434b509141a6ee9681116b8d552069126e615f/reader.go#L74-L76
// https://github.com/valyala/fasthttp/blob/b06f4e21d918faa84ae0aa12c9e4dc7285b9767e/http.go#L505-L512
//
// So my crappy solution is to rewrite the part where it decompresses Brotli
// and explicitly ignore that "brotli: excessive input" error.
package brotlihack

import (
	"io"

	"github.com/andybalholm/brotli"
)

type brotliHackReader struct {
	r *brotli.Reader
}

var (
	// Make sure we implement io.Reader.
	_ io.Reader = &brotliHackReader{}
)

// NewReader creates a new wrapped Brotli decoder.
func NewReader(r io.Reader) *brotliHackReader {
	return &brotliHackReader{
		r: brotli.NewReader(r),
	}
}

// Reads data from the Brotli reader.
func (b *brotliHackReader) Read(data []byte) (n int, err error) {
	n, err = b.r.Read(data)
	if err != nil && err.Error() == "brotli: excessive input" {
		//       ^ I told you this sucked!
		// In this case we should just set err to io.EOF.
		err = io.EOF
	}

	return
}
