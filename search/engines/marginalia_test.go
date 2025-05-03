package engines

import (
	"testing"

	"git.sr.ht/~cmcevoy/srchd/internal/engtest"
	"git.sr.ht/~cmcevoy/srchd/search"
)

func TestMarginaliaSearch(t *testing.T) {
	engtest.New("marginalia", search.Config{}).RunTests(t,
		"hello world",
		"wikipedia",
		"big bang theory",
		"exanple", // intentionally misspelled
	)
}
