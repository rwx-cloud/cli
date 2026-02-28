package main

import (
	"errors"
	"fmt"
	"os"

	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/cli"
)

// A HandledError has already been handled in the called function,
// but should return a non-zero exit code.
var HandledError = cli.HandledError

func main() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	// Captain's ExecutionError carries custom exit codes from subprocess execution
	if e, ok := captainerrors.AsExecutionError(err); ok {
		os.Exit(e.Code)
	}

	if !errors.Is(err, HandledError) {
		if Debug {
			// Enabling debug output will print stacktraces
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
	}

	os.Exit(1)
}
