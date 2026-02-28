package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	"github.com/rwx-cloud/cli/internal/captain/config"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/mint"
	"github.com/rwx-cloud/cli/internal/captain/providers"
	"github.com/rwx-cloud/cli/internal/captain/reporting"
	"github.com/rwx-cloud/cli/internal/captain/runpartition"
	"github.com/rwx-cloud/cli/internal/captain/targetedretries"
	v1 "github.com/rwx-cloud/cli/internal/captain/testingschema/v1"
)

type testRootCliArgs struct {
	configFilePath  string
	debug           bool
	githubJobName   string
	githubJobMatrix string
	insecure        bool
	suiteID         string
	positionalArgs  []string
}

type testFrameworkParams struct {
	kind     string
	language string
}

type testCliArgs struct {
	command                   string
	testResults               string
	failOnUploadError         bool
	failOnDuplicateTestID     bool
	failOnMisconfiguredRetry  bool
	failRetriesFast           bool
	flakyRetries              int
	intermediateArtifactsPath string
	additionalArtifactPaths   []string
	maxTestsToRetry           string
	postRetryCommands         []string
	preRetryCommands          []string
	printSummary              bool
	quiet                     bool
	reporters                 []string
	retries                   int
	retryCommandTemplate      string
	updateStoredResults       bool
	genericProvider           providers.GenericEnv
	frameworkParams           testFrameworkParams
	rootCliArgs               testRootCliArgs
	partitionIndex            int
	partitionTotal            int
	partitionDelimiter        string
	partitionCommandTemplate  string
	partitionGlobs            []string
	partitionRoundRobin       bool
	partitionTrimPrefix       string
	quarantinedTestRetries    int
}

var testArgs = testCliArgs{}

var testCmd = &cobra.Command{
	Use:    "test",
	Short:  "Manage test suites",
	Hidden: true,
	// Override root's PersistentPreRunE which initializes Docker, SSH, Git,
	// and API clients that test commands don't need. Each test subcommand
	// handles its own initialization via PreRunE.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	configureTestRootFlags(testCmd, &testArgs)

	testRunCmd := createTestRunCmd(&testArgs)
	if err := addTestRunFlags(testRunCmd, &testArgs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testRunCmd.SetHelpTemplate(testHelpTemplate)
	testRunCmd.SetUsageTemplate(testShortUsageTemplate)
	testCmd.AddCommand(testRunCmd)

	configureTestResultsCmd(testCmd, &testArgs)
}

func configureTestRootFlags(cmd *cobra.Command, cliArgs *testCliArgs) {
	cmd.PersistentFlags().StringVar(&cliArgs.rootCliArgs.configFilePath, "config-file", "", "the config file for rwx test")

	suiteIDFromEnv := getEnvWithFallback("RWX_TEST_SUITE_ID", "CAPTAIN_SUITE_ID")
	cmd.PersistentFlags().StringVar(&cliArgs.rootCliArgs.suiteID, "suite-id", suiteIDFromEnv, "the id of the test suite")

	cmd.PersistentFlags().BoolVar(&cliArgs.rootCliArgs.debug, "test-debug", false, "enable debug output for test commands")
	_ = cmd.PersistentFlags().MarkHidden("test-debug")

	cmd.PersistentFlags().BoolVar(&cliArgs.rootCliArgs.insecure, "insecure", false, "disable TLS for the API")
	_ = cmd.PersistentFlags().MarkHidden("insecure")

	cmd.PersistentFlags().BoolVarP(&cliArgs.quiet, "quiet", "q", false, "disables most default output")
}

func extractTestSuiteID(rootArgs *testRootCliArgs, args []string) error {
	rootArgs.positionalArgs = args
	if rootArgs.suiteID != "" {
		return nil
	}

	if len(rootArgs.positionalArgs) == 0 {
		return captainerrors.NewInputError("required flag \"suite-id\" not set")
	}

	rootArgs.suiteID = rootArgs.positionalArgs[0]
	rootArgs.positionalArgs = rootArgs.positionalArgs[1:]

	return nil
}

func bindTestRootFlags(cfg testConfig, rootArgs testRootCliArgs) testConfig {
	if rootArgs.debug {
		cfg.Output.Debug = true
	}

	if rootArgs.insecure {
		cfg.Cloud.Insecure = true
	}

	return cfg
}

func addTestFrameworkFlags(command *cobra.Command, fp *testFrameworkParams) {
	formattedKnownFrameworks := make([]string, len(v1.KnownFrameworks))
	for i, framework := range v1.KnownFrameworks {
		formattedKnownFrameworks[i] = fmt.Sprintf("  --language %v --framework %v", framework.Language, framework.Kind)
	}

	command.Flags().StringVar(
		&fp.language,
		"language",
		"",
		fmt.Sprintf(
			"The programming language of the test suite (required if framework is set).\n"+
				"These can be set to anything, but rwx test has specific handling for these combinations:\n%v",
			strings.Join(formattedKnownFrameworks, "\n"),
		),
	)
	command.Flags().StringVar(
		&fp.kind,
		"framework",
		"",
		fmt.Sprintf(
			"The framework of the test suite (required if language is set).\n"+
				"These can be set to anything, but rwx test has specific handling for these combinations:\n%v",
			strings.Join(formattedKnownFrameworks, "\n"),
		),
	)
}

func bindTestFrameworkFlags(cfg testConfig, fp testFrameworkParams, suiteID string) testConfig {
	if suiteConfig, ok := cfg.TestSuites[suiteID]; ok {
		if fp.kind != "" {
			suiteConfig.Results.Framework = fp.kind
		}

		if fp.language != "" {
			suiteConfig.Results.Language = fp.language
		}

		cfg.TestSuites[suiteID] = suiteConfig
	}

	return cfg
}

func addTestShaFlag(cmd *cobra.Command, destination *string) {
	cmd.Flags().StringVar(
		destination,
		"sha",
		getEnvWithFallback("RWX_TEST_SHA", "CAPTAIN_SHA"),
		"the git commit sha hash of the commit being built",
	)
}

func addTestGenericProviderFlags(cmd *cobra.Command, destination *providers.GenericEnv) {
	cmd.Flags().StringVar(
		&destination.Branch,
		"branch",
		getEnvWithFallback("RWX_TEST_BRANCH", "CAPTAIN_BRANCH"),
		"the branch name of the commit being built\n"+
			"if using a supported CI provider, this will be automatically set\n"+
			"otherwise use this flag or set the environment variable RWX_TEST_BRANCH\n",
	)

	cmd.Flags().StringVar(
		&destination.Who,
		"who",
		getEnvWithFallback("RWX_TEST_WHO", "CAPTAIN_WHO"),
		"the person who triggered the build\n"+
			"if using a supported CI provider, this will be automatically set\n"+
			"otherwise use this flag or set the environment variable RWX_TEST_WHO\n",
	)

	addTestShaFlag(cmd, &destination.Sha)

	cmd.Flags().StringVar(
		&destination.CommitMessage,
		"commit-message",
		getEnvWithFallback("RWX_TEST_COMMIT_MESSAGE", "CAPTAIN_COMMIT_MESSAGE"),
		"the git commit message of the commit being built\n"+
			"if using a supported CI provider, this will be automatically set\n"+
			"otherwise use this flag or set the environment variable RWX_TEST_COMMIT_MESSAGE\n",
	)

	cmd.Flags().StringVar(
		&destination.BuildURL,
		"build-url",
		getEnvWithFallback("RWX_TEST_BUILD_URL", "CAPTAIN_BUILD_URL"),
		"the URL of the build results\n"+
			"if using a supported CI provider, this will be automatically set\n"+
			"otherwise use this flag or set the environment variable RWX_TEST_BUILD_URL\n",
	)

	cmd.Flags().StringVar(
		&destination.Title,
		"title",
		getEnvWithFallback("RWX_TEST_TITLE", "CAPTAIN_TITLE"),
		"a descriptive title for the test suite run, such as the commit message or build message\n"+
			"if using a supported CI provider, this will be automatically set\n"+
			"otherwise use this flag or set the environment variable RWX_TEST_TITLE\n",
	)
}

func createTestRunCmd(cliArgs *testCliArgs) *cobra.Command {
	return &cobra.Command{
		Use:   "run [flags] --suite-id=<suite> <args>",
		Short: "Execute a test suite",
		Long:  "'rwx test run' can be used to execute a test suite and optionally upload the resulting artifacts.",
		Example: `  rwx test run --suite-id="your-project-rake" -c "bundle exec rake"` + "\n" +
			`  rwx test run --suite-id="your-project-jest" --test-results "jest-result.json" -c jest`,
		PreRunE: initTestService(cliArgs, providers.Validate),
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := func() error {
				args := cliArgs.rootCliArgs.positionalArgs

				reporterFuncs := make(map[string]captaincli.Reporter)

				cfg, err := getTestConfig(cmd)
				if err != nil {
					return captainerrors.WithStack(err)
				}

				captain, err := captaincli.GetService(cmd)
				if err != nil {
					return captainerrors.WithStack(err)
				}

				var runConfig captaincli.RunConfig
				if suiteConfig, ok := cfg.TestSuites[cliArgs.rootCliArgs.suiteID]; ok {
					for name, path := range suiteConfig.Output.Reporters {
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

					rwxTestResultsDir := os.Getenv("RWX_TEST_RESULTS")
					if rwxTestResultsDir != "" {
						if _, err := os.Stat(rwxTestResultsDir); err != nil {
							captain.Log.Warnf("RWX_TEST_RESULTS directory does not exist: %s", rwxTestResultsDir)
						} else {
							rwxOutputPath := filepath.Join(rwxTestResultsDir, cliArgs.rootCliArgs.suiteID+".json")

							rwxAbsPath, err := filepath.Abs(rwxOutputPath)
							if err != nil {
								captain.Log.Warnf("Unable to resolve absolute path for RWX_TEST_RESULTS output: %s", err.Error())
							} else {
								isDuplicate := false
								for existingPath := range reporterFuncs {
									existingAbsPath, err := filepath.Abs(existingPath)
									if err == nil && existingAbsPath == rwxAbsPath {
										isDuplicate = true
										break
									}
								}

								if !isDuplicate {
									reporterFuncs[rwxOutputPath] = reporting.WriteJSONSummary
								}
							}
						}
					}

					partitionIndex := cliArgs.partitionIndex
					partitionTotal := cliArgs.partitionTotal
					provider, err := cfg.ProvidersEnv.MakeProvider()
					if err != nil {
						return captainerrors.Wrap(err, "failed to construct provider")
					}

					if partitionIndex < 0 {
						partitionIndex = provider.PartitionNodes.Index
					}

					if partitionTotal < 0 {
						partitionTotal = provider.PartitionNodes.Total
					}

					if suiteConfig.Retries.MaxTests == "" && suiteConfig.Retries.MaxTestsLegacyName != "" {
						suiteConfig.Retries.MaxTests = suiteConfig.Retries.MaxTestsLegacyName
					}

					runConfig = captaincli.RunConfig{
						Args:                      args,
						CloudOrganizationSlug:     "deep_link",
						Command:                   suiteConfig.Command,
						FailOnUploadError:         suiteConfig.FailOnUploadError,
						FailOnMisconfiguredRetry:  suiteConfig.Retries.FailOnMisconfiguration,
						FailRetriesFast:           suiteConfig.Retries.FailFast,
						FlakyRetries:              suiteConfig.Retries.FlakyAttempts,
						IntermediateArtifactsPath: suiteConfig.Retries.IntermediateArtifactsPath,
						AdditionalArtifactPaths:   suiteConfig.Retries.AdditionalArtifactPaths,
						MaxTestsToRetry:           suiteConfig.Retries.MaxTests,
						PostRetryCommands:         suiteConfig.Retries.PostRetryCommands,
						PreRetryCommands:          suiteConfig.Retries.PreRetryCommands,
						PrintSummary:              suiteConfig.Output.PrintSummary,
						Quiet:                     suiteConfig.Output.Quiet,
						Reporters:                 reporterFuncs,
						Retries:                   suiteConfig.Retries.Attempts,
						RetryCommandTemplate:      suiteConfig.Retries.Command,
						QuarantinedTestRetries:    suiteConfig.Retries.QuarantinedAttempts,
						SubstitutionsByFramework:  targetedretries.SubstitutionsByFramework,
						SuiteID:                   cliArgs.rootCliArgs.suiteID,
						TestResultsFileGlob:       os.ExpandEnv(suiteConfig.Results.Path),
						UpdateStoredResults:       cliArgs.updateStoredResults,
						UploadResults:             true,
						PartitionCommandTemplate:  suiteConfig.Partition.Command,
						PartitionConfig: captaincli.PartitionConfig{
							SuiteID:       cliArgs.rootCliArgs.suiteID,
							TestFilePaths: suiteConfig.Partition.Globs,
							PartitionNodes: config.PartitionNodes{
								Index: partitionIndex,
								Total: partitionTotal,
							},
							Delimiter:  suiteConfig.Partition.Delimiter,
							RoundRobin: suiteConfig.Partition.RoundRobin,
							TrimPrefix: suiteConfig.Partition.TrimPrefix,
						},
						PartitionRoundRobin:         suiteConfig.Partition.RoundRobin,
						PartitionTrimPrefix:         suiteConfig.Partition.TrimPrefix,
						WriteRetryFailedTestsAction: mint.IsMint(),
						DidRetryFailedTestsInMint:   mint.DidRetryFailedTests(),
					}
				}

				err = captain.RunSuite(cmd.Context(), runConfig)
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
}

func addTestRunFlags(runCmd *cobra.Command, cliArgs *testCliArgs) error {
	runCmd.Flags().StringVarP(&cliArgs.command, "command", "c", "", "the command to run")
	runCmd.Flags().StringVar(&cliArgs.testResults, "test-results", "", "a filepath to a test result - supports globs for multiple result files")
	runCmd.Flags().BoolVar(&cliArgs.failOnUploadError, "fail-on-upload-error", false, "return a non-zero exit code in case the test results upload fails")
	runCmd.Flags().BoolVar(&cliArgs.failOnDuplicateTestID, "fail-on-duplicate-test-id", false, "return a non-zero exit code in case the identifiers in test results are not unique")
	runCmd.Flags().BoolVar(&cliArgs.failOnMisconfiguredRetry, "fail-on-misconfigured-retry", false, "return a non-zero exit code in case the retry command isn't producing the expect test result output")
	runCmd.Flags().StringVar(&cliArgs.intermediateArtifactsPath, "intermediate-artifacts-path", "", "the path to store intermediate artifacts under. Intermediate artifacts will be removed if not set.")
	runCmd.Flags().StringArrayVar(&cliArgs.additionalArtifactPaths, "additional-artifact-paths", []string{}, "additional artifact paths (globs) to preserve across retries. Requires --intermediate-artifacts-path to be set.")
	runCmd.Flags().StringArrayVar(&cliArgs.postRetryCommands, "post-retry", []string{}, "commands to run immediately after rwx test retries a test")
	runCmd.Flags().StringArrayVar(&cliArgs.preRetryCommands, "pre-retry", []string{}, "commands to run immediately before rwx test retries a test")
	runCmd.Flags().BoolVar(&cliArgs.printSummary, "print-summary", false, "prints a summary of all tests to the console")
	runCmd.Flags().StringArrayVar(&cliArgs.reporters, "reporter", []string{}, "one or more `type=output_path` pairs to enable different reporting options.\nAvailable reporters are 'rwx-v1-json', 'junit-xml', 'markdown-summary', and 'github-step-summary'.")
	runCmd.Flags().IntVar(&cliArgs.retries, "retries", -1, "the number of times failed tests should be retried (e.g. --retries 2 would mean a maximum of 3 attempts of any given test)")
	runCmd.Flags().IntVar(&cliArgs.flakyRetries, "flaky-retries", -1, "the number of times failing flaky tests should be retried (takes precedence over --retries if the test is known to be flaky)")
	runCmd.Flags().StringVar(&cliArgs.maxTestsToRetry, "max-tests-to-retry", "", "if set, retries will not be run when there are more than N tests to retry or if more than N%% of all tests need retried")
	runCmd.Flags().BoolVar(&cliArgs.failRetriesFast, "fail-retries-fast", false, "if set, your test suite will fail as quickly as we know it will fail")
	runCmd.Flags().IntVar(&cliArgs.quarantinedTestRetries, "quarantined-test-retries", -1, "number of retries for quarantined tests, similar to --flaky-retries. Set to 0 to disable retrying quarantined tests")
	runCmd.Flags().IntVar(&cliArgs.partitionIndex, "partition-index", -1, "The 0-indexed index of a particular partition")
	runCmd.Flags().IntVar(&cliArgs.partitionTotal, "partition-total", -1, "The desired number of partitions. Any empty partitions will result in a noop.")
	runCmd.Flags().StringVar(&cliArgs.partitionDelimiter, "partition-delimiter", " ", "The delimiter used to separate partitioned files.")
	runCmd.Flags().StringArrayVar(&cliArgs.partitionGlobs, "partition-globs", []string{}, "Filepath globs used to identify the test files you wish to partition")
	runCmd.Flags().StringVar(&cliArgs.partitionCommandTemplate, "partition-command", "",
		fmt.Sprintf(
			"The command that will be run to execute a subset of your tests while partitioning\n"+
				"(required if --partition-index or --partition-total is passed)\n"+
				"Examples:\n  Custom: --partition-command \"%v\"",
			runpartition.DelimiterSubstitution{}.Example(),
		),
	)
	runCmd.Flags().BoolVar(&cliArgs.partitionRoundRobin, "partition-round-robin", false, "Whether to naively round robin tests across partitions. When false, historical test timing data will be used to evenly balance the partitions.")
	runCmd.Flags().StringVar(&cliArgs.partitionTrimPrefix, "partition-trim-prefix", "", "A prefix to trim from the beginning of local test file paths when comparing them to historical timing data.")

	runCmd.Flags().StringVar(&cliArgs.rootCliArgs.githubJobName, "github-job-name", "", "the name of the current Github Job")
	if err := runCmd.Flags().MarkDeprecated("github-job-name", "the value will be ignored"); err != nil {
		return captainerrors.WithStack(err)
	}
	runCmd.Flags().StringVar(&cliArgs.rootCliArgs.githubJobMatrix, "github-job-matrix", "", "the JSON encoded job-matrix from Github")
	if err := runCmd.Flags().MarkDeprecated("github-job-matrix", "the value will be ignored"); err != nil {
		return captainerrors.WithStack(err)
	}

	formattedSubstitutionExamples := make([]string, len(targetedretries.SubstitutionsByFramework))
	i := 0
	for framework, substitution := range targetedretries.SubstitutionsByFramework {
		formattedSubstitutionExamples[i] = fmt.Sprintf("  %v: --retry-command \"%v\"", framework, substitution.Example())
		i++
	}
	sort.SliceStable(formattedSubstitutionExamples, func(i, j int) bool {
		return strings.ToLower(formattedSubstitutionExamples[i]) < strings.ToLower(formattedSubstitutionExamples[j])
	})

	runCmd.Flags().StringVar(
		&cliArgs.retryCommandTemplate,
		"retry-command",
		"",
		fmt.Sprintf(
			"the command that will be run to execute a subset of your tests while retrying "+
				"(required if --retries or --flaky-retries is passed)\n"+
				"Examples:\n  Custom: --retry-command \"%v\"\n%v",
			targetedretries.JSONSubstitution{}.Example(),
			strings.Join(formattedSubstitutionExamples, "\n"),
		),
	)

	runCmd.Flags().BoolVar(&cliArgs.updateStoredResults, "update-stored-results", false,
		"if set, rwx test will update its internal storage files under '.rwx/test' with the latest test results, "+
			"such as flaky tests and test timings.",
	)

	addTestGenericProviderFlags(runCmd, &cliArgs.genericProvider)
	addTestFrameworkFlags(runCmd, &cliArgs.frameworkParams)
	return nil
}

func bindTestRunFlags(cfg testConfig, cliArgs testCliArgs, cmd *cobra.Command) testConfig {
	if suiteConfig, ok := cfg.TestSuites[cliArgs.rootCliArgs.suiteID]; ok {
		if cliArgs.command != "" {
			suiteConfig.Command = cliArgs.command
		}
		if cliArgs.failOnUploadError {
			suiteConfig.FailOnUploadError = true
		}
		if cliArgs.failOnDuplicateTestID {
			suiteConfig.FailOnDuplicateTestID = true
		}
		if cliArgs.failOnMisconfiguredRetry {
			suiteConfig.Retries.FailOnMisconfiguration = true
		}
		if cliArgs.testResults != "" {
			suiteConfig.Results.Path = cliArgs.testResults
		}
		if len(cliArgs.postRetryCommands) != 0 {
			suiteConfig.Retries.PostRetryCommands = cliArgs.postRetryCommands
		}
		if len(cliArgs.preRetryCommands) != 0 {
			suiteConfig.Retries.PreRetryCommands = cliArgs.preRetryCommands
		}
		if cliArgs.failRetriesFast {
			suiteConfig.Retries.FailFast = true
		}
		if suiteConfig.Retries.QuarantinedAttempts == 0 || cliArgs.quarantinedTestRetries != -1 {
			suiteConfig.Retries.QuarantinedAttempts = cliArgs.quarantinedTestRetries
		}
		if suiteConfig.Retries.FlakyAttempts == 0 || cliArgs.flakyRetries != -1 {
			suiteConfig.Retries.FlakyAttempts = cliArgs.flakyRetries
		}
		if cliArgs.maxTestsToRetry != "" {
			suiteConfig.Retries.MaxTests = cliArgs.maxTestsToRetry
		}
		if cliArgs.printSummary {
			suiteConfig.Output.PrintSummary = true
		}
		if cliArgs.quiet {
			suiteConfig.Output.Quiet = true
		}
		if len(cliArgs.reporters) > 0 {
			reporterConfig := suiteConfig.Output.Reporters
			if reporterConfig == nil {
				reporterConfig = make(map[string]string)
			}
			for _, r := range cliArgs.reporters {
				name, path, _ := strings.Cut(r, "=")
				reporterConfig[name] = path
			}
			suiteConfig.Output.Reporters = reporterConfig
		}
		if suiteConfig.Retries.Attempts == 0 || cliArgs.retries != -1 {
			suiteConfig.Retries.Attempts = cliArgs.retries
		}
		if cliArgs.retryCommandTemplate != "" {
			suiteConfig.Retries.Command = cliArgs.retryCommandTemplate
		}
		if cliArgs.intermediateArtifactsPath != "" {
			suiteConfig.Retries.IntermediateArtifactsPath = cliArgs.intermediateArtifactsPath
		}
		if len(cliArgs.additionalArtifactPaths) > 0 {
			suiteConfig.Retries.AdditionalArtifactPaths = cliArgs.additionalArtifactPaths
		}
		if suiteConfig.Partition.Delimiter == "" {
			suiteConfig.Partition.Delimiter = cliArgs.partitionDelimiter
		}
		if cliArgs.partitionCommandTemplate != "" {
			suiteConfig.Partition.Command = cliArgs.partitionCommandTemplate
		}
		if len(cliArgs.partitionGlobs) != 0 {
			suiteConfig.Partition.Globs = cliArgs.partitionGlobs
		}
		if cmd.Flags().Changed("partition-round-robin") {
			suiteConfig.Partition.RoundRobin = cliArgs.partitionRoundRobin
		}
		if cmd.Flags().Changed("partition-trim-prefix") {
			suiteConfig.Partition.TrimPrefix = cliArgs.partitionTrimPrefix
		}

		cfg.TestSuites[cliArgs.rootCliArgs.suiteID] = suiteConfig
		cfg.ProvidersEnv.Generic = providers.MergeGeneric(cfg.ProvidersEnv.Generic, cliArgs.genericProvider)
	}

	return cfg
}

// Help templates for the run subcommand
const testHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}{{end}}

Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

const testShortUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}

Run "{{.CommandPath}} --help" to see all available flags for this command.
`
