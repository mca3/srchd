package engines

import (
	"context"
	"testing"

	"git.sr.ht/~cmcevoy/srchd/search"
)

func TestMediawikiSearch(t *testing.T) {
	d := search.MustInit("mediawiki", "mediawiki", map[string]any{"endpoint": "https://en.wikipedia.org/w/api.php"}).(search.GeneralSearcher)

	res, err := d.GeneralSearch(context.Background(), "hello world", 0)
	if err != nil {
		panic(err)
	} else if len(res) == 0 {
		t.Fatalf("search returned no results")
	}

	for _, r := range res {
		t.Logf("title: %s", r.Title)
		t.Logf("link: %s", r.Link)
		t.Logf("desc: %s", r.Description)
	}
}
