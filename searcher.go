package main

import (
	"log"
	"slices"
	"sort"
	"strings"
	"sync"

	"git.sr.ht/~cmcevoy/mwr"
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

func findWantedEngines(c *mwr.Ctx) []string {
	request := strings.TrimSpace(c.Cookie("engines"))
	if len(request) == 0 {
		return nil
	}
	return strings.Split(request, ",")
}

func normalizeLink(link string) string {
	// TODO: Properly
	return strings.TrimSuffix(link, "/")
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
		return res[i].Score > res[j].Score
	})

	// Return the (modified) slice.
	return res
}

// Searches all requested engines.
func doSearch(c *mwr.Ctx, category category, query string, page int) ([]search.Result, error) {
	wg := sync.WaitGroup{}

	wantEngines := findWantedEngines(c)
	results := []search.Result{}
	mu := sync.Mutex{}

	for name, eng := range engines {
		if len(wantEngines) != 0 && !slices.Contains(wantEngines, name) {
			continue
		}

		wg.Add(1)
		go func(e search.Engine) {
			defer wg.Done()

			var res []search.Result
			var err error

			switch category {
			case General:
				eng, ok := e.(search.GeneralSearcher)
				if !ok {
					return
				}
				res, err = eng.GeneralSearch(c.Context(), query, page)
			case News:
				eng, ok := e.(search.NewsSearcher)
				if !ok {
					return
				}
				res, err = eng.NewsSearch(c.Context(), query, page)
			case Videos:
				eng, ok := e.(search.VideoSearcher)
				if !ok {
					return
				}
				res, err = eng.VideoSearch(c.Context(), query, page)
			case Images:
				eng, ok := e.(search.ImageSearcher)
				if !ok {
					return
				}
				res, err = eng.ImageSearch(c.Context(), query, page)
			}

			if err != nil {
				log.Printf("search failed: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()

			results = append(results, res...)
		}(eng)
	}

	wg.Wait()

	return mergeResults(results), nil
}
