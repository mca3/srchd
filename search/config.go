package search

import (
	"time"
)

// DefaultConfig specifies the fallback configuration values, which are used
// when an engine doesn't have a configuration key explicitly set.
var DefaultConfig = map[string]any{
	"user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.3",
	"timeout":    "10s",
	"debug":      false,

	// This doesn't actually have any effect in this package, but is still
	// used by srchd elsewhere. See ./searcher.go.
	"weight": 1.0,
}

// getConfig returns a config from the passed slice.
// Used for the init functions, which are variadic functions accepting map[string]any.
func getConfig(config []map[string]any) map[string]any {
	if len(config) == 0 {
		return DefaultConfig
	}
	return config[0]
}

// GetConfigValue gets a configuration value from the config.
//
// If the key is not found in the config, then the key is looked up in
// [DefaultConfig].
// If the key is still not found, the zero value and false is returned.
// If the value was unable to be cast to the target type, then the zero value
// and false is also returned.
func GetConfigValue[T any](config map[string]any, key string) (value T, ok bool) {
	var val any

	if config != nil {
		val, ok = config[key]
	}
	if !ok {
		val, ok = DefaultConfig[key]
		if !ok {
			return
		}
	}

	value, ok = val.(T)
	if !ok {
		ok = false
		return
	}
	return
}

func newHttpClient(config map[string]any) *HttpClient {
	// Attempt to parse the timeout duration.
	var timeout time.Duration
	{
		t, ok := GetConfigValue[string](config, "timeout")
		if !ok || t == "" {
			panic("request timeout not specified or is invalid")
		}

		var err error
		timeout, err = time.ParseDuration(t)
		if err != nil {
			panic(err)
		}
	}

	// Attempt to parse the userAgent duration.
	var userAgent string
	{
		var ok bool
		userAgent, ok = GetConfigValue[string](config, "user_agent")
		if !ok || userAgent == "" {
			panic("request user agent not specified or is invalid")
		}
	}

	// Set the debug flag if we need to.
	var debug bool
	{
		var ok bool
		debug, ok = GetConfigValue[bool](config, "debug")
		if !ok {
			panic("debug value is invalid")
		}
	}

	return &HttpClient{
		Timeout:   timeout,
		UserAgent: userAgent,
		Debug:     debug,
	}
}
