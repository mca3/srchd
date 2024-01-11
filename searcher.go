package main

import (
	"log"
	"slices"
	"strings"
	"sync"

	"git.int21h.xyz/mwr"
	"git.int21h.xyz/srchd/search"
)

var engines = map[string]search.Engine{}

func findWantedEngines(c *mwr.Ctx) []string {
	request := strings.TrimSpace(c.Cookie("engines"))
	if len(request) == 0 {
		return nil
	}
	return strings.Split(request, ";")
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
		link := normalizeLink(res[i].Link)

		_, ok := firstSeen[link]
		if !ok {
			// First occurrence.
			firstSeen[link] = i
			continue
		}

		// Swap with the last element and shrink the slice.
		res[i], res[len(res)-1] = res[len(res)-1], res[i]
		res = res[:len(res)-1]

		// Decrement i so we can try the next element.
		i--
	}

	// Return the (modified) slice.
	return res
}

// Searches all requested engines.
func doSearch(c *mwr.Ctx, category search.Category, query string, page int) ([]search.Result, error) {
	wg := sync.WaitGroup{}

	wantEngines := findWantedEngines(c)
	results := []search.Result{}
	mu := sync.Mutex{}

	for name, eng := range engines {
		if len(wantEngines) != 0 && !slices.Contains(wantEngines, name) {
			log.Printf("skipping %q, l = %d", name, len(wantEngines))
			continue
		}

		wg.Add(1)
		go func(e search.Engine) {
			defer wg.Done()

			res, err := e.Search(c.Context(), category, query, page)
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
