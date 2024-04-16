package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sync"
	"time"

	"git.sr.ht/~cmcevoy/srchd/search"
	"gopkg.in/yaml.v3"
)

type config struct {
	// Addr is the address that the HTTP server listens on.
	Addr string

	// BaseURL is the address that the HTTP server is *served* on.
	// This can be different, such as if you are listening on
	// localhost:8080 but access it through example.com.
	BaseURL string `yaml:"base_url"`

	// Rewrite/drop rules.
	Rewrite []rewriteRule

	// Determines the interval to check the connection to certain engines.
	PingInterval timeDuration `yaml:"ping_interval"`

	// Pprof specifies an address to serve pprof on.
	// It cannot listen on the same port as Addr.
	//
	// This has no default and should be turned off unless you know what
	// you are doing.
	Pprof string

	// Engines specifies configuration for engines.
	//
	// An engine that is in here is implicitly enabled unless it also
	// exists in the Disabled field.
	Engines map[string]search.Config `yaml:"engines"`

	// Disabled lists the names of engines to not initialize.
	Disabled []string `yaml:"disabled"`
}

// timeDuration is a wrapper on time.Duration which allows the decoding of
// time.Duration values.
type timeDuration struct {
	time.Duration
}

type rewriteRule struct {
	// Regular expression that matches against the link of a search result.
	Regexp string `yaml:"find"`

	// Matches an exact hostname.
	Hostname string `yaml:"hostname"`

	// Replace the affected part with this value.
	ReplaceWith string `yaml:"replace"`

	r *regexp.Regexp
}

var defaultConfig = config{
	Addr:         ":8080",
	BaseURL:      "http://localhost:8080",
	PingInterval: timeDuration{time.Minute * 15},

	Engines: map[string]search.Config{},
}

var cfg = defaultConfig

func loadConfig(path string) error {
	h, err := os.Open(path)
	if err != nil {
		return err
	}
	defer h.Close()

	if err := yaml.NewDecoder(h).Decode(&cfg); err != nil {
		return err
	}

	// Load all of the regexp rules
	for i, v := range cfg.Rewrite {
		if v.Regexp != "" && v.Hostname != "" {
			return fmt.Errorf("regexp and hostname defined in rule")
		}

		if v.Hostname == "" {
			v.r = regexp.MustCompile(v.Regexp)
			cfg.Rewrite[i] = v
		}
	}

	return nil
}

// Attempt to rewrite a URL.
//
// Stops on the first rule that matches the URL.
func rewriteUrl(in string) string {
	var parsedUrl *url.URL
	var err error
	for _, v := range cfg.Rewrite {
		if v.r != nil {
			// v.r != nil when v.Hostname == ""

			if v.r.MatchString(in) {
				return v.r.ReplaceAllString(in, v.ReplaceWith)
			}
		} else if err == nil { // v.Hostname != ""
			// Note that err == nil is checked because we lazily
			// initialize the URL, and we get err for free if we
			// failed to parse the URL.

			// Lazily parse the URL.
			if parsedUrl == nil {
				parsedUrl, err = url.Parse(in)
				if err != nil {
					continue
				}
			}

			// Swap out the hostname.
			if parsedUrl.Host == v.Hostname {
				parsedUrl.Host = v.ReplaceWith
				return parsedUrl.String()
			}
		}
	}

	return in
}

func (t *timeDuration) UnmarshalYAML(data *yaml.Node) error {
	// This looks extremely weird, and I agree, but the point is that the
	// line below checks to see if data is a string or not.
	// I have taken the time to comprehend the docs just enough to say this.
	if data.Kind != yaml.ScalarNode || data.Tag != "!!str" {
		return fmt.Errorf("expected string, got %v", data.Tag)
	}

	var err error
	t.Duration, err = time.ParseDuration(data.Value)
	return err
}

// Attempts to initialize an engine.
//
// Uses the engine's configuration as specified in the configuration, and also
// merges in the default config.
func initializeEngine(name string) (search.Engine, error) {
	cfg, ok := cfg.Engines[name]
	if !ok {
		cfg.Type = name
	}
	cfg.Name = name

	return cfg.New()
}

// Determines if a specific engine has been disabled.
//
// An engine is disabled if it is explicitly disabled, or if it has no
// configuration and is not an engine enabled by default.
func engineIsDisabled(name string) bool {
	// Check if it is explicitly disabled first.
	if slices.Contains(cfg.Disabled, name) {
		// Engine was disabled.
		return true
	}

	// Not explicitly disabled.
	// Check to see if it was configured.
	_, ok := cfg.Engines[name]
	if ok {
		// Was configured.
		return false
	}

	// Finally, check to see if it is supposed to be enabled by default.
	if slices.Contains(search.DefaultEngines(), name) {
		// Enabled by default.
		return false
	}

	// All of the above checks failed, so it is disabled.
	return true
}

// Returns a list of enabled engines.
//
// An enabled engine is one that is not explicitly disabled and is either set
// to be a "default" engine or has been configured.
var enabledEngines = sync.OnceValue(func() []string {
	engines := make([]string, 0)

	// Copy in all default engines provided they aren't disabled.
	for _, v := range search.DefaultEngines() {
		if !engineIsDisabled(v) {
			engines = append(engines, v)
		}
	}

	// Copy in everything that was explicitly configured.
	for k := range cfg.Engines {
		if !engineIsDisabled(k) && !slices.Contains(engines, k) {
			engines = append(engines, k)
		}
	}

	return engines
})
