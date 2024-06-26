package engtest

import (
	"crypto/md5"
	"encoding/base32"
	"encoding/gob"
	"os"
	"path/filepath"

	"github.com/valyala/fasthttp"
)

// mockTransport implements [github.com/valyala/fasthttp.RoundTripper] and
// intercepts all requests to either return the cached copy or to request a new
// copy for the cache.
type mockTransport struct {
	// Base directory to read/write files.
	//
	// If the directory does not exist, it will be created for you.
	Base string

	// Update will actually perform the requests and save the responses to
	// disk instead of simply mocking the response.
	Update bool
}

// HTTP header k/v pair.
type header struct {
	Key, Value string
}

// request holds all information from a request.
type request struct {
	URL     string
	Method  string
	Headers []header
	Body    []byte
}

// response holds all information returned from the server.
type response struct {
	Status  int
	Headers []header
	Body    []byte
}

// Request/response pair for testdata.
type reqres struct {
	Req request
	Res response
}

// RoundTrip handles requests to a HTTP server by either responding to them by
// looking at the cache or by actually requesting data from the server and
// caching it.
//
// This way, tests can be done deterministically without hammering the actual
// search engine in question.
func (ms *mockTransport) RoundTrip(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (retry bool, err error) {
	if ms.Update {
		return false, ms.updateHandle(req, resp)
	}
	return false, ms.mockHandle(req, resp)
}

// Default mocking request handler.
// This reads the cache and returns the response.
func (ms *mockTransport) mockHandle(req *fasthttp.Request, res *fasthttp.Response) error {
	r := getRequestInfo(req)
	h, err := os.Open(filepath.Join(ms.Base, hashuri(r.URL)))
	if err != nil {
		return err
	}
	defer h.Close()

	rr := reqres{}
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
func (ms *mockTransport) updateHandle(req *fasthttp.Request, res *fasthttp.Response) error {
	r := getRequestInfo(req)

	// Perform the actual request
	client := &fasthttp.Client{
		DialDualStack: true,

		// Don't add anything else to my request.
		NoDefaultUserAgentHeader: true,
	}

	// Perform the request.
	if err := client.DoRedirects(req, res, 5); err != nil {
		return err
	}

	// Grab the response and save it to disk.
	resp := getResponseInfo(res)
	ms.save(r, resp)

	// Success!
	return nil
}

// Saves a request/response pair to disk.
func (ms *mockTransport) save(req request, res response) {
	rr := reqres{req, res}

	// Create the contianing directory if we need to.
	if err := os.MkdirAll(ms.Base, 0755); err != nil {
		panic(err)
	}

	// Save stuff to disk.
	h, err := os.Create(filepath.Join(ms.Base, hashuri(req.URL)))
	if err != nil {
		panic(err)
	}
	defer h.Close()

	// Gob is convenient because it works quite well for our use cases.
	if err := gob.NewEncoder(h).Encode(rr); err != nil {
		panic(err)
	}
}

// Extract pertient information from the request to the server.
func getRequestInfo(ctx *fasthttp.Request) request {
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
func getResponseInfo(ctx *fasthttp.Response) response {
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
	body, err := ctx.BodyUncompressed()
	if err != nil {
		panic(err)
	}
	if len(body) > 0 {
		res.Body = make([]byte, len(body))
		copy(res.Body, body)
	}

	return res
}

// It is often problematic to save files with "/" in the name.
// Send it through MD5 and base32 the result; it's overkill, but it works.
func hashuri(uri string) string {
	md := md5.New()
	s := md.Sum([]byte(uri))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(s[:])
}
