package main

import (
	"slices"

	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/errors"

	"github.com/spf13/cobra"
)

var (
	LintFailure = errors.Wrap(HandledError, "lint failure")

	LintRwxDirectory     string
	LintWarningsAsErrors bool
	LintOutputFormat     string

	lintCmd = &cobra.Command{
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
		Short:  "Lint Mint configuration files",
		Use:    "lint [flags]",
		Hidden: true,
	}
)

func init() {
	lintCmd.Flags().BoolVar(&LintWarningsAsErrors, "warnings-as-errors", false, "treat warnings as errors")
	lintCmd.Flags().StringVarP(&LintRwxDirectory, "dir", "d", "", "the directory your Mint files are located in, typically `.mint`. By default, the CLI traverses up until it finds a `.mint` directory.")
	lintCmd.Flags().StringVarP(&LintOutputFormat, "output", "o", "multiline", "output format: multiline, oneline, none")
}
