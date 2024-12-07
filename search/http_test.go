package search

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func initCompressionServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/gzip", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Encoding", "gzip")
		w := gzip.NewWriter(rw)
		w.Write([]byte("hello world gzip"))
		w.Close()
	})

	mux.HandleFunc("/br", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Encoding", "br")
		w := brotli.NewWriter(rw)
		w.Write([]byte("hello world br"))
		w.Close()
	})

	mux.HandleFunc("/deflate", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Encoding", "deflate")
		w := zlib.NewWriter(rw)
		w.Write([]byte("hello world deflate"))
		w.Close()
	})

	return httptest.NewServer(mux)
}

// This function exists to prevent copying and pasting over 30 lines of code.
func testResponseDecompression(t *testing.T, method string) {
	srv := initCompressionServer()
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/"+method, nil)
	if err != nil {
		panic(err)
	}

	cli := srv.Client()

	// Change the transport used by the client to not bother with
	// compression
	cliTp := cli.Transport.(*http.Transport)
	cliTp.DisableCompression = true

	// Perform the request
	res, err := cli.Do(req)
	if err != nil {
		panic(err)
	}
	handleResponseDecompression(res)
	defer res.Body.Close()

	// Check for the expected value
	buf := &strings.Builder{}
	if _, err := io.Copy(buf, res.Body); err != nil {
		panic(err)
	}

	exp := "hello world " + method
	if buf.String() != exp {
		t.Errorf("expected %q, got %q", exp, buf.String())
	}
}

func TestResponseDecompressionGzip(t *testing.T) {
	testResponseDecompression(t, "gzip")
}

func TestResponseDecompressionBrotli(t *testing.T) {
	testResponseDecompression(t, "br")
}

func TestResponseDecompressionDeflate(t *testing.T) {
	testResponseDecompression(t, "deflate")
}
