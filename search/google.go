package search

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type google struct {
	name string
	http *HttpClient
}

func init() {
	Add("google", func(name string, config ...any) (Engine, error) {
		return &google{
			name: name,
			http: &HttpClient{},
		}, nil
	})
}

// Parses a general query results page.
func (g *google) parseGeneral(doc *goquery.Document) ([]Result, error) {
	// Because we don't use the non-JS variant, we are given an extra class
	// on search results which is *very* helpful!
	// And it doesn't change occasionally.
	// Each search result we care about has these properties:
	// - has the class "g"
	// - has a jscontroller attribute
	elem := doc.Find(`.g[jscontroller]`)

	// Perfect.
	// So these divs have some child divs, with these contents:
	// 1. header, title, link
	// 2. description
	// 3. ??? but it doesn't matter
	// In child div 1, we can grab the title from the one and only h3 tag,
	// and also the href.
	// In the second div, the inner text is the description.
	// And that's all we need!
	results := make([]Result, int(elem.Length()))

	for i := range results {
		v := Result{}

		e := elem.Eq(i).Children().First().Children()
		title := e.Eq(0).Find("h3")
		link := e.Eq(0).Find("a[href]")
		desc := e.Eq(1).Find("div")

		v.Title = title.Text()
		v.Link, _ = link.Attr("href")
		v.Description = strings.TrimSpace(desc.Text())
		v.Source = g.name

		results[i] = v
	}

	return results, nil
}

// Parses news query results.
func (g *google) parseNews(doc *goquery.Document) ([]Result, error) {
	// News stuff is done a bit differently than normal but isn't too hard to parse.
	elem := doc.Find("#rso > div > div").Children()

	// And there's all of our news results.
	results := make([]Result, int(elem.Length()))

	for i := range results {
		v := Result{}

		// Everything is wrapped in the a element.
		a := elem.Eq(i).Find("div > div > a")
		v.Link, _ = a.Attr("href")

		e := a.Children()

		// Title is followed by the description.
		// The role is a pretty good way to find the title.
		title := e.Find(`div[role="heading"]`)
		desc := title.Next()

		v.Title = title.Text()
		v.Description = strings.TrimSpace(desc.Text())
		v.Source = g.name

		results[i] = v
	}

	return results, nil
}

// Search attempts to query the engine and returns a number of results.
func (g *google) Search(ctx context.Context, category Category, query string, page int) ([]Result, error) {
	form := url.Values{}

	form.Set("q", query)

	switch category {
	case General:
		// This space is intentionally left blank.
	case News:
		form.Set("tbm", "nws")
	default:
		return nil, errors.ErrUnsupported
	}

	if page >= 1 {
		form.Set("start", fmt.Sprint(page*10))
	}

	ctx, cancel := g.http.Context(ctx)
	defer cancel()

	res, err := g.http.Get(
		ctx,
		fmt.Sprintf("https://google.com/search?%s", form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse html: %w", err)
	}

	switch category {
	case General:
		return g.parseGeneral(doc)
	case News:
		return g.parseNews(doc)
	default:
		panic("unreachable")
	}
}

// Ping checks to see if the engine is reachable.
func (g *google) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	res, err := g.http.Get(ctx, "https://google.com")
	if err != nil {
		res.Body.Close()
		return nil
	}
	return err
}