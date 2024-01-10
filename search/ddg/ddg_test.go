package ddg

import (
	"context"
	"fmt"
	"testing"

	"git.int21h.xyz/srchd/search"
)

func TestPageParams(t *testing.T) {
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

func TestSearch(t *testing.T) {
	d := &ddg{
		http: &search.HttpClient{},
		vqd:  map[string]string{},
	}

	res, err := d.Search(context.Background(), search.General, "hello world", 0)
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
	res, err = d.Search(context.Background(), search.General, "hello world", 1)
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
	res, err = d.Search(context.Background(), search.General, "hello world", 2)
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
