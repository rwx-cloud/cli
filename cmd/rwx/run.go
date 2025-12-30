package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

const flagInit = "init"

var (
	InitParameters []string
	Json           bool
	RwxDirectory   string
	MintFilePath   string
	TargetedTasks  []string
	NoCache        bool
	Open           bool
	Debug          bool
	Title          string

	// Run list filter flags
	ListRepositoryNames    []string
	ListBranchNames        []string
	ListTagNames           []string
	ListAuthors            []string
	ListCommitShas         []string
	ListDefinitionPaths    []string
	ListTriggers           []string
	ListTargetedTaskKeys   []string
	ListResultStatuses     []string
	ListExecutionStatuses  []string
	ListMergeRequestLabels []string
	ListStartDate          string
	ListMyRuns             bool
	ListJson               bool

	runCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Only validate args when creating a run (no subcommand matched)
			// Skip validation if the first arg is a subcommand
			if cmd.Name() == "run" && len(args) > 0 {
				firstArg := args[0]
				isSubcommand := false
				for _, subCmd := range cmd.Commands() {
					if subCmd.Name() == firstArg {
						isSubcommand = true
						break
					}
				}
				if !isSubcommand {
					for _, arg := range args {
						if strings.Contains(arg, "=") {
							initParam := strings.Split(arg, "=")[0]
							return fmt.Errorf(
								"You have specified a task target with an equals sign: \"%s\".\n"+
									"Are you trying to specify an init parameter \"%s\"?\n"+
									"You can define multiple init parameters by specifying --%s multiple times.\n"+
									"You may have meant to specify --%s \"%s\".",
								arg,
								initParam,
								flagInit,
								flagInit,
								arg,
							)
						}
					}

					fileFlag := cmd.Flags().Lookup("file")
					if (len(args) > 0 && fileFlag.Changed) || len(args) > 1 {
						return fmt.Errorf(
							"positional arguments are not supported for task targeting.\n" +
								"Use --target to specify task targets instead.\n" +
								"For example: rwx run <file> --target <task>",
						)
					}
				}
			}

			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				MintFilePath = args[0]
			}

			initParams, err := ParseInitParameters(InitParameters)
			if err != nil {
				return errors.Wrap(err, "unable to parse init parameters")
			}

			runResult, err := service.InitiateRun(cli.InitiateRunConfig{
				InitParameters: initParams,
				Json:           Json,
				RwxDirectory:   RwxDirectory,
				MintFilePath:   MintFilePath,
				NoCache:        NoCache,
				TargetedTasks:  TargetedTasks,
				Title:          Title,
			})
			if err != nil {
				return err
			}

			if Json {
				jsonOutput := struct {
					RunId            string
					RunURL           string
					TargetedTaskKeys []string
					DefinitionPath   string
					Message          string
				}{
					RunId:            runResult.RunId,
					RunURL:           runResult.RunURL,
					TargetedTaskKeys: runResult.TargetedTaskKeys,
					DefinitionPath:   runResult.DefinitionPath,
					Message:          runResult.Message,
				}
				runResultJson, err := json.Marshal(jsonOutput)
				if err != nil {
					return err
				}

				fmt.Println(string(runResultJson))
			} else {
				fmt.Print(runResult.Message)
			}

			if Open {
				if err := open.Run(runResult.RunURL); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
				}
			}

			if Debug {
				fmt.Println("\nWaiting for run to hit a breakpoint...")

				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for range ticker.C {
					err := service.DebugTask(cli.DebugTaskConfig{DebugKey: runResult.RunId})
					if errors.Is(err, errors.ErrRetry) {
						continue
					}
					if errors.Is(err, errors.ErrGone) {
						fmt.Println("Run finished without encountering a breakpoint.")
						break
					}

					return err
				}
			}

			return nil

		},
		Short: "Manage runs",
		Use:   "run <file> [flags]",
	}

	runListCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := service.ListRuns(cli.ListRunsConfig{
				RepositoryNames:    ListRepositoryNames,
				BranchNames:        ListBranchNames,
				TagNames:           ListTagNames,
				Authors:            ListAuthors,
				CommitShas:         ListCommitShas,
				DefinitionPaths:    ListDefinitionPaths,
				Triggers:           ListTriggers,
				TargetedTaskKeys:   ListTargetedTaskKeys,
				ResultStatuses:     ListResultStatuses,
				ExecutionStatuses:  ListExecutionStatuses,
				MergeRequestLabels: ListMergeRequestLabels,
				StartDate:          ListStartDate,
				MyRuns:             ListMyRuns,
				Json:               ListJson,
			})
			if err != nil {
				return err
			}

			if ListJson {
				jsonOutput, err := json.Marshal(result)
				if err != nil {
					return errors.Wrap(err, "unable to marshal JSON")
				}
				fmt.Println(string(jsonOutput))
			} else {
				if len(result.Runs) == 0 {
					fmt.Println("No runs found")
					return nil
				}
				for _, run := range result.Runs {
					runId := run.ID
					title := "N/A"
					if run.Title != nil {
						title = *run.Title
					}
					status := "N/A"
					if run.ResultStatus != nil {
						status = *run.ResultStatus
					}
					fmt.Printf("%s\t%s\t%s\n", runId, status, title)
				}
			}

			return nil
		},
		Short: "List runs",
		Use:   "list [flags]",
	}

	runViewCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runId := args[0]
			fmt.Printf("View run %s (placeholder)\n", runId)
			return nil
		},
		Short: "View a run",
		Use:   "view <runId> [flags]",
	}
)

func init() {
	runCmd.Flags().BoolVar(&NoCache, "no-cache", false, "do not read or write to the cache")
	runCmd.Flags().StringArrayVar(&InitParameters, flagInit, []string{}, "initialization parameters for the run, available in the `init` context. Can be specified multiple times")
	runCmd.Flags().StringArrayVar(&TargetedTasks, "target", []string{}, "task to target for execution. Can be specified multiple times")
	runCmd.Flags().StringVarP(&MintFilePath, "file", "f", "", "an RWX config file to use for sourcing task definitions (required)")
	_ = runCmd.Flags().MarkHidden("file")
	addRwxDirFlag(runCmd)
	runCmd.Flags().BoolVar(&Open, "open", false, "open the run in a browser")
	runCmd.Flags().BoolVar(&Debug, "debug", false, "start a remote debugging session once a breakpoint is hit")
	runCmd.Flags().StringVar(&Title, "title", "", "the title the UI will display for the run")
	runCmd.Flags().BoolVar(&Json, "json", false, "output json data to stdout")

	runCmd.AddCommand(runListCmd)
	runCmd.AddCommand(runViewCmd)

	// Run list flags
	runListCmd.Flags().StringArrayVar(&ListRepositoryNames, "repository", []string{}, "filter by repository name (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListBranchNames, "branch", []string{}, "filter by branch name (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListTagNames, "tag", []string{}, "filter by tag name (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListAuthors, "author", []string{}, "filter by author (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListCommitShas, "commit-sha", []string{}, "filter by commit SHA (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListDefinitionPaths, "definition-path", []string{}, "filter by definition path (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListTriggers, "trigger", []string{}, "filter by trigger (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListTargetedTaskKeys, "targeted-task-key", []string{}, "filter by targeted task key (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListResultStatuses, "result-status", []string{}, "filter by result status (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListExecutionStatuses, "execution-status", []string{}, "filter by execution status (can be specified multiple times)")
	runListCmd.Flags().StringArrayVar(&ListMergeRequestLabels, "merge-request-label", []string{}, "filter by merge request label (can be specified multiple times)")
	runListCmd.Flags().StringVar(&ListStartDate, "start-date", "", "filter by start date (YYYY-MM-DD format)")
	runListCmd.Flags().BoolVar(&ListMyRuns, "my-runs", false, "filter to show only runs by the current user")
	runListCmd.Flags().BoolVar(&ListJson, "json", false, "output json data to stdout")
}
