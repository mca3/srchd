package engines

import (
	"testing"

	"git.sr.ht/~cmcevoy/srchd/internal/engtest"
	"git.sr.ht/~cmcevoy/srchd/search"
)

func TestYahooSearch(t *testing.T) {
	engtest.New("yahoo", search.Config{}).RunTests(t,
		"hello world",
		"wikipedia",
		"big bang theory",
		"exanple", // intentionally misspelled
	)
}
