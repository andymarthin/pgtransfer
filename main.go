package main

import (
	"os"

	"github.com/andymarthin/pgtransfer/cmd"
)

func main() {
	// Handle version flag (mirrors the `version` subcommand output).
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		cmd.PrintVersion()
		os.Exit(0)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
