package search

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// [Engine] configuration.
// Specifies settings that controls how the engine behaves.
//
// This struct should not be modified once passed to an engine.
//
// The zero-value is safe to use, and the struct itself may be unmarshaled in
// YAML configuration files.
type Config struct {
	// Type determines what backend to use for this engine.
	// For example, if you wanted to use the "example" engine for
	// example.com, then you would put "example" into this field.
	//
	// If you leave this blank, then it defaults to Name.
	// An empty Name and Type is an error.
	Type string `yaml:"type,omitempty"`

	// Name of the engine; when retrieving search results, this is the
	// string that would put be in the "sources" field.
	//
	// If left blank, then it defaults to Type.
	// An empty Name and Type is an error.
	Name string `yaml:"name,omitempty"`

	// User-Agent header value; often set to mimic the Chrome web browser
	// to get past most bot detection.
	//
	// If left empty, then [DefaultUserAgent] is used.
	UserAgent string `yaml:"user_agent,omitempty"`

	// Timeout is the total amount of time an engine will wait to retrieve
	// a full HTTP response.
	//
	// If set to 0, then [DefaultTimeout] is used.
	Timeout stringDuration `yaml:"timeout"`

	// Weight determines the initial amount of score for a result.
	//
	// A zero weight is analogous to a weight of 1.0.
	//
	// Note that this field *should not* affect the engines themselves;
	// this field exists here solely for ranking in srchd.
	Weight float64 `yaml:"weight"`

	// Debug logs extra information when doing HTTP requests, and may also
	// enable debugging features in an engine.
	Debug bool `yaml:"debug"`

	// Extra contains extra settings that have no corresponding field in
	// this struct.
	// They are generally [Engine] specific, and may or may not be
	// optional.
	Extra map[string]any `yaml:"-"`
}

// Wrapper struct to allow decoding time.Duration string values (such as "5s"
// or "15m") directly from YAML.
type stringDuration struct {
	time.Duration
}

// DefaultUserAgent is the user agent that is used when the UserAgent field in
// [Config] is left empty.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.3"

// Default timeout setting.
const DefaultTimeout = time.Second * 10

// Initializes the specified from struct values.
func (c Config) New() (Engine, error) {
	driverType := c.Type
	if driverType == "" {
		// Fallback to c.Name for the driver type.
		driverType = c.Name
	}
	if driverType == "" {
		// Both c.Name and c.Type is empty.
		return nil, fmt.Errorf("engine config has no name or type")
	}

	// At least one of c.Name or c.Type is known to be non-empty so we only
	// have to check once.
	if c.Name == "" {
		// Default to the name of the engine.
		c.Name = c.Type
	}

	// Initialize the driver, if we found it.
	fn, ok := engines[driverType]
	if !ok {
		return nil, fmt.Errorf("engine %q is not known", driverType)
	}
	return fn(c)
}

// MustNew attempts to initialize an [Engine] from the configuration, but
// panics if it fails to do so.
func (c Config) MustNew() Engine {
	e, err := c.New()
	if err != nil {
		panic(err)
	}
	return e
}

// Create a [HttpClient] according to values set in the configuration.
func (c Config) NewHttpClient() *HttpClient {
	// Determine timeout.
	timeout := c.Timeout.Duration
	if timeout <= 0 {
		// Invalid timeout.
		timeout = DefaultTimeout
	}

	// Determine user agent.
	userAgent := c.UserAgent
	if userAgent == "" {
		// Empty user agent, use default.
		userAgent = DefaultUserAgent
	}

	return &HttpClient{
		Timeout:   timeout,
		UserAgent: userAgent,
		Debug:     c.Debug,
	}
}

// UnmarshalJSON parses a JSON configuration.
//
// This is required so we can use extra keys.
func (c *Config) UnmarshalYAML(data *yaml.Node) error {
	// Long story short here: We cannot get unknown keys through yaml
	// unless we use some tricks and unmarshal twice.
	// It's not ideal, but we only do this once on startup.

	// Ensure we even have the right datatype to begin with.
	if data.Kind != yaml.MappingNode {
		// TODO: This is not the way to go.
		return fmt.Errorf("expected mapping, got %v", data.Kind)
	}

	// Define a new type to lose all receiver functions.
	// This means that Config won't satisfy yaml.Unmarshaler anymore, so no
	// recursion occurs.
	type _Config Config

	// Attempt to unmarshal the data into our new type.
	var d _Config
	if err := data.Decode(&d); err != nil {
		return err
	}

	// Now unmarshal b again, but this time into "Extra".
	// This includes all of the extra keys.
	if err := data.Decode(&d.Extra); err != nil {
		return err
	}

	// Cleanup.
	// Since we parsed it as map[string]any, it includes *all* keys, even
	// those which have a corresponding field.
	// Remove those.
	for _, key := range []string{"type", "name", "user_agent", "timeout", "weight", "debug"} {
		delete(d.Extra, key)
	}

	// Set the receiver to the parsed config and return nil.
	*c = Config(d)
	return nil
}

// UnmarshalYAML attempts to parse a string into a [time.Duration].
func (t *stringDuration) UnmarshalYAML(data *yaml.Node) error {
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
