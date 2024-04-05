package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// User agent to send requests with.
type mediawiki struct {
	name     string
	endpoint string
	http     *HttpClient
}

var (
	_ GeneralSearcher = &mediawiki{}
)

type mediawikiResult struct {
	Titles       []string
	Descriptions []string
	Links        []string
}

func init() {
	// Default is false because this requires configuration
	Add("mediawiki", false, func(name string, config ...map[string]any) (Engine, error) {
		cfg := getConfig(config)
		var ep string
		var ok bool

		if _, ok = cfg["endpoint"]; !ok {
			return nil, errors.New("endpoint not specified")
		}

		if ep, ok = cfg["endpoint"].(string); !ok {
			return nil, errors.New("endpoint is not a string")
		}

		// TODO: Is endpoint valid?

		return &mediawiki{
			name:     name,
			endpoint: ep,
			http:     newHttpClient(cfg),
		}, nil
	})
}

func (w *mediawiki) GeneralSearch(ctx context.Context, query string, page int) ([]Result, error) {
	form := url.Values{}

	if page > 1 {
		// MediaWiki doesn't support offset
		return nil, nil
	}

	form.Set("action", "opensearch")
	form.Set("search", query)
	form.Set("limit", "10") // Arbitrary
	form.Set("profile", "fuzzy")
	form.Set("format", "json")
	form.Set("namespace", "0")

	ctx, cancel := w.http.Context(ctx)
	defer cancel()

	res, err := w.http.Get(ctx, w.endpoint+"?"+form.Encode())
	if err != nil {
		return nil, err
	}

	body, err := res.BodyUncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var wres []any
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&wres); err != nil {
		return nil, err
	}

	if len(wres) != 4 {
		return nil, fmt.Errorf("expected %d arrays, got %d", 4, len(wres))
	}

	// Ensure everything is as we expect
	var titles, descriptions, links []any
	var ok bool
	if titles, ok = wres[1].([]any); !ok {
		return nil, fmt.Errorf("expected []any in second field, got %T", wres[1])
	} else if descriptions, ok = wres[2].([]any); !ok {
		return nil, fmt.Errorf("expected []any in third field, got %T", wres[2])
	} else if links, ok = wres[3].([]any); !ok {
		return nil, fmt.Errorf("expected []any in fourth field, got %T", wres[3])
	}

	results := make([]Result, len(wres[1].([]any)))
	for i := range results {
		// Sanity checking

		title, ok := titles[i].(string)
		if !ok {
			return nil, fmt.Errorf("result %d has invalid title type %T", i, titles[i])
		}

		desc, ok := descriptions[i].(string)
		if !ok {
			return nil, fmt.Errorf("result %d has invalid description type %T", i, descriptions[i])
		}

		link, ok := links[i].(string)
		if !ok {
			return nil, fmt.Errorf("result %d has invalid link type %T", i, links[i])
		}

		// All good!
		results[i] = Result{
			Title:       title,
			Description: desc,
			Link:        link,
		}
	}

	return results, nil
}

// Ping checks to see if the engine is reachable.
func (w *mediawiki) Ping(ctx context.Context) error {
	// Just access the index to see if we're okay.
	_, err := w.http.Get(ctx, w.endpoint)
	return err
}
