package main

import (
	"github.com/spf13/cobra"

	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
)

func configureTestResultsCmd(parentCmd *cobra.Command, cliArgs *testCliArgs) {
	resultsParseCmd := &cobra.Command{
		Use:     "parse [flags] <test-results-files>",
		Short:   "Parse test results files into RWX v1 JSON",
		Long:    "'rwx test results parse' will parse test results files and output RWX v1 JSON.",
		Example: `  rwx test results parse rspec.json`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: unsafeInitTestParsingOnly(cliArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := func() error {
				artifacts := cliArgs.rootCliArgs.positionalArgs

				captain, err := captaincli.GetService(cmd)
				if err != nil {
					return captainerrors.WithStack(err)
				}

				err = captain.Parse(cmd.Context(), artifacts)
				if _, ok := captainerrors.AsConfigurationError(err); !ok {
					cmd.SilenceUsage = true
				}

				return captainerrors.WithStack(err)
			}()
			if err != nil {
				return captainerrors.WithDecoration(err)
			}
			return nil
		},
	}

	addTestFrameworkFlags(resultsParseCmd, &cliArgs.frameworkParams)

	resultsCmd := &cobra.Command{
		Use:   "results",
		Short: "Manage test results",
	}

	resultsCmd.AddCommand(resultsParseCmd)
	configureTestMergeCmd(resultsCmd, cliArgs)
	parentCmd.AddCommand(resultsCmd)
}
