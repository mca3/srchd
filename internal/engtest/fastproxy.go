package engtest

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"

	"github.com/andybalholm/brotli"
	"github.com/valyala/fasthttp"
)

// mockFasthttpTransport implements [github.com/valyala/fasthttp.RoundTripper] and
// intercepts all requests to either return the cached copy or to request a new
// copy for the cache.
type mockFasthttpTransport struct {
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
func (ms *mockFasthttpTransport) RoundTrip(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (retry bool, err error) {
	if ms.Update {
		return false, ms.updateHandle(req, resp)
	}
	return false, ms.mockHandle(req, resp)
}

// Default mocking request handler.
// This reads the cache and returns the response.
func (ms *mockFasthttpTransport) mockHandle(req *fasthttp.Request, res *fasthttp.Response) error {
	r := getFasthttpRequestInfo(req)
	h, err := os.Open(filepath.Join(ms.Base, hashuri(r.URL)))
	if err != nil {
		return err
	}
	defer h.Close()

	rr := TestData{}
	if err := gob.NewDecoder(h).Decode(&rr); err != nil {
		return err
	}
	resp := rr.Res

	res.SetStatusCode(resp.Status)
	for _, v := range resp.Headers {
		res.Header.Add(v.Key, v.Value)
	}
	res.SetBody(resp.Body)
	return nil
}

// Handles requests when Update mode is on.
func (ms *mockFasthttpTransport) updateHandle(req *fasthttp.Request, res *fasthttp.Response) error {
	r := getFasthttpRequestInfo(req)

	// Perform the actual request
	client := &fasthttp.Client{
		DialDualStack: true,

		// This is required because of Brave.
		ReadBufferSize: 8192,

		// Don't add anything else to my request.
		NoDefaultUserAgentHeader: true,
	}

	// Perform the request.
	if err := client.DoRedirects(req, res, 5); err != nil {
		return err
	}

	// Grab the response and save it to disk.
	resp := getFasthttpResponseInfo(res)
	save(ms.Base, r, resp)

	// Success!
	return nil
}

// Extract pertient information from the request to the server.
func getFasthttpRequestInfo(ctx *fasthttp.Request) request {
	req := request{}
	req.URL = string(ctx.RequestURI())
	req.Method = string(ctx.Header.Method())

	// The order is important when sending stuff to the server because it
	// could trip anti-bot protection.
	// While yes, we are robots in this case, the entire point of srchd is
	// to search engines on behalf of the user so we must convince the
	// engines we talk to that we are your average human.
	ctx.Header.VisitAllInOrder(func(k, v []byte) {
		// We need to make a copy of k and v because fasthttp says so.
		// Converting to a string does just this.
		kk := string(k)
		vv := string(v)
		req.Headers = append(req.Headers, header{Key: kk, Value: vv})
	})

	// Save the body.
	body := ctx.Body()
	if len(body) > 0 {
		req.Body = make([]byte, len(body))
		copy(req.Body, body)
	}

	return req
}

// Extract pertient information from the response from the server.
func getFasthttpResponseInfo(ctx *fasthttp.Response) response {
	res := response{}
	res.Status = ctx.Header.StatusCode()

	// The order is not important this time, because we don't care and
	// we're the one that's taking in the data.
	ctx.Header.VisitAll(func(k, v []byte) {
		// We need to make a copy of k and v because fasthttp says so.
		// Converting to a string does just this.
		kk := string(k)
		if kk == "Content-Encoding" {
			// We automatically decompress it so this field does not matter
			return
		}
		vv := string(v)
		res.Headers = append(res.Headers, header{Key: kk, Value: vv})
	})

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
	if string(ctx.Header.ContentEncoding()) == "br" {
		br := brotli.NewReader(bytes.NewReader(ctx.Body()))
		body, err = io.ReadAll(br)
		if err != nil && err.Error() != "brotli: excessive input" {
			panic(err)
		}
	} else {
		body, err = ctx.BodyUncompressed()
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
