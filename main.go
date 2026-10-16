package main

import (
	"os"

	"github.com/andymarthin/pgtransfer/cmd"
	"github.com/andymarthin/pgtransfer/internal/output"
)

func main() {
	// Handle version flag (mirrors the `version` subcommand output).
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		cmd.PrintVersion()
		os.Exit(0)
	}

	// --json must take effect before the command tree resolves the run, so
	// commands emit structured output from the very first line.
	output.Configure(hasJSONFlag(os.Args[1:]))

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// hasJSONFlag reports whether --json was passed anywhere on the command line.
// It allows the output mode to be set before cobra parses flags, which matters
// for early errors (e.g. an invalid subcommand) that should still be JSON.
func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "--json=true" {
			return true
		}
	}
	return false
}
