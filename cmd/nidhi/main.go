package main

import (
	"fmt"
	"os"
)

// Build metadata injected via ldflags at compile time.
// See Makefile LDFLAGS: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("nidhi %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	// TODO: Initialize config, git runner, BubbleTea program
	fmt.Println("nidhi -- purpose-built TUI for git stash mastery")
	fmt.Println("Run with --help for usage.")
}

func printUsage() {
	fmt.Print(`nidhi -- purpose-built TUI for git stash mastery

Usage:
  nidhi [flags]

Flags:
  -h, --help              Show this help message
  -v, --version           Show version information
      --log-level string  Log level (off, error, warn, info, debug)
      --trace-git         Log all git commands with args, exit code, duration
      --debug             Print startup timing breakdown and exit
      --no-color          Disable all colors
      --no-animation      Disable animations
      --icons string      Icon set: auto (default), nerd, ascii
  -C, --directory string  Run as if started in <path>
`)
}
