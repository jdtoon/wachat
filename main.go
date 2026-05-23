// Command wachat is a lean, native desktop WhatsApp client.
//
// The current build is a placeholder — see CLAUDE.md §12 for the roadmap.
package main

import (
	"flag"
	"fmt"
	"os"
)

// Version is the current version of wachat. Set via -ldflags at build time
// for release builds; defaults to "dev" for local builds.
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("wachat", Version)
		return
	}

	fmt.Fprintln(os.Stderr, "wachat: not yet implemented. See CLAUDE.md §12 for the roadmap.")
	fmt.Fprintln(os.Stderr, "Run with -version to see the current build version.")
	os.Exit(1)
}
