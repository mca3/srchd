package main

import (
	"context"
	"time"
)

var timings = map[string]time.Duration{}

// findTimings attempts to find the timings for each configured engine.
func findTimings(ctx context.Context) {
	for name, eng := range engines {
		then := time.Now()
		if err := eng.Ping(ctx); err != nil {
			timings[name] = 0
			continue
		}
		dur := time.Since(then).Truncate(time.Millisecond)
		timings[name] = dur
	}
}
