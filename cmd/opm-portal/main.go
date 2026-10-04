// Command opm-portal is the Open Platform Model web portal.
//
// This build only reports its version: it opens no listener and contacts no
// cluster.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/open-platform-model/opm-portal/internal/version"
)

// Exit codes.
const (
	exitOK    = 0
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command for args and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0, len(args) == 1 && (args[0] == "version" || args[0] == "--version"):
		fmt.Fprintf(stdout, "opm-portal %s\n", version.Full())
		return exitOK
	default:
		fmt.Fprintln(stderr, "usage: opm-portal [version | --version]")
		return exitUsage
	}
}
