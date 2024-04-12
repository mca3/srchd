package search

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

// User agent to send requests with.
type bing struct {
	name string
	http *HttpClient
}

var (
	_ GeneralSearcher = &bing{}
)

func init() {
	Add("bing", false, func(name string, config ...map[string]any) (Engine, error) {
		cfg := getConfig(config)

		return &bing{
			name: name,
			http: newHttpClient(cfg),
		}, nil
	})
}

// GeneralSearch attempts to query the engine and returns a number of results.
func (b *bing) GeneralSearch(ctx context.Context, query string, page int) ([]Result, error) {
	form := url.Values{}

	form.Set("q", query)

	if page >= 1 {
		form.Set("first", fmt.Sprint(10*page))
	}

	ctx, cancel := b.http.Context(ctx)
	defer cancel()

	res, err := b.http.Get(
		ctx,
		"https://www.bing.com/search?"+form.Encode(),
	)
	if err != nil {
		return nil, err
	}

	body, err := res.BodyUncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("unable to parse html: %w", err)
	}

	// Remove all of these. Prepends "Web" to every result.
	doc.Find(`.algoSlug_icon`).Remove()

	elem := doc.Find(`.b_algo`)

	// Bing results are laid out like this:
	// h2 > a: title and link
	// div.b_caption > p: desc

	results := make([]Result, elem.Length())

	for i := range results {
		v := Result{}

		e := elem.Eq(i)
		title := e.Find("h2 > a")
		v.Link, _ = title.Attr("href")
		v.Link = CleanURL(v.Link)
		v.Title = title.Text()
		v.Description = e.Find("div > p").Text()
		v.Sources = []string{b.name}

		results[i] = v
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (b *bing) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := b.http.Get(ctx, "https://www.bing.com/")
	return err
}
