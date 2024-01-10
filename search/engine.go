// Package search implements an interface for searching search engines.
package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Engine is an interface that implements the bare essentials for doing web
// searches.
type Engine interface {
	// Search attempts to query the engine and returns a number of results.
	Search(ctx context.Context, category Category, query string, page int) ([]Result, error)

	// Ping checks to see if the engine is reachable.
	Ping(ctx context.Context) error
}

// Result represents a single search result from an [Engine].
type Result struct {
	// Title is the title of the webpage for this result.
	Title string

	// Description is a small snippet of text from the webpage for this
	// result, usually containing a portion or all of the query.
	Description string

	// Link is the URL of this result.
	Link string

	// Source holds an identifier for this search engine.
	Source string
}

type Category int

const (
	General Category = iota
	Videos
	Images
	News
)

var engines = map[string]func(name string, config ...any) (Engine, error){}

// Add adds a search engine to the list of supported engines.
//
// If a name is already in use, Add panics.
func Add(name string, fn func(name string, config ...any) (Engine, error)) {
	if _, ok := engines[name]; ok {
		panic(fmt.Sprintf("name %q already taken", name))
	}

	engines[name] = fn
}

// New creates a new instance of a search engine, given a backend name and an
// ID to tag results with.
//
// If an engine does not exist, Engine will be nil and error will be
// [errors.ErrUnsupported].
func New(engine, name string, config ...any) (Engine, error) {
	fn, ok := engines[engine]
	if !ok {
		return nil, errors.ErrUnsupported
	}

	return fn(name, config...)
}

// Strips the preceeding http:// or https:// from the link.
func (r *Result) FancyURL() string {
	if strings.HasPrefix(r.Link, "http://") {
		return r.Link[len("http://"):]
	} else if strings.HasPrefix(r.Link, "https://") {
		return r.Link[len("https://"):]
	}

	return r.Link
}
