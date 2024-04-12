package search

import (
	"fmt"
	"regexp"
)

// Parameter names to remove.
//
// Automatically compiled at startup; see init.
var stripParams = []string{
	"ref",
	"refid",
	"ref_[a-z]*",
	"referrer",
	"utm_[a-z_]*",

	// The following are included exclusively for link normalization.
	"hl",
}

// Contains compiled regular expressions used for cleaning URLs.
var cleaningRegexps []*regexp.Regexp

func init() {
	// Compile everything in stripParams.
	cleaningRegexps = make([]*regexp.Regexp, len(stripParams))
	for i, v := range stripParams {
		cleaningRegexps[i] = regexp.MustCompile(fmt.Sprintf(`[?&]%s=[^&]*`, v))
	}
}

// Removes tracking parameters from URLs.
func CleanURL(url string) string {
	for _, re := range cleaningRegexps {
		// This is written this way for two reasons:
		// - It allows us to check if the string starts with '?' so we
		//   don't return urls like
		//   'https://example.com/test&thisis=broken'
		// - We use the indicies so we don't have to run the regexp
		//   again
		// It's probably overengineered, but it does the job.

		loc := re.FindStringIndex(url)
		if loc == nil {
			continue
		}

		if url[loc[0]] == '?' && loc[1] < len(url) {
			// Replace it with a ?
			url = fmt.Sprintf("%s?%s", url[:loc[0]], url[loc[1]+1:])
		} else {
			// Just put them together (replace with nothing)
			url = url[:loc[0]] + url[loc[1]:]
		}
	}

	return url
}
