package search

import (
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
	Add("bing", func(name string, config ...any) (Engine, error) {
		return &bing{
			name: name,
			http: &HttpClient{},
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
		"https://bing.com/search?"+form.Encode(),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse html: %w", err)
	}

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
		v.Title = title.Text()
		v.Description = e.Find("div > p").Text()
		v.Source = b.name

		results[i] = v
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (b *bing) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	res, err := b.http.Get(ctx, "https://bing.com")
	if err != nil {
		res.Body.Close()
		return nil
	}
	return err
}
