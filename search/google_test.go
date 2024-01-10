package search

import (
	"context"
	"testing"
)

func TestGoogleSearch(t *testing.T) {
	d := &google{
		http: &HttpClient{},
	}

	res, err := d.Search(context.Background(), General, "hello world", 0)
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
	res, err = d.Search(context.Background(), General, "hello world", 1)
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
	res, err = d.Search(context.Background(), General, "hello world", 2)
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
