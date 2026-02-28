package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/reporting"
)

func configureTestMergeCmd(parentCmd *cobra.Command, cliArgs *testCliArgs) {
	mergeCmd := &cobra.Command{
		Use:   "merge results-glob [results-globs] <args>",
		Short: "Merge test results files",
		Long: "'rwx test results merge' takes test results files produced by partitioned " +
			"executions and merges them into a single set of results.",
		Example: `  rwx test results merge tmp/results/*.json`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: unsafeInitTestParsingOnly(cliArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := func() error {
				captain, err := captaincli.GetService(cmd)
				if err != nil {
					return captainerrors.WithStack(err)
				}

				reporterFuncs := make(map[string]captaincli.Reporter)
				for _, r := range cliArgs.reporters {
					name, path, _ := strings.Cut(r, "=")

					switch name {
					case "rwx-v1-json":
						reporterFuncs[path] = reporting.WriteJSONSummary
					case "junit-xml":
						reporterFuncs[path] = reporting.WriteJUnitSummary
					case "markdown-summary":
						reporterFuncs[path] = reporting.WriteMarkdownSummary
					case "github-step-summary":
						stepSummaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
						if stepSummaryPath == "" {
							captain.Log.Debug(
								"Skipping configuration of the 'github-step-summary' reporter " +
									"(the 'GITHUB_STEP_SUMMARY' environment variable is not set).",
							)
							continue
						}

						reporterFuncs[stepSummaryPath] = reporting.WriteMarkdownSummary
					default:
						return captainerrors.NewConfigurationError(
							fmt.Sprintf("Unknown reporter %q", name),
							"Available reporters are 'rwx-v1-json', 'junit-xml', 'markdown-summary', and 'github-step-summary'.",
							"",
						)
					}
				}

				mergeConfig := captaincli.MergeConfig{
					ResultsGlobs: cliArgs.rootCliArgs.positionalArgs,
					PrintSummary: cliArgs.printSummary,
					Reporters:    reporterFuncs,
				}

				err = captain.Merge(cmd.Context(), mergeConfig)
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

	mergeCmd.Flags().BoolVar(&cliArgs.printSummary, "print-summary", false, "prints a summary of all tests to the console")

	mergeCmd.Flags().StringArrayVar(&cliArgs.reporters, "reporter", []string{},
		"one or more `type=output_path` pairs to enable different reporting options.\n"+
			"Available reporters are 'rwx-v1-json', 'junit-xml', 'markdown-summary', and 'github-step-summary'.",
	)

	addTestFrameworkFlags(mergeCmd, &cliArgs.frameworkParams)

	parentCmd.AddCommand(mergeCmd)
}
