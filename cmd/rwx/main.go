package main

import (
	"errors"
	"fmt"
	"os"

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
