package wiby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"git.int21h.xyz/srchd/search"
)

// User agent to send requests with.
type wiby struct {
	name string
	http *search.HttpClient
}

type wibyResult struct {
	URL     string
	Title   string
	Snippet string
}

var (
	_ search.Engine = &wiby{}
)

func init() {
	search.Add("wiby", func(name string, config ...any) (search.Engine, error) {
		return &wiby{
			name: name,
			http: &search.HttpClient{},
		}, nil
	})
}

func (w *wiby) toNativeResult(r wibyResult) search.Result {
	return search.Result{
		Link:        r.URL,
		Title:       r.Title,
		Description: r.Snippet,
		Source:      w.name,
	}
}

func (w *wiby) Search(ctx context.Context, category search.Category, query string, page int) ([]search.Result, error) {
	if category != search.General {
		return nil, errors.ErrUnsupported
	}

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

	results := make([]search.Result, len(wres))
	for i := range results {
		results[i] = w.toNativeResult(wres[i])
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (w *wiby) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	res, err := w.http.Get(ctx, "https://wiby.me")
	if err != nil {
		res.Body.Close()
		return nil
	}
	return err
}
