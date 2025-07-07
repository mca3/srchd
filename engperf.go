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

var engineResultCount = map[string]int{}
var engineResultCountMu sync.RWMutex

var engineDroppedCount = map[string]int{}
var engineDroppedCountMu sync.RWMutex

var engineErrorCount = map[string]int{}
var engineErrorCountMu sync.RWMutex

var engineReqTotalTime = map[string]time.Duration{}
var engineReqTotalTimeMu sync.RWMutex

var engineReqCount = map[string]int{}
var engineReqCountMu sync.RWMutex

// Ping loop.
func pinger(ctx context.Context) {
	ticker := time.NewTicker(cfg.PingInterval.Duration)
	defer ticker.Stop()

	fn := func(name string, eng search.Engine) {
		then := time.Now()
		if err := eng.Ping(ctx); err != nil {
			engineLatencyMu.Lock()
			engineLatency[name] = 0
			engineLatencyMu.Unlock()

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

// Returns the total number of results an engine has returned since srchd has
// started.
func getEngineResultCount(name string) int {
	engineResultCountMu.RLock()
	defer engineResultCountMu.RUnlock()

	return engineResultCount[name]
}

// Returns the total number of dropped results an engine has returned since
// srchd has started.
func getEngineDroppedCount(name string) int {
	engineDroppedCountMu.RLock()
	defer engineDroppedCountMu.RUnlock()

	return engineDroppedCount[name]
}

// Returns the total number of errors an engine has returned since srchd has
// started.
func getEngineErrorCount(name string) int {
	engineErrorCountMu.RLock()
	defer engineErrorCountMu.RUnlock()

	return engineErrorCount[name]
}

// Returns the average amount of time a search request takes to complete for a
// specified engine.
func getEngineAverageReqTime(name string) time.Duration {
	engineReqTotalTimeMu.RLock()
	defer engineReqTotalTimeMu.RUnlock()
	engineReqCountMu.RLock()
	defer engineReqCountMu.RUnlock()

	if engineReqCount[name] == 0 {
		return 0
	}

	return (engineReqTotalTime[name] / time.Duration(engineReqCount[name])).Truncate(time.Millisecond)
}

// Increments the number of results an engine has returned since srchd has
// started.
func addEngineResultCount(name string, count int) {
	engineResultCountMu.Lock()
	defer engineResultCountMu.Unlock()

	engineResultCount[name] += count
}

// Increments the number of dropped results an engine has returned since srchd
// has started.
func addEngineDroppedCount(name string, count int) {
	engineDroppedCountMu.Lock()
	defer engineDroppedCountMu.Unlock()

	engineDroppedCount[name] += count
}

// Increments the total amount of time spent waiting for results from an engine.
func recordEngineReqTime(name string, d time.Duration) {
	engineReqTotalTimeMu.Lock()
	defer engineReqTotalTimeMu.Unlock()
	engineReqCountMu.Lock()
	defer engineReqCountMu.Unlock()

	engineReqTotalTime[name] += d
	engineReqCount[name]++
}

// Increments the number of errors an engine has returned since srchd has
// started by 1.
func incrementEngineErrorCount(name string) {
	engineErrorCountMu.Lock()
	defer engineErrorCountMu.Unlock()

	engineErrorCount[name]++
}
