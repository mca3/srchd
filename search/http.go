package search

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

// HttpClient is a helpful wrapper around [net/http.Client] that does useful
// things to HTTP requests and responses you would've had to write anyway.
//
// The zero value is ready to use.
type HttpClient struct {
	// Timeout is the maximum amount of time to wait for the request to
	// complete.
	Timeout time.Duration

	// UserAgent holds the value of the User-Agent header of HTTP requests.
	//
	// If UserAgent is empty, then [DefaultUserAgent] is used.
	UserAgent string

	// Debug logs all HTTP requests sent through this HttpClient if it is
	// true before the first request is made.
	Debug bool

	http *http.Client
	once sync.Once
}

// HttpError represents a generic HTTP error.
type HttpError struct {
	// Status code of response.
	Status int

	// URL of request.
	URL string

	// Method of request.
	Method string
}

// decompReader is a simple wrapper around two related readers.
//
// TODO: This may be completely unnecessary.
type decompReader struct {
	c io.Closer
	r io.Reader
}

// debugRoundTripper logs all requests sent through the HTTP client to the console.
type debugRoundTripper struct {
	proxy http.RoundTripper
}

var (
	_ io.ReadCloser = &decompReader{}
)

func (h HttpError) Error() string {
	return fmt.Sprintf("%s %q failed with status code %d", h.Method, h.URL, h.Status)
}

// Reads data from the decompression stream.
func (d *decompReader) Read(data []byte) (int, error) {
	return d.r.Read(data)
}

// Closes the underlying stream.
func (d *decompReader) Close() error {
	if cr, ok := d.r.(io.ReadCloser); ok {
		cr.Close()
	}
	return d.c.Close()
}

// May or may not create a new reader that decompresses content on the fly.
func newReader(r io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	switch contentEncoding {
	case "gzip":
		dr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return &decompReader{r, dr}, nil
	case "deflate":
		dr := flate.NewReader(r)
		return &decompReader{r, dr}, nil
	case "br":
		dr := brotli.NewReader(r)
		return &decompReader{r, dr}, nil
	}

	return r, nil
}

// RoundTrip logs the request to the console and calls the actual RoundTrip function.
func (drt *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	buf := &strings.Builder{}
	fmt.Fprintf(buf, "%s %s\n", req.Method, req.URL)

	req.Header.Write(buf)
	buf.WriteString("\n\n")

	resp, err := drt.proxy.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	resp.Header.Write(buf)
	log.Println(buf.String())

	return resp, nil
}

// Ensures that the HttpClient is ready to perform requests.
func (h *HttpClient) ensureReady() {
	h.once.Do(func() {
		// Create a new HTTP client.
		if h.http == nil {
			h.http = &http.Client{}
		}

		// The debug flag requires us to use a different Transport than
		// the default one.
		if h.Debug {
			h.http.Transport = &debugRoundTripper{http.DefaultTransport}
		}
	})
}

// Creates a new context from a parent context.
func (h *HttpClient) Context(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, h.Timeout)
}

// Get a non-empty user agent.
func (h *HttpClient) ua() string {
	return h.UserAgent
}

// New creates a new HTTP request.
func (h *HttpClient) New(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	h.ensureReady()

	req, err := http.NewRequestWithContext(ctx, "GET", url, body)
	if err != nil {
		return nil, err
	}

	// Set user agent.
	req.Header.Set("User-Agent", h.ua())

	// Set some headers too to make us seem more real.
	// TODO: This probably isn't enough, or isn't convincing.
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	return req, nil
}

// Get performs a GET request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Get(ctx context.Context, url string) (*http.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform the request.
	res, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	newBody, err := newReader(res.Body, res.Header.Get("Content-Encoding"))
	if err != nil {
		res.Body.Close()
		return nil, fmt.Errorf("failed to decompress body: %w", err)
	}
	res.Body = newBody

	// Error out on non-200 status codes.
	if res.StatusCode != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		res.Body.Close()

		return nil, HttpError{Status: res.StatusCode, URL: url, Method: "GET"}
	}

	// All good.
	return res, nil
}

// Post performs a POST request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	// Try to set Content-Length.
	// If we can't do anything here, then we will send the request without it.
	switch v := body.(type) {
	case interface{ Len() int }:
		// Interfaces such as strings.Reader and bytes.Buffer provide
		// this function.
		req.Header.Set("Content-Length", fmt.Sprint(v.Len()))
	case io.ReadSeeker:
		// Naive solution, but worth a shot.

		offs, err := v.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, fmt.Errorf("seek to end failed: %w", err)
		}
		req.Header.Set("Content-Length", fmt.Sprint(offs))

		_, err = v.Seek(0, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("seek to start failed: %w", err)
		}
	}

	// Perform the request.
	res, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	newBody, err := newReader(res.Body, res.Header.Get("Content-Encoding"))
	if err != nil {
		res.Body.Close()
		return nil, fmt.Errorf("failed to decompress body: %w", err)
	}
	res.Body = newBody

	// Error out on non-200 status codes.
	if res.StatusCode != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		if h.Debug {
			buf := &strings.Builder{}
			io.Copy(buf, res.Body)
			log.Printf("post %s failed with status code %d: %s", url, res.StatusCode, buf.String())
		}
		res.Body.Close()

		return nil, HttpError{Status: res.StatusCode, URL: url, Method: "POST"}
	}

	// All good.
	return res, nil
}
