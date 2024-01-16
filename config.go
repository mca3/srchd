package main

import (
	"encoding/json"
	"os"
	"regexp"

	"git.int21h.xyz/srchd/search"
)

type config struct {
	Addr    string
	Engines []string
	Rewrite []rewriteRule
}

type rewriteRule struct {
	Regexp      string `json:"find"`
	ReplaceWith string `json:"replace"`

	r *regexp.Regexp
}

var defaultConfig = config{
	Addr:    ":8080",
	Engines: search.Supported(),
}

var cfg = defaultConfig

func loadConfig(path string) error {
	h, err := os.Open(path)
	if err != nil {
		return err
	}
	defer h.Close()

	if err := json.NewDecoder(h).Decode(&cfg); err != nil {
		return err
	}

	// Load all of the regexp rules
	for i, v := range cfg.Rewrite {
		v.r = regexp.MustCompile(v.Regexp)
		cfg.Rewrite[i] = v
	}

	return nil
}

// Attempt to rewrite a URL.
//
// Stops on the first rule that matches the URL.
func rewriteUrl(url string) string {
	for _, v := range cfg.Rewrite {
		if v.r.MatchString(url) {
			return v.r.ReplaceAllString(url, v.ReplaceWith)
		}
	}

	return url
}
