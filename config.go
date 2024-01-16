package main

import (
	"encoding/json"
	"os"

	"git.int21h.xyz/srchd/search"
)

type config struct {
	Addr    string
	Engines []string
}

var defaultConfig = config{
	Addr:    ":8080",
	Engines: search.Supported(),
}

var cfg = defaultConfig

func loadConfig(path string) error {
	h, err := os.Open(path)
	if err != nil {
		return err
	}
	defer h.Close()

	return json.NewDecoder(h).Decode(&cfg)
}
