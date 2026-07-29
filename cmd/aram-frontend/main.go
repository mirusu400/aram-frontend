//go:build (windows || linux || darwin) && !android && !ios

package main

import (
	"fmt"
	"os"
)

import "github.com/mirusu400/aram-frontend/frontend"

func main() {
	initial := ""
	if len(os.Args) > 1 {
		initial = os.Args[1]
	}
	if err := frontend.Run(frontend.NullBackend{}, initial); err != nil {
		fmt.Fprintln(os.Stderr, "aram-frontend:", err)
		os.Exit(1)
	}
}
