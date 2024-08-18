package search

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/quic-go/quic-go/http3"

	"git.sr.ht/~cmcevoy/srchd/internal/brotlihack"
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

	// Send requests using this HTTP proxy.
	//
	// This does not default to the HTTP_PROXY environment variable and
	// must be explicitly set to use a proxy for all HTTP requests.
	HttpProxy string

	// Enable HTTP/3 using quic-go.
	QUIC bool

	// Enable zero roundtrip time for a performance boost on subsequent
	// connections.
	// Requires QUIC to be true.
	//
	// Using 0-RTT can have implications on the security of your connections as it
	// becomes possible to replay the data you send to the server.
	// Generally it is only safe to use it if the requests you are doing are
	// idempotent.
	// For srchd, this is always the case as of writing.
	//
	// For more information, refer to section 8 of RFC 8446:
	// https://datatracker.ietf.org/doc/html/rfc8446#section-8
	QUIC_0RTT bool

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

func (h HttpError) Error() string {
	return fmt.Sprintf("%s %q failed with status code %d", h.Method, h.URL, h.Status)
}

// Ensures that the HttpClient is ready to perform requests.
func (h *HttpClient) ensureReady() {
	h.once.Do(func() {
		// Create a new HTTP client.
		if h.http == nil {
			h.http = &http.Client{
				Timeout: h.Timeout,
			}

			if h.QUIC {
				// Use HTTP/3.
				rt := &http3.RoundTripper{}
				if h.QUIC_0RTT {
					// Requires a little bit of extra
					// configuration for TLS.
					rt.TLSClientConfig = &tls.Config{
						ClientSessionCache: tls.NewLRUClientSessionCache(100),
					}
				}

				h.http.Transport = rt
			}

			// TODO: Proxy stuff
		}

		// The debug flag requires us to use a different Transport than
		// the default one.
		/*
			if h.Debug {
				h.http.Transport = &debugRoundTripper{http.DefaultTransport}
			}
		*/
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

// Client fetches the [github.com/valyala/http.Client] for this specific
// HTTP client.
//
// Do not change fields of the returned Client struct once you have performed a
// request.
func (h *HttpClient) Client() *http.Client {
	// This is a function because the HttpClient is lazily initialized.
	h.ensureReady()
	return h.http
}

// If using QUIC and 0-RTT, then GET/HEAD require special methods.
func (h *HttpClient) quicMethod(method string) string {
	if !h.QUIC || !h.QUIC_0RTT {
		// QUIC and 0RTT aren't both enabled at the same time so just
		// fallback to the usual methods.
		return method
	}

	switch method {
	case http.MethodGet:
		return http3.MethodGet0RTT
	case http.MethodHead:
		return http3.MethodHead0RTT
	default:
		return method
	}
}

// New creates a new HTTP request.
func (h *HttpClient) New(ctx context.Context, method, url string, body []byte, contentType ...string) (*http.Request, error) {
	h.ensureReady()

	// We don't want to create a bytes.Reader on a nil body.
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	// Initialize the request. This does a lot of the work for us.
	req, err := http.NewRequestWithContext(ctx, h.quicMethod(method), url, bodyReader)
	if err != nil {
		return nil, err
	}

	// NOTE: Unlike Fasthttp, I don't think the ordering of headers matters
	// all that much here, which is unfortunate because that was
	// essentially the only reason that I moved to it in the first place.
	// If I ever feel like reinventing the wheel, I could introduce some
	// mandatory ordering by reimplementing RoundTripper I think, but
	// that's a project for another day.

	// Add some headers too to make us seem more real.
	// TODO: This probably isn't enough, or isn't convincing.
	req.Header.Set("sec-ch-ua", `"Chromium";v="121", "Not)A;Brand";v="24", "Google Chrome";v="121"`)
	req.Header.Set("sec-ch-ua-mobile", `?0`)
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("Upgrade-Insecure-Requests", `1`)
	req.Header.Set("User-Agent", h.ua())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")

	if body != nil {
		req.Header.Set("Content-Type", contentType[0])
		req.Header.Set("Content-Length", fmt.Sprint(len(body)))
	}

	req.Header.Set("Sec-Fetch-Site", `none`)
	req.Header.Set("Sec-Fetch-Mode", `navigate`)
	req.Header.Set("Sec-Fetch-User", `?1`)
	req.Header.Set("Sec-Fetch-Dest", `document`)
	req.Header.Set("Accept-Encoding", `gzip, deflate, br`)
	req.Header.Set("Accept-Language", `en-US,en;q=0.9`)

	return req, nil
}

func (h *HttpClient) Do(req *http.Request) (*http.Response, error) {
	return h.http.Do(req)
}

// Get performs a GET request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Get(ctx context.Context, url string) (*http.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform the request.
	res, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Error out on non-200 status codes.
	if res.StatusCode != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		return nil, HttpError{Status: res.StatusCode, URL: url, Method: "GET"}
	}

	// All good.
	return res, nil
}

// Post performs a POST request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Post(ctx context.Context, url string, contentType string, body []byte) (*http.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, http.MethodPost, url, body, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform the request.
	res, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Error out on non-200 status codes.
	if res.StatusCode != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		if h.Debug {
			log.Printf("post %s failed with status code %d", url, res.StatusCode)
		}

		return nil, HttpError{Status: res.StatusCode, URL: url, Method: "POST"}
	}

	// All good.
	return res, nil
}

// Shared code between HtmlGet and HtmlPost.
func documentFromHttpResponse(res *http.Response) (*goquery.Document, error) {
	var err error

	// Decode.
	// This could probably be done better but it gets the job done.
	var body io.Reader
	contentEncoding := res.Header.Values("Content-Encoding")
	if !res.Uncompressed && len(contentEncoding) > 0 {
		// net/http's default transport will decompress when it can,
		// but it needs some extra help for Brotli and gzip.

		switch contentEncoding[0] {
		case "br":
			body = brotlihack.NewReader(res.Body)
		case "gzip":
			body, err = gzip.NewReader(res.Body)
			if err != nil {
				return nil, err
			}
		default:
			// Unlikely since we support everything we request, but
			// it's there if we need it.
			return nil, fmt.Errorf("unknown content encoding: %v", contentEncoding)
		}
	} else {
		// net/http has probably decompressed it on its own or the
		// server didn't compress its response at all.
		body = res.Body
	}

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %w", err)
	}
	return doc, nil
}

// Helper function to fetch HTML using a GET request and automatically parse
// it.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [FasthttpError].
func (h *HttpClient) HtmlGet(ctx context.Context, url string) (*goquery.Document, error) {
	// Fire off a request.
	res, err := h.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return documentFromHttpResponse(res)
}

// Helper function to fetch HTML using a GET request and automatically parse
// it.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [FasthttpError].
func (h *HttpClient) HtmlPost(ctx context.Context, url string, contentType string, body []byte) (*goquery.Document, error) {
	// Fire off a request.
	res, err := h.Post(ctx, url, contentType, body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return documentFromHttpResponse(res)
}
