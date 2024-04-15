package main

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"git.sr.ht/~cmcevoy/srchd/search"
)

var engines = map[string]search.Engine{}

const maxTitleLen = 100
const maxDescriptionLen = 300

// Determines the default set of requested engines from the request.
func findWantedEngines(r *http.Request) []string {
	cookie, err := r.Cookie("engines")
	if err != nil {
		return nil
	}

	return strings.Split(strings.TrimSpace(cookie.Value), ",")
}

// Handles the ':' search operator which specifies specific engines to search.
func processOperators(query string) (requestedEngines []string, newQuery string) {
	if !strings.ContainsRune(query, ':') {
		// No colon operator, so nothing to change.
		return nil, query
	}

	toks := strings.Split(query, " ")

	for i, tok := range toks {
		if strings.HasPrefix(tok, "\\:") {
			toks[i] = tok[1:] // Remove the backslash
		} else if strings.HasPrefix(tok, ":") {
			// Just set it to a blank string.
			// This should change nothing with a search query.
			toks[i] = ""
			requestedEngines = append(requestedEngines, tok[1:])
		}
	}

	newQuery = strings.TrimSpace(strings.Join(toks, " "))
	return
}

func normalizeLink(link string) string {
	// TODO: Properly
	return strings.TrimSuffix(link, "/")
}

// Calculates the multiplier of the result score.
func calculateWeight(res search.Result) float64 {
	sum := 0.0

	for _, name := range res.Sources {
		val := 1.0

		// Override the above value if there was one set.
		engineConfig, ok := cfg.Engines[name]
		if ok {
			// Ensure the weight is 1.
			val = engineConfig.Weight
			if val == 0 {
				val = 1
			}
		}

		sum += val
	}

	return sum
}

// Calculates the score to sort against.
func calculateSortingScore(res search.Result) float64 {
	weight := calculateWeight(res)
	return weight * res.Score
}

// Truncates a string to n letters.
func truncate(s string, n int) string {
	if len(s) <= n || utf8.RuneCountInString(s) <= n {
		return s
	}

	// We must trim.
	// Go provides zero handholding when it comes to this, so this is
	// implemented using slices.
	ptr := 0
	for pos := range s {
		if ptr == n {
			return s[:pos] + "…"
		}
		ptr++
	}

	panic("unreachable")
}

// Merges and sorts results.
func processResults(res []search.Result) []search.Result {
	// Track the first time we see a link and move stuff around.
	firstSeen := map[string]int{}

	for i := 0; i < len(res); i++ {
		// Update link. TODO: Move this out of here, maybe.
		link := rewriteUrl(normalizeLink(res[i].Link))
		res[i].Link = link

		idx, ok := firstSeen[link]
		if !ok {
			// First occurrence.
			firstSeen[link] = i

			if res[i].Score == 0 {
				res[i].Score = 1
			}

			// Ensure all fields are proper before we continue.
			res[i].Title = truncate(res[i].Title, maxTitleLen)
			res[i].Description = truncate(res[i].Description, maxDescriptionLen)
			continue
		}

		// Add in the engine source(s).
		// Technically there's only supposed to be one, so this may be unnecessary.
		for _, name := range res[i].Sources {
			if !slices.Contains(res[idx].Sources, name) {
				res[idx].Sources = append(res[idx].Sources, name)
			}
		}

		// If we're missing text, replace it with this.
		if res[idx].Title == "" {
			res[idx].Title = truncate(res[i].Title, maxTitleLen)
		}
		if res[idx].Description == "" {
			res[idx].Description = truncate(res[i].Description, maxDescriptionLen)
		}

		// Swap with the last element and shrink the slice.
		res[i], res[len(res)-1] = res[len(res)-1], res[i]
		res = res[:len(res)-1]

		// Increment the score.
		// This is for sorting; results seen several times will appear
		// higher in the search results.
		res[idx].Score++

		// Decrement i so we can try the next element.
		i--
	}

	// Sort based upon the score.
	sort.Slice(res, func(i, j int) bool {
		// > is used so the results are descending and not ascending.
		return calculateSortingScore(res[i]) > calculateSortingScore(res[j])
	})

	// Return the (modified) slice.
	return res
}

// Searches all requested engines.
func doSearch(r *http.Request, requestQuery string, page int) ([]search.Result, map[string]error, error) {
	wg := sync.WaitGroup{}

	wantEngines, query := processOperators(requestQuery)
	if len(wantEngines) == 0 {
		wantEngines = findWantedEngines(r)
	}

	if len(query) == 0 {
		// Empty queries are likely an error.
		return nil, nil, fmt.Errorf("empty query")
	}

	var errors map[string]error
	results := []search.Result{}
	mu := sync.Mutex{}

	for name, eng := range engines {
		if len(wantEngines) > 0 && !slices.Contains(wantEngines, name) {
			continue
		}

		wg.Add(1)
		go func(name string, e search.Engine) {
			defer wg.Done()

			res, err := e.Search(r.Context(), query, page)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if errors == nil {
					// Lazily initialize the map.
					// In most cases, there will be no errors so there's no point in allocating it.
					errors = map[string]error{}
				}

				errors[name] = err
				log.Printf("searching %q failed: %v", name, err)

				return
			}

			results = append(results, res...)
		}(name, eng)
	}

	wg.Wait()

	log.Println(len(results))
	return processResults(results), errors, nil
}
