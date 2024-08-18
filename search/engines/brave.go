package engines

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"

	"git.sr.ht/~cmcevoy/srchd/search"
)

type brave struct {
	name string
	http *search.FasthttpClient
}

var (
	_ search.Engine = &brave{}
)

func init() {
	search.Add("brave", true, func(config search.Config) (search.Engine, error) {
		// Brave returns a lot of content in its headers, so we need to
		// increase the size that FastHTTP is willing to read
		cli := config.NewFasthttpClient()
		cli.Client().ReadBufferSize = 8192

		return &brave{
			name: config.Name,
			http: cli,
		}, nil
	})
}

// Search attempts to query the engine and returns a number of results.
func (b *brave) Search(ctx context.Context, query string, page int) ([]search.Result, error) {
	form := url.Values{}

	form.Set("q", query)

	if page >= 1 {
		form.Set("offset", fmt.Sprint(page))
	}

	ctx, cancel := b.http.Context(ctx)
	defer cancel()

	res, err := b.http.Get(
		ctx,
		"https://search.brave.com/search?"+form.Encode(),
	)
	if err != nil {
		return nil, err
	}

	var body []byte
	if string(res.Header.ContentEncoding()) == "br" {
		// We need to handle Brotli a little specially because the
		// decompresser we're using doesn't like Brave's response for
		// some reason.
		//
		// The error from the decompressor isn't fatal but it is
		// treated as such:
		// https://github.com/andybalholm/brotli/blob/57434b509141a6ee9681116b8d552069126e615f/reader.go#L74-L76
		// https://github.com/valyala/fasthttp/blob/b06f4e21d918faa84ae0aa12c9e4dc7285b9767e/http.go#L505-L512
		//
		// So my crappy solution is to rewrite the part where it
		// decompresses Brotli into a byte buffer and explicitly ignore
		// that "brotli: excessive input" error.
		br := brotli.NewReader(bytes.NewReader(res.Body()))
		body, err = io.ReadAll(br)
		if err != nil && err.Error() != "brotli: excessive input" {
			//       ^ I told you this sucked!
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
	} else {
		body, err = res.BodyUncompressed()
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("unable to parse html: %w", err)
	}

	elem := doc.Find(`#results .snippet[data-type]`)

	// Brave results are laid out like this:
	// .h: link (a element)
	// .title: you can probably guess what this is
	// .snippet-description: ditto

	results := make([]search.Result, 0, int(elem.Length()))

	for i := 0; i < cap(results); i++ {
		v := search.Result{}
		e := elem.Eq(i)

		link := e.Find("a.h")
		v.Link, _ = link.Attr("href")
		v.Link = search.CleanURL(v.Link)

		title := e.Find(".title")
		v.Title = title.Text()

		v.Description = e.Find(".snippet-description").Text()
		v.Sources = []string{b.name}

		results = append(results, v)
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (b *brave) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := b.http.Get(ctx, "https://search.brave.com/")
	return err
}
