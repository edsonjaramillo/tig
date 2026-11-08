package main

import "github.com/edsonjaramillo/tig/cmd"

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	cmd.Execute(Version)
}
