package main

import (
	"runtime/debug"
)

// Version of srchd.
var Version = "unknown"

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	Version = info.Main.Version
}
