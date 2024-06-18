package search

import (
	"testing"
)

func TestCleanURL(t *testing.T) {
	tests := []struct{ in, out string }{
		{"http://example.com", "http://example.com"},
		{"http://example.com/test", "http://example.com/test"},
		{"http://example.com/test?ref=123", "http://example.com/test"},
		{"http://example.com/test?ref=123&number=42", "http://example.com/test?number=42"},
		{"http://example.com/test?number=42&ref=123", "http://example.com/test?number=42"},
		{"http://example.com/test#abc", "http://example.com/test"},
		{"http://example.com/?hl=en_US", "http://example.com/"},
	}

	for _, v := range tests {
		t.Run(v.in, func(t *testing.T) {
			res := CleanURL(v.in)
			if res != v.out {
				t.Errorf("%q != %q", res, v.out)
			}
		})
	}
}
