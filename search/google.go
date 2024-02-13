package search

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type google struct {
	name string
	http *HttpClient
}

var (
	_ GeneralSearcher = &google{}
	_ NewsSearcher    = &google{}
)

func init() {
	Add("google", func(name string, config ...map[string]any) (Engine, error) {
		cfg := getConfig(config)

		return &google{
			name: name,
			http: newHttpClient(cfg),
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
		v.Sources = []string{g.name}

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
		v.Sources = []string{g.name}

		results[i] = v
	}

	return results, nil
}

// Search attempts to query the engine and returns a number of results.
func (g *google) GeneralSearch(ctx context.Context, query string, page int) ([]Result, error) {
	form := url.Values{}

	form.Set("q", query)

	if page >= 1 {
		form.Set("start", fmt.Sprint(page*10))
	}

	ctx, cancel := g.http.Context(ctx)
	defer cancel()

	res, err := g.http.Get(
		ctx,
		fmt.Sprintf("https://www.google.com/search?%s", form.Encode()),
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

	return g.parseGeneral(doc)
}

// NewsSearch attempts to query the engine and returns a number of results.
func (g *google) NewsSearch(ctx context.Context, query string, page int) ([]Result, error) {
	form := url.Values{}

	form.Set("q", query)
	form.Set("tbm", "nws")

	if page >= 1 {
		form.Set("start", fmt.Sprint(page*10))
	}

	ctx, cancel := g.http.Context(ctx)
	defer cancel()

	res, err := g.http.Get(
		ctx,
		fmt.Sprintf("https://www.google.com/search?%s", form.Encode()),
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

	return g.parseNews(doc)
}

// Ping checks to see if the engine is reachable.
func (g *google) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := g.http.Get(ctx, "https://google.com/")
	return err
}
