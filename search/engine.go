// Package search implements an interface for searching search engines.
package search

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Engine is an interface that implements the bare essentials for doing web
// searches.
type Engine interface {
	// Ping checks to see if the engine is reachable.
	Ping(ctx context.Context) error

	// Search attempts to query the engine and returns a number of results.
	Search(ctx context.Context, query string, page int) ([]Result, error)
}

// An Initializer is a function that initializes an engine from a config.
type Initializer func(config Config) (Engine, error)

// Result represents a single search result from an [Engine].
type Result struct {
	// Title is the title of the webpage for this result.
	Title string

	// Description is a small snippet of text from the webpage for this
	// result, usually containing a portion or all of the query.
	Description string

	// Link is the URL of this result.
	Link string

	// Sources holds all engine names that had this result.
	//
	// Engines must only populate this with their name.
	// Results are merged and this field will be populated based upon what
	// engines return a result similar to this one.
	Sources []string

	// Score holds the score for this result.
	//
	// Engines should not fill this value.
	Score float64
}

var engines = map[string]Initializer{}
var defaultEngines = map[string]struct{}{}

var Supported = sync.OnceValue(func() []string {
	supportedEngines := []string{}
	for name := range engines {
		supportedEngines = append(supportedEngines, name)
	}
	return supportedEngines
})

var DefaultEngines = sync.OnceValue(func() []string {
	engines := []string{}
	for name := range defaultEngines {
		engines = append(engines, name)
	}
	return engines
})

// Add adds a search engine to the list of supported engines.
//
// If a name is already in use, Add panics.
func Add(name string, isDefault bool, fn Initializer) {
	if _, ok := engines[name]; ok {
		panic(fmt.Sprintf("name %q already taken", name))
	}

	engines[name] = fn
	if isDefault {
		defaultEngines[name] = struct{}{}
	}
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
