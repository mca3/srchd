package engines

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"git.sr.ht/~cmcevoy/srchd/search"
)

// User agent to send requests with.
type yahoo struct {
	name string
	http *search.HttpClient
}

var (
	_ search.GeneralSearcher = &yahoo{}
)

func init() {
	search.Add("yahoo", true, func(name string, config ...map[string]any) (search.Engine, error) {
		cfg := search.GetConfig(config)

		return &yahoo{
			name: name,
			http: search.NewHttpClient(cfg),
		}, nil
	})
}

func decodeYahooHref(href string) string {
	// In the form of:
	// https://r.search.yahoo.com/.../.../RU=yourlinkishere/.../...

	idx := strings.Index(href, "/RU=")
	if idx == -1 {
		return href
	}

	newHref := href[idx+4:]

	idx = strings.Index(newHref, "/")
	if idx == -1 {
		return href
	}

	newHref = newHref[:idx]
	newHref, err := url.PathUnescape(newHref)
	if err != nil {
		return href
	}
	return newHref
}

// GeneralSearch attempts to query the engine and returns a number of results.
func (b *yahoo) GeneralSearch(ctx context.Context, query string, page int) ([]search.Result, error) {
	form := url.Values{}

	form.Set("p", query)
	form.Set("nojs", "1")

	if page >= 1 {
		form.Set("b", fmt.Sprint(1+7*page))
		form.Set("pz", "7")
	}

	ctx, cancel := b.http.Context(ctx)
	defer cancel()

	res, err := b.http.Get(
		ctx,
		"https://search.yahoo.com/search?"+form.Encode(),
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

	elem := doc.Find(`.algo`)

	results := make([]search.Result, elem.Length())

	for i := range results {
		v := search.Result{}

		e := elem.Eq(i)
		title := e.Find("h3.title > a")
		title.Find(`span`).Remove()
		v.Link, _ = title.Attr("href")
		v.Link = search.CleanURL(decodeYahooHref(v.Link))
		v.Title = title.Text()
		v.Description = strings.TrimSpace(e.Find(".compText > p").Text())
		v.Sources = []string{b.name}

		results[i] = v
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (b *yahoo) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := b.http.Get(ctx, "https://search.yahoo.com/")
	return err
}
