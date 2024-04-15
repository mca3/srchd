package engines

import (
	"context"
	"fmt"
	"testing"

	"git.sr.ht/~cmcevoy/srchd/search"
)

func TestDDGPageParams(t *testing.T) {
	// This setup is completely unnecessary but it does verify my math.

	tests := []struct{ page, s, dc int }{
		{0, 0, 0},
		{1, 24, 25},
		{2, 74, 75},
		{3, 124, 125},
		{4, 174, 175},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.page), func(t *testing.T) {
			s, dc := determinePageParams(test.page)
			if s != test.s || dc != test.dc {
				t.Fatalf("s = %d, wanted %d; dc = %d, wanted %d", s, test.s, dc, test.dc)
			}
		})
	}
}

func TestDDGSearch(t *testing.T) {
	d := (search.Config{Type: "ddg"}).MustNew()

	res, err := d.Search(context.Background(), "hello world", 0)
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

	// Ensure page 1 has results
	res, err = d.Search(context.Background(), "hello world", 1)
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

	// Ensure page 2 has results
	res, err = d.Search(context.Background(), "hello world", 2)
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
