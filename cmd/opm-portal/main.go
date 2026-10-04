// Command opm-portal is the Open Platform Model web portal.
//
// "opm-portal serve" runs local mode: the read API on a loopback address,
// reading the cluster as the user's kubeconfig. "opm-portal version" (or no
// arguments) prints the version and contacts nothing.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-platform-model/opm-portal/internal/version"
)

// Exit codes.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const usage = "usage: opm-portal [version | --version | serve [flags]]"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run executes the command for args and returns the process exit code. ctx
// is done when the process is asked to stop.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0, len(args) == 1 && (args[0] == "version" || args[0] == "--version"):
		fmt.Fprintf(stdout, "opm-portal %s\n", version.Full())
		return exitOK
	case args[0] == "serve":
		return serve(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}
}
