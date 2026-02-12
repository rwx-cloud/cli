package main

import (
	"os"
	"slices"
	"time"

	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/lsp"

	"github.com/spf13/cobra"
)

var (
	LintFailure = errors.Wrap(HandledError, "lint failure")

	LintRwxDirectory     string
	LintWarningsAsErrors bool
	LintOutputFormat     string
	LintTimeout          time.Duration
	LintFix              bool

	lintCmd = &cobra.Command{
		GroupID: "definitions",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := lsp.NewCheckConfig(
				LintRwxDirectory,
				LintOutputFormat,
				LintTimeout,
				args,
				LintFix,
			)
			if err != nil {
				return err
			}

			result, err := lsp.Check(cmd.Context(), cfg, os.Stdout)
			if err != nil {
				return err
			}

			if len(result.Diagnostics) == 0 {
				return nil
			}

			hasError := slices.ContainsFunc(result.Diagnostics, func(d lsp.CheckDiagnostic) bool {
				return d.Severity == "error"
			})

			if hasError || LintWarningsAsErrors {
				return LintFailure
			}

			return nil
		},
		Short: "Lint RWX configuration files",
		Use:   "lint [flags] [file...]",
	}
)

func init() {
	lintCmd.Flags().BoolVar(&LintWarningsAsErrors, "warnings-as-errors", false, "treat warnings as errors")
	lintCmd.Flags().StringVarP(&LintRwxDirectory, "dir", "d", "", "the directory your RWX configuration files are located in, typically `.rwx`. By default, the CLI traverses up until it finds a `.rwx` directory.")
	lintCmd.Flags().StringVarP(&LintOutputFormat, "output", "o", "multiline", "output format: text, multiline, oneline, json, none")
	lintCmd.Flags().DurationVar(&LintTimeout, "timeout", 30*time.Second, "timeout for the LSP check operation")
	lintCmd.Flags().BoolVar(&LintFix, "fix", false, "automatically apply available fixes")
}
