package main

import (
	"encoding/json"
	"os"
	"regexp"
	"time"

	"git.sr.ht/~cmcevoy/srchd/search"
)

type config struct {
	Addr         string
	Engines      []string
	Rewrite      []rewriteRule
	PingInterval timeDuration `json:"ping_interval"`
	BaseURL      string       `json:"base_url"`

	EngineConfig map[string]map[string]any `json:"engine_config"`
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
	BaseURL:      "http://localhost:8080",
	Engines:      search.Supported(),
	PingInterval: timeDuration{time.Minute * 15},

	EngineConfig: map[string]map[string]any{
		"default": search.DefaultConfig,
	},
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

// Attempts to initialize an engine.
//
// Uses the engine's configuration as specified in the configuration, and also
// merges in the default config.
func initializeEngine(driver, name string) (search.Engine, error) {
	engineCfg, ok := cfg.EngineConfig[name]
	if !ok {
		engineCfg = cfg.EngineConfig["default"]

		// No need to merge in the default config.
		goto done
	}

	// Set all of the values of the default configuration.
	for k, v := range cfg.EngineConfig["default"] {
		_, ok := engineCfg[k]
		if ok {
			// Don't overwrite.
			continue
		}

		engineCfg[k] = v
	}

done:
	return search.New(driver, name, engineCfg)
}
