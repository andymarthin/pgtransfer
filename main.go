package main

import (
	"fmt"
	"os"

	"github.com/andymarthin/pgtransfer/cmd"
)

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("pgtransfer %s\n", cmd.Version)
		fmt.Printf("Commit: %s\n", cmd.Commit)
		fmt.Printf("Built: %s\n", cmd.Date)
		os.Exit(0)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
