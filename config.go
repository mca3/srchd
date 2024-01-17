package main

import (
	"encoding/json"
	"os"
	"regexp"
	"time"

	"git.int21h.xyz/srchd/search"
)

type config struct {
	Addr         string
	Engines      []string
	Rewrite      []rewriteRule
	PingInterval timeDuration `json:"ping_interval"`
}

// timeDuration is a wrapper on time.Duration which allows the decoding of
// time.Duration values.
type timeDuration struct {
	time.Duration
}

type rewriteRule struct {
	Regexp      string `json:"find"`
	ReplaceWith string `json:"replace"`

	r *regexp.Regexp
}

var defaultConfig = config{
	Addr:         ":8080",
	Engines:      search.Supported(),
	PingInterval: timeDuration{time.Minute * 15},
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

func (t *timeDuration) UnmarshalJSON(data []byte) error {
	var err error

	str := ""
	if err = json.Unmarshal(data, &str); err != nil {
		return err
	}

	t.Duration, err = time.ParseDuration(str)
	return err
}
