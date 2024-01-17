package main

import (
	"context"
	"log"
	"sync"
	"time"

	"git.int21h.xyz/srchd/search"
)

var timings = map[string]time.Duration{}
var timingMu sync.RWMutex

// findTimings attempts to find the timings for each configured engine.
func findTimings(ctx context.Context) {
	for name, eng := range engines {
		go func(name string, eng search.Engine) {
			then := time.Now()
			if err := eng.Ping(ctx); err != nil {
				timings[name] = 0
				log.Printf("timing for %s failed: %v", err)
				return
			}

			dur := time.Since(then).Truncate(time.Millisecond)
			log.Printf("timing for %s: %v", name, dur)

			timingMu.Lock()
			timings[name] = dur
			timingMu.Unlock()
		}(name, eng)
	}
}

// Ping loop.
func pinger(ctx context.Context) {
	ticker := time.NewTicker(cfg.PingInterval.Duration)

	defer ticker.Stop()

	for {
		findTimings(ctx)

		select {
		case <-ticker.C:
			// This space is intentionally left blank.
		case <-ctx.Done():
			return
		}
	}
}

func getTiming(name string) time.Duration {
	timingMu.RLock()
	defer timingMu.RUnlock()

	return timings[name]
}
