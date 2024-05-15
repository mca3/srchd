package engines

import (
	"testing"

	"git.sr.ht/~cmcevoy/srchd/internal/engtest"
	"git.sr.ht/~cmcevoy/srchd/search"
)

func TestGoogleSearch(t *testing.T) {
	engtest.New("google", search.Config{}).RunTests(t,
		"hello world",
		"wikipedia",
		"big bang theory",
		"exanple", // intentionally misspelled
	)
}
