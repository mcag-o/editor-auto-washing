package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("content-hub server starting...")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Phase 1: placeholder
	return nil
}
