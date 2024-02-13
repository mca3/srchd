package search

import (
	"context"
	"fmt"
	"io"
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
func (h *HttpClient) New(ctx context.Context, method, url string, body io.Reader) (*fasthttp.Request, error) {
	h.ensureReady()

	req := fasthttp.AcquireRequest()

	req.SetRequestURI(url)

	// Set user agent.
	req.Header.Add("User-Agent", h.ua())

	// Add some headers too to make us seem more real.
	// *The order is important.*
	// TODO: This probably isn't enough, or isn't convincing.
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("Sec-Fetch-Site", "none")
	req.Header.Add("Sec-Fetch-Mode", "navigate")
	req.Header.Add("Sec-Fetch-User", "?1")
	req.Header.Add("Sec-Fetch-Dest", "document")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Accept-Language", "en-US,en;q=0.9")

	if body != nil {
		req.SetBodyStream(body, -1)
	}

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
func (h *HttpClient) Post(ctx context.Context, url string, contentType string, body io.Reader) (*fasthttp.Response, error) {
	h.ensureReady()

	// Create a request.
	req, err := h.New(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.SetContentType(contentType)

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
