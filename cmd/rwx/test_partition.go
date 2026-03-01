package main

import (
	"github.com/spf13/cobra"

	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	"github.com/rwx-cloud/cli/internal/captain/config"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/providers"
)

func configureTestPartitionCmd(parentCmd *cobra.Command, cliArgs *testCliArgs) {
	var pNodes config.PartitionNodes
	var delimiter string
	var roundRobin bool
	var trimPrefix string

	partitionCmd := &cobra.Command{
		Use: "partition [--help] [--config-file=<path>] [--delimiter=<delim>] [--sha=<sha>] --suite-id=<suite> --index=<i> " +
			"--total=<total> <args>",
		Short: "Partition a test suite using historical file timings",
		Long: "'rwx test partition' can be used to split up your test suite by test file, leveraging test file timings " +
			"recorded by rwx test.",
		Example: "" +
			"  bundle exec rspec $(rwx test partition your-project-rspec --index 0 --total 2 spec/**/*_spec.rb)\n" +
			"  bundle exec rspec $(rwx test partition your-project-rspec --index 1 --total 2 spec/**/*_spec.rb)",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := func() error {
				if err := extractTestSuiteID(&cliArgs.rootCliArgs, args); err != nil {
					return err
				}

				cfg, err := initTestConfig(cmd, *cliArgs)
				if err != nil {
					return err
				}

				provider, err := cfg.ProvidersEnv.MakeProvider()
				if err != nil {
					return captainerrors.Wrap(err, "failed to construct provider")
				}

				if pNodes.Index < 0 {
					if provider.PartitionNodes.Index < 0 {
						return captainerrors.NewConfigurationError(
							"Partition index invalid.",
							"Partition index must be 0 or greater.",
							"You can set the partition index by using the --index flag or the RWX_TEST_PARTITION_INDEX environment variable.",
						)
					}
					pNodes.Index = provider.PartitionNodes.Index
				}

				if pNodes.Total < 0 {
					if provider.PartitionNodes.Total < 1 {
						return captainerrors.NewConfigurationError(
							"Partition total invalid.",
							"Partition total must be 1 or greater.",
							"You can set the partition total by using the --total flag or the RWX_TEST_PARTITION_TOTAL environment variable.",
						)
					}
					pNodes.Total = provider.PartitionNodes.Total
				}

				return initTestServiceWithConfig(cmd, cfg, cliArgs.rootCliArgs.suiteID, func(p providers.Provider) error {
					if p.CommitSha == "" {
						return captainerrors.NewConfigurationError(
							"Missing commit SHA",
							"rwx test requires a commit SHA in order to track test runs correctly.",
							"You can specify the SHA by using the --sha flag or the RWX_TEST_SHA environment variable",
						)
					}
					return nil
				})
			}()
			if err != nil {
				return captainerrors.WithDecoration(err)
			}
			return nil
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
			err := func() error {
				args := cliArgs.rootCliArgs.positionalArgs
				captain, err := captaincli.GetService(cmd)
				if err != nil {
					return captainerrors.WithStack(err)
				}
				err = captain.Partition(cmd.Context(), captaincli.PartitionConfig{
					SuiteID:        cliArgs.rootCliArgs.suiteID,
					TestFilePaths:  args,
					PartitionNodes: pNodes,
					Delimiter:      delimiter,
					RoundRobin:     roundRobin,
					TrimPrefix:     trimPrefix,
				})
				return captainerrors.WithStack(err)
			}()
			if err != nil {
				return captainerrors.WithDecoration(err)
			}
			return nil
		},
	}

	partitionCmd.Flags().IntVar(&pNodes.Index, "index", -1, "the 0-indexed index of a particular partition")
	partitionCmd.Flags().IntVar(&pNodes.Total, "total", -1, "the total number of partitions")
	addTestShaFlag(partitionCmd, &cliArgs.genericProvider.Sha)

	defaultDelimiter := getEnvWithFallback("RWX_TEST_DELIMITER", "CAPTAIN_DELIMITER")
	if defaultDelimiter == "" {
		defaultDelimiter = " "
	}

	partitionCmd.Flags().StringVar(&delimiter, "delimiter", defaultDelimiter,
		"the delimiter used to separate partitioned files.\nIt can also be set using the env var RWX_TEST_DELIMITER.")

	partitionCmd.Flags().BoolVar(&roundRobin, "round-robin", false,
		"Whether to naively round robin tests across partitions. When false, historical test timing data will be used to"+
			" evenly balance the partitions.",
	)

	partitionCmd.Flags().StringVar(&trimPrefix, "trim-prefix", "",
		"A prefix to trim from the beginning of local test file paths when comparing them to historical timing data.",
	)

	parentCmd.AddCommand(partitionCmd)
}
