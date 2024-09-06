package engtest

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"git.sr.ht/~cmcevoy/srchd/internal/brotlihack"
)

// mockTransport implements [net/http.RoundTripper] and intercepts all requests
// to either return the cached copy or to request a new copy for the cache.
type mockTransport struct {
	// Base directory to read/write files.
	//
	// If the directory does not exist, it will be created for you.
	Base string

	// Update will actually perform the requests and save the responses to
	// disk instead of simply mocking the response.
	Update bool
}

// RoundTrip handles requests to a HTTP server by either responding to them by
// looking at the cache or by actually requesting data from the server and
// caching it.
//
// This way, tests can be done deterministically without hammering the actual
// search engine in question.
func (ms *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if ms.Update {
		return ms.updateHandle(req)
	}
	return ms.mockHandle(req)
}

// Default mocking request handler.
// This reads the cache and returns the response.
func (ms *mockTransport) mockHandle(req *http.Request) (*http.Response, error) {
	res := &http.Response{}

	r := getRequestInfo(req)
	h, err := os.Open(filepath.Join(ms.Base, hashuri(r.URL)))
	if err != nil {
		return nil, err
	}
	defer h.Close()

	rr := reqres{}
	if err := gob.NewDecoder(h).Decode(&rr); err != nil {
		return nil, err
	}
	resp := rr.Res

	res.StatusCode = resp.Status
	for _, v := range resp.Headers {
		res.Header.Add(v.Key, v.Value)
	}
	res.Body = io.NopCloser(bytes.NewReader(resp.Body))
	return res, nil
}

// Handles requests when Update mode is on.
func (ms *mockTransport) updateHandle(req *http.Request) (*http.Response, error) {
	r := getRequestInfo(req)

	// Perform the actual request
	client := &http.Client{}

	// Perform the request.
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Grab the response and save it to disk.
	resp := getResponseInfo(res)
	save(ms.Base, r, resp)

	// Success!
	return res, nil
}

// Extract pertient information from the request to the server.
func getRequestInfo(ctx *http.Request) request {
	req := request{}
	req.URL = string(ctx.URL.String())
	req.Method = string(ctx.Method)

	// Copy most headers.
	for k, vals := range ctx.Header {
		for _, v := range vals {
			req.Headers = append(req.Headers, header{Key: k, Value: v})
		}
	}

	// Save the body.
	if ctx.Body != nil {
		var err error
		req.Body, err = io.ReadAll(ctx.Body)
		if err != nil {
			panic(err)
		}
	}

	return req
}

// Extract pertient information from the response from the server.
func getResponseInfo(ctx *http.Response) response {
	res := response{}
	res.Status = ctx.StatusCode

	// Copy most headers.
	for k, vals := range ctx.Header {
		if k == "Content-Encoding" {
			// We automatically decompress it so this field does not matter
			continue
		}

		for _, v := range vals {
			res.Headers = append(res.Headers, header{Key: k, Value: v})
		}
	}

	// Don't bother with compression when saving the body.
	// It might be nice to have it compressed on disk, but I would rather
	// it be for the entire test file and not just for those who had a
	// compressed response sent to them.
	//
	// Also, we have to do the same Brotli shenanigans as we do in brave
	// here.
	// For more info as to why, please look at search/engines/brave.go.
	var body []byte
	var err error
	if slices.Contains(ctx.TransferEncoding, "br") {
		br := brotlihack.NewReader(bytes.NewReader(body))
		body, err = io.ReadAll(br)
		if err != nil && err.Error() != "brotli: excessive input" {
			panic(err)
		}
	} else {
		// All other content encodings should be automatically
		// decompressed by the default RoundTripper.

		body, err = io.ReadAll(ctx.Body)
		if err != nil {
			panic(err)
		}
	}

	if len(body) > 0 {
		res.Body = make([]byte, len(body))
		copy(res.Body, body)
	}

	return res
}
