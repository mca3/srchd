package search

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
)

// User agent to send requests with.
type wiby struct {
	name string
	http *HttpClient
}

type wibyResult struct {
	URL     string
	Title   string
	Snippet string
}

var (
	_ GeneralSearcher = &wiby{}
)

func init() {
	Add("wiby", func(name string, config ...map[string]any) (Engine, error) {
		cfg := getConfig(config)

		return &wiby{
			name: name,
			http: newHttpClient(cfg),
		}, nil
	})
}

func (w *wiby) toNativeResult(r wibyResult) Result {
	// wiby escapes all text for direct inclusion in HTML, presumably.
	// Go's text/template does this for us, so text should be unescaped
	// here to prevent "&amp;" and other similar escapes from appearing as
	// text in the results page.
	return Result{
		Link:        html.UnescapeString(r.URL),
		Title:       html.UnescapeString(r.Title),
		Description: html.UnescapeString(r.Snippet),
		Sources:     []string{w.name},
	}
}

func (w *wiby) GeneralSearch(ctx context.Context, query string, page int) ([]Result, error) {
	// Wiby has a native API we can use.
	// There's probably some encoding/json tomfoolery I could employ so we
	// don't need an intermediate step, but whatever.

	form := url.Values{}

	form.Set("q", query)
	if page > 1 {
		form.Set("p", fmt.Sprint(page+1))
	}

	ctx, cancel := w.http.Context(ctx)
	defer cancel()

	res, err := w.http.Get(ctx, "https://wiby.me/json?"+form.Encode())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var wres []wibyResult
	if err := json.NewDecoder(res.Body).Decode(&wres); err != nil {
		return nil, err
	}

	results := make([]Result, len(wres))
	for i := range results {
		results[i] = w.toNativeResult(wres[i])
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (w *wiby) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	res, err := w.http.Get(ctx, "https://wiby.me/")
	if err != nil {
		res.Body.Close()
		return nil
	}
	return err
}
