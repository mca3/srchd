package search

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// HttpClient is a helpful wrapper around [net/fasthttp.Client] that does useful
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

	http *fasthttp.Client
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
			h.http = &fasthttp.Client{
				NoDefaultUserAgentHeader: true,
				DialDualStack:            true,
				ReadTimeout:              h.Timeout,
				WriteTimeout:             h.Timeout,
			}
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

// New creates a new HTTP request.
func (h *HttpClient) New(ctx context.Context, method, url string, body []byte, contentType ...string) (*fasthttp.Request, error) {
	h.ensureReady()

	req := fasthttp.AcquireRequest()

	// Enable hard mode.
	// This is done because otherwise fasthttp orders things differently
	// than a browser would, and we *want* to look as close to a browser as
	// possible.
	req.Header.DisableSpecialHeader()
	req.Header.DisableNormalizing()

	// Since we have no special header handling, we have to set all of this ourselves.
	uri := fasthttp.AcquireURI()
	uri.Parse(nil, []byte(url))

	req.SetURI(uri)

	req.Header.SetMethod(method)
	req.Header.SetBytesV("Host", uri.Host())

	req.Header.Set("sec-ch-ua", `"Chromium";v="121", "Not)A;Brand";v="24", "Google Chrome";v="121"`)
	req.Header.Set("sec-ch-ua-mobile", `?0`)
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("Upgrade-Insecure-Requests", `1`)
	req.Header.Set("User-Agent", h.ua())

	// Add some headers too to make us seem more real.
	// *The order is important.*
	// TODO: This probably isn't enough, or isn't convincing.
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")

	if body != nil {
		req.SetBody(body)

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

func (h *HttpClient) Do(req *fasthttp.Request) (*fasthttp.Response, error) {
	res := fasthttp.AcquireResponse()
	return res, h.http.DoRedirects(req, res, 5)
}

// Get performs a GET request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Get(ctx context.Context, url string) (*fasthttp.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform the request.
	res, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Error out on non-200 status codes.
	if res.StatusCode() != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		return nil, HttpError{Status: res.StatusCode(), URL: url, Method: "GET"}
	}

	// All good.
	return res, nil
}

// Post performs a POST request on a given URL.
//
// If the server responds with a non-200 status code, then the returned
// response will be nil and err will be of type [HttpError].
func (h *HttpClient) Post(ctx context.Context, url string, contentType string, body []byte) (*fasthttp.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, "POST", url, body, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Perform the request.
	res, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Error out on non-200 status codes.
	if res.StatusCode() != 200 {
		// The request itself succeeded but we aren't interested in
		// anything we got due to the failure status.
		if h.Debug {
			stream, _ := res.BodyUncompressed()
			log.Printf("post %s failed with status code %d: %s", url, res.StatusCode(), string(stream))
		}

		return nil, HttpError{Status: res.StatusCode(), URL: url, Method: "POST"}
	}

	// All good.
	return res, nil
}
