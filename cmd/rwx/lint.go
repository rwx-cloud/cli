package main

import (
	"slices"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/spf13/cobra"
)

var (
	LintFailure = errors.Wrap(HandledError, "lint failure")

	LintRwxDirectory     string
	LintWarningsAsErrors bool
	LintOutputFormat     string

	lintCmd = &cobra.Command{
		GroupID: "definitions",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			lintConfig, err := cli.NewLintConfig(
				LintRwxDirectory,
				LintOutputFormat,
			)
			if err != nil {
				return err
			}

			lintResult, err := service.Lint(lintConfig)
			if err != nil {
				return err
			}

			if len(lintResult.Problems) == 0 {
				return nil
			}

			hasError := slices.ContainsFunc(lintResult.Problems, func(lf api.LintProblem) bool {
				return lf.Severity == "error"
			})

			if hasError || LintWarningsAsErrors {
				return LintFailure
			}

			return nil
		},
		Short: "Lint RWX configuration files",
		Use:   "lint [flags]",
	}
)

func init() {
	lintCmd.Flags().BoolVar(&LintWarningsAsErrors, "warnings-as-errors", false, "treat warnings as errors")
	lintCmd.Flags().StringVarP(&LintRwxDirectory, "dir", "d", "", "the directory your RWX configuration files are located in, typically `.rwx`. By default, the CLI traverses up until it finds a `.rwx` directory.")
	lintCmd.Flags().StringVarP(&LintOutputFormat, "output", "o", "multiline", "output format: multiline, oneline, json, none")
}
