package main

import (
	"log"
	"sync"
	"strings"
	"slices"

	"git.int21h.xyz/mwr"
	"git.int21h.xyz/srchd/search"
)

var engines = map[string]search.Engine{}

func findWantedEngines(c *mwr.Ctx) []string {
	request := c.Cookie("engines")
	if len(request) == 0 {
		return nil
	}
	return strings.Split(request, ";")
}

func doSearch(c *mwr.Ctx, query string, page int) ([]search.Result, error) {
	wg := sync.WaitGroup{}

	wantEngines := findWantedEngines(c)
	results := []search.Result{}
	mu := sync.Mutex{}

	log.Println(wantEngines)

	for name, eng := range engines {
		if len(wantEngines) != 0 && !slices.Contains(wantEngines, name) {
			log.Printf("skipping %q, l = %d", name, len(wantEngines))
			continue
		}

		wg.Add(1)
		go func(e search.Engine) {
			defer wg.Done()

			res, err := e.Search(c.Context(), query, page)
			if err != nil {
				log.Printf("search failed: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()

			results = append(results, res...)
		}(eng)
	}

	wg.Wait()
	return results, nil
}
