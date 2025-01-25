package engines

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"git.sr.ht/~cmcevoy/srchd/search"
)

// User agent to send requests with.
type ddg struct {
	name string
	http *search.HttpClient

	// vqd parameter.
	//
	// It's different for every query, and if we don't have it then DDG bot
	// detection might kick off.
	// It may be useful to employ a key-value store of sorts, such as Redis.
	vqd   map[string]string
	vqdMu sync.RWMutex
}

var (
	_ search.Engine = &ddg{}
)

func init() {
	search.Add("ddg", true, func(config search.Config) (search.Engine, error) {
		return &ddg{
			name: config.Name,
			http: config.NewHttpClient(),
			vqd:  map[string]string{},
		}, nil
	})
}

func (d *ddg) lookupVqd(query string) string {
	d.vqdMu.RLock()
	defer d.vqdMu.RUnlock()

	vqd, ok := d.vqd[query]
	if !ok {
		return ""
	}
	return vqd
}

func (d *ddg) setVqd(query, val string) {
	d.vqdMu.Lock()
	defer d.vqdMu.Unlock()

	d.vqd[query] = val
}

// Determines the s and dc parameters from a page number.
//
// This function exists primarily for testing.
func determinePageParams(page int) (s int, dc int) {
	// TODO: These values seem to change. I don't know why or how.
	// These values are just what I got while reverse engineering on my
	// own.

	switch page {
	case 0:
		// s and dc are zero on page 0.
		// This space is therefore intentionally left blank.
	case 1:
		// 24 results on page 1. I don't know why.
		s = 24
		dc = 25
	default:
		// Pages after the first page returns 50 results each.
		s = 24 + (page-1)*50
		dc = 25 + (page-1)*50
	}

	return
}

// Decodes the DDG href.
func decodeDDGHref(href string) string {
	i := strings.IndexRune(href, '?')
	if i == -1 {
		return ""
	}

	href = href[i+1:]
	if href == "" {
		return ""
	}

	if href[:4] == "uddg=" {
		s := strings.IndexRune(href, '=')
		e := strings.IndexRune(href, '&')
		if e == -1 {
			e = len(href)
		}
		href = href[s:e]
		href, _ = url.QueryUnescape(href)
		return href
	}

	v, err := url.ParseQuery(href)
	if err != nil {
		return ""
	}
	return v.Get("uddg")
}

// Escapes bangs when they appear in the query.
func encodeDDGQuery(query string) string {
	if !strings.ContainsRune(query, '!') {
		return query
	}

	toks := strings.Split(query, " ")
	for i, tok := range toks {
		if strings.HasPrefix(tok, "!") {
			toks[i] = "'" + tok + "'"
		}
	}

	return strings.Join(toks, " ")
}

// Search attempts to query the engine and returns a number of results.
func (d *ddg) Search(ctx context.Context, query string, page int) ([]search.Result, error) {
	form := url.Values{}

	form.Set("q", encodeDDGQuery(query))
	if vqd := d.lookupVqd(query); vqd != "" {
		form.Set("vqd", vqd)
	}

	if page >= 1 {
		// These are not present in the initial request.
		form.Set("api", "d.js")
		form.Set("o", "json")
		form.Set("v", "l")
		form.Set("nextParams", "")

		// Set s and dc.
		s, dc := determinePageParams(page)
		form.Set("s", fmt.Sprint(s))
		form.Set("dc", fmt.Sprint(dc))
	}

	ctx, cancel := d.http.Context(ctx)
	defer cancel()

	doc, err := d.http.HtmlPost(
		ctx,
		"https://lite.duckduckgo.com/lite/",
		"application/x-www-form-urlencoded",
		[]byte(form.Encode()),
	)
	if err != nil {
		// Special case for captcha status code
		var h search.HttpError
		if errors.As(err, &h) && h.Status == 202 {
			return nil, search.ErrCaptcha
		}

		return nil, err
	}

	// Update vqd value.
	vqd, _ := doc.Find(`input[name="vqd"]`).Attr("value")
	d.setVqd(query, vqd)

	elem := doc.Find(`div.filters > table[border="0"]`).Eq(2).Children().Children()
	// first children selects the tbody, second children selects the children of the tbody

	// DDG lays out results like this using tr:
	// 1. number, title, encoded link
	// 2. description
	// 3. timestamp
	// 4. nothing
	// So, the max number of results is floor(number of children / 4).
	// Ads are stripped out, of course, and it's unknown how many of those
	// there are.

	results := make([]search.Result, 0, int(elem.Length()/4))

	for i := 0; i < int(elem.Length()/4); i++ {
		v := search.Result{}

		header := elem.Eq(i * 4)
		link := header.Find("a.result-link")

		desc := elem.Eq((i * 4) + 1).First().Find("tr .result-snippet")

		// Check for ads.
		v.Link, _ = link.Attr("href")

		// NOTE: For some reason, when I made my requests match Chrome
		// more, I stopped needing to decode hrefs.
		// v.Link = decodeDDGHref(v.Link)

		if strings.HasPrefix(v.Link, "https://duckduckgo.com/y.js") || v.Link == "" {
			continue
		}

		v.Link = search.CleanURL(v.Link)
		v.Title = link.Text()
		v.Description = strings.TrimSpace(desc.Text())
		v.Sources = []string{d.name}

		results = append(results, v)
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (d *ddg) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := d.http.Get(ctx, "https://lite.duckduckgo.com/lite")
	return err
}
