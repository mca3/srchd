package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	_ "git.sr.ht/~cmcevoy/srchd/search/engines"
	_ "net/http/pprof"
)

var (
	configPath = flag.String("conf", "", "configuration file; ./config.yaml will be used if it exists")
)

var blacklist = newBlacklist()

func main() {
	if *configPath == "" {
		// Try config.yaml
		if _, err := os.Stat("./config.yaml"); err == nil {
			*configPath = "./config.yaml"
		}
	}

	log.Printf("srchd %s", Version)

	if *configPath != "" {
		if err := loadConfig(*configPath); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		}
	}

	for _, v := range cfg.Blacklists {
		n, err := blacklist.LoadFile(v)
		if err != nil {
			// TODO: should this be fatal?
			log.Printf("failed to load blacklist %q: %v", v, err)
		} else {
			log.Printf("loaded %d rules from %s", n, v)
		}
	}

	for _, v := range enabledEngines() {
		log.Printf("initializing engine %q", v)

		eng, err := initializeEngine(v)
		if err != nil {
			panic(err)
		}
		engines[v] = eng
	}

	go pinger(context.TODO())

	if cfg.Pprof != "" {
		go func() {
			// TODO: VERY TEMPORARY
			log.Println(http.ListenAndServe(cfg.Pprof, nil))
		}()
	}

	log.Fatal(serveHTTP(context.TODO()))
}
