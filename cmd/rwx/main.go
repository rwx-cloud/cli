package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rwx-cloud/rwx/internal/cli"
	internalerrors "github.com/rwx-cloud/rwx/internal/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// A HandledError has already been handled in the called function,
// but should return a non-zero exit code.
var HandledError = cli.HandledError

func main() {
	start := time.Now()
	err := rootCmd.Execute()

	recordTelemetry(err, start)

	if err == nil {
		return
	}

	var exitErr *cli.ExitCodeError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
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

// telemetrySafeData defines which flag values and positional args are safe
// to include in telemetry, per command. Commands not listed here only send
// flag names (existing behavior).
var telemetrySafeData = map[string]struct {
	flagValues []string
	args       bool
}{
	"run":           {flagValues: []string{"no-cache", "target", "dir", "open", "debug", "wait", "fail-fast", "title"}, args: false},
	"debug":         {args: true},
	"dispatch":      {flagValues: []string{"ref", "open", "debug", "title"}, args: true},
	"lint":          {flagValues: []string{"warnings-as-errors", "dir", "output", "timeout", "fix"}, args: true},
	"results":       {flagValues: []string{"wait", "fail-fast"}, args: true},
	"logs":          {flagValues: []string{"output-dir", "output-file", "auto-extract", "open"}, args: true},
	"sandbox start": {flagValues: []string{"dir", "open", "wait"}, args: true},
	"sandbox exec":  {flagValues: []string{"dir", "open", "no-sync"}, args: false},
	"sandbox list":  {},
	"sandbox stop":  {flagValues: []string{"all"}},
	"sandbox reset": {flagValues: []string{"dir", "open", "wait"}, args: true},
	"sandbox init":  {args: true},
}

func safeTelemetryProps(commandName string, cmd *cobra.Command) map[string]any {
	safe, ok := telemetrySafeData[commandName]
	if !ok || cmd == nil {
		return nil
	}

	props := map[string]any{}

	safeSet := make(map[string]struct{}, len(safe.flagValues))
	for _, name := range safe.flagValues {
		safeSet[name] = struct{}{}
	}

	flagValues := make(map[string]string)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if _, isSafe := safeSet[f.Name]; isSafe {
			flagValues[f.Name] = f.Value.String()
		}
	})
	if len(flagValues) > 0 {
		props["flag_values"] = flagValues
	}

	if safe.args {
		if args := cmd.Flags().Args(); len(args) > 0 {
			props["args"] = args
		}
	}

	return props
}

func recordTelemetry(err error, start time.Time) {
	if telem == nil {
		return
	}

	cmd, _, _ := rootCmd.Find(os.Args[1:])

	commandName := "rwx"
	if cmd != nil {
		commandName = cmd.CommandPath()
	}
	// Normalize "rwx <sub>" to just the subcommand path (e.g. "sandbox exec")
	commandName = strings.TrimPrefix(commandName, "rwx ")

	var flagNames []string
	if cmd != nil {
		cmd.Flags().Visit(func(f *pflag.Flag) {
			flagNames = append(flagNames, f.Name)
		})
	}

	safeProps := safeTelemetryProps(commandName, cmd)

	commandProps := map[string]any{
		"command":       commandName,
		"flags":         flagNames,
		"output_format": Output,
		"duration_ms":   time.Since(start).Milliseconds(),
		"success":       err == nil,
	}
	for k, v := range safeProps {
		commandProps[k] = v
	}
	telem.Record("cli.command", commandProps)

	if err != nil {
		errorProps := map[string]any{
			"command":    commandName,
			"flags":      flagNames,
			"error_type": classifyError(err),
			"handled":    errors.Is(err, HandledError),
		}
		for k, v := range safeProps {
			errorProps[k] = v
		}
		telem.Record("cli.error", errorProps)
	}

	telem.Flush()
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, internalerrors.ErrBadRequest):
		return "bad_request"
	case errors.Is(err, internalerrors.ErrNotFound):
		return "not_found"
	case errors.Is(err, internalerrors.ErrGone):
		return "gone"
	case errors.Is(err, internalerrors.ErrSSH):
		return "ssh_failed"
	case errors.Is(err, internalerrors.ErrPatch):
		return "patch_failed"
	case errors.Is(err, internalerrors.ErrTimeout):
		return "timeout"
	case errors.Is(err, internalerrors.ErrLSP):
		return "lsp_error"
	default:
		return "unknown"
	}
}
