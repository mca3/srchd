package main

import (
	"context"
	"log"
	"sync"
	"time"

	"git.sr.ht/~cmcevoy/srchd/search"
)

var engineLatency = map[string]time.Duration{}
var engineLatencyMu sync.RWMutex

// Ping loop.
func pinger(ctx context.Context) {
	ticker := time.NewTicker(cfg.PingInterval.Duration)
	defer ticker.Stop()

	fn := func(name string, eng search.Engine) {
		then := time.Now()
		if err := eng.Ping(ctx); err != nil {
			engineLatency[name] = 0
			log.Printf("pinging %s failed: %v", name, err)
			return
		}

		dur := time.Since(then).Truncate(time.Millisecond)
		log.Printf("ping for %s took %v", name, dur)

		engineLatencyMu.Lock()
		engineLatency[name] = dur
		engineLatencyMu.Unlock()
	}

	for {
		for name, eng := range engines {
			go fn(name, eng)
		}

		select {
		case <-ticker.C:
			// This space is intentionally left blank.
		case <-ctx.Done():
			return
		}
	}
}

func getEngineLatency(name string) time.Duration {
	engineLatencyMu.RLock()
	defer engineLatencyMu.RUnlock()

	return engineLatency[name]
}
