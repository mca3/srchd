package engines

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"git.sr.ht/~cmcevoy/srchd/search"
)

type google struct {
	name  string
	http  *search.HttpClient
	debug bool
}

var (
	_ search.Engine = &google{}
)

func init() {
	search.Add("google", true, func(config search.Config) (search.Engine, error) {
		return &google{
			name:  config.Name,
			http:  config.NewHttpClient(),
			debug: config.Debug,
		}, nil
	})
}

// Parses a general query results page.
func (g *google) parseGeneral(doc *goquery.Document) ([]search.Result, error) {
	// Because we don't use the non-JS variant, we are given an extra class
	// on search results which is *very* helpful!
	// And it doesn't change occasionally.
	elem := doc.Find(`#rso > div .g`)

	// Perfect.
	// So these divs have some child divs, with these contents:
	// 1. header, title, link
	// 2. description (or an image)
	// 3. ??? but it doesn't matter (unless it's the description)
	// In child div 1, we can grab the title from the one and only h3 tag,
	// and also the href.
	// In the second (or third) div, the inner text is the description.
	// And that's all we need!
	results := make([]search.Result, 0, int(elem.Length()))

	for i := 0; i < cap(results); i++ {
		v := search.Result{}

		e := elem.Eq(i)
		if goquery.NodeName(e.Children().First()) == "g-section-with-header" || e.Children().First().HasClass("kp-wholepage") {
			// kp-wholepage:
			// Somehow goquery puts this into my selector even
			// though it should be in #rhs and never actually
			// matter.
			continue
		} else if e.Find(".g .tF2Cxc").Length() > 0 {
			// Handle the first result, which may or may not be special.
			// The class may or may not have to be changed.
			e = e.Find(".g .tF2Cxc").First().Children()
		} else {
			e = e.Children().First().Children()
		}

		title := e.Eq(0).Find("h3")
		link := e.Eq(0).Find("a[href]")
		desc := e.Eq(1).Children()

		v.Title = title.Text()
		v.Link, _ = link.Attr("href")
		v.Link = search.CleanURL(v.Link)
		v.Description = strings.TrimSpace(desc.Text())
		if v.Description == "" {
			// Try the next one over.
			// I tried to make this a less intrusive change but I
			// couldn't get what I wanted to do work.

			desc = e.Eq(2).Children()
			v.Description = strings.TrimSpace(desc.Text())
		}

		// Debug code to log the HTML of results missing some attributes.
		// The code I wrote is buggy and because you aren't really
		// meant to ingest HTML mechanically, prone to breakage.
		if g.debug && (v.Title == "" || v.Link == "" || v.Description == "") {
			// Collect a list of things that are missing.
			missing := []string{}
			if v.Title == "" {
				missing = append(missing, "title")
			}
			if v.Link == "" {
				missing = append(missing, "link")
			}
			if v.Description == "" {
				missing = append(missing, "description")
			}

			ehtml, _ := elem.Eq(i).Html()
			log.Printf("google: missing %v: %v", missing, ehtml)
		}

		v.Sources = []string{g.name}

		results = append(results, v)
	}

	return results, nil
}

// Search attempts to query the engine and returns a number of results.
func (g *google) Search(ctx context.Context, query string, page int) ([]search.Result, error) {
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

// Ping checks to see if the engine is reachable.
func (g *google) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := g.http.Get(ctx, "https://www.google.com/")
	return err
}
