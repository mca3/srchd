package main

import (
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"

	"git.sr.ht/~cmcevoy/srchd/search"
)

var engines = map[string]search.Engine{}

type category int

const (
	General category = iota
	News
	Videos
	Images
)

func findWantedEngines(r *http.Request) []string {
	cookie, err := r.Cookie("engines")
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(cookie.Value), ",")
}

func normalizeLink(link string) string {
	// TODO: Properly
	return strings.TrimSuffix(link, "/")
}

// Calculates the multiplier of the result score.
func calculateWeight(res search.Result) float64 {
	sum := 0.0

	for _, name := range res.Sources {
		val, ok := search.GetConfigValue[float64](cfg.EngineConfig[name], "weight")
		if !ok {
			val = 1
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

// Drops result entries if the link was already seen earlier in the result slice.
func mergeResults(res []search.Result) []search.Result {
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
			continue
		}

		// Add in the engine source(s).
		// Technically there's only supposed to be one, so this may be unnecessary.
		for _, name := range res[i].Sources {
			if !slices.Contains(res[idx].Sources, name) {
				res[idx].Sources = append(res[idx].Sources, name)
			}
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
func doSearch(r *http.Request, category category, query string, page int) ([]search.Result, map[string]error, error) {
	wg := sync.WaitGroup{}

	wantEngines := findWantedEngines(r)
	var errors map[string]error
	results := []search.Result{}
	mu := sync.Mutex{}

	for name, eng := range engines {
		if len(wantEngines) != 0 && !slices.Contains(wantEngines, name) {
			continue
		}

		wg.Add(1)
		go func(name string, e search.Engine) {
			defer wg.Done()

			var res []search.Result
			var err error

			switch category {
			case General:
				eng, ok := e.(search.GeneralSearcher)
				if !ok {
					return
				}
				res, err = eng.GeneralSearch(r.Context(), query, page)
			case News:
				eng, ok := e.(search.NewsSearcher)
				if !ok {
					return
				}
				res, err = eng.NewsSearch(r.Context(), query, page)
			case Videos:
				eng, ok := e.(search.VideoSearcher)
				if !ok {
					return
				}
				res, err = eng.VideoSearch(r.Context(), query, page)
			case Images:
				eng, ok := e.(search.ImageSearcher)
				if !ok {
					return
				}
				res, err = eng.ImageSearch(r.Context(), query, page)
			}

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

	return mergeResults(results), errors, nil
}
