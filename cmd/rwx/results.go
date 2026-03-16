package main

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/rwx-cloud/rwx/internal/errors"
	"github.com/rwx-cloud/rwx/internal/git"

	"github.com/spf13/cobra"
)

var (
	ResultsWait     bool
	ResultsFailFast bool
	ResultsAll      bool

	resultsCmd = &cobra.Command{
		GroupID: "outputs",
		Use:     "results [run-id]",
		Short:   "Get results for a run",
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			useJson := useJsonOutput()

			var runID, branchName, repositoryName string
			if len(args) > 0 {
				runID = args[0]
			} else {
				branchName = service.GitClient.GetBranch()
				repositoryName = git.RepoNameFromOriginUrl(service.GitClient.GetOriginUrl())

				if branchName == "" || repositoryName == "" {
					return fmt.Errorf("unable to determine the current branch and repository from git; please provide a run ID")
				}

				if !useJson {
					fmt.Printf("Fetching the latest run for %s repository on branch %s...\n", repositoryName, branchName)
				}
			}

			result, err := service.GetRunStatus(cli.GetRunStatusConfig{
				RunID:          runID,
				BranchName:     branchName,
				RepositoryName: repositoryName,
				Wait:           ResultsWait,
				FailFast:       ResultsFailFast,
				Json:           useJson,
			})
			if err != nil {
				if runID == "" && errors.Is(err, api.ErrNotFound) {
					return fmt.Errorf("no run found for %s repository on branch %s", repositoryName, branchName)
				}
				return err
			}

			if result.RunID == "" {
				return fmt.Errorf("no run found for %s repository on branch %s", repositoryName, branchName)
			}

			if useJson {
				if ResultsAll {
					promptResult, err := service.GetRunPrompt(cli.GetRunPromptConfig{
						RunID: result.RunID,
						All:   true,
						Json:  true,
					})
					if err != nil {
						return err
					}
					jsonOutput := struct {
						RunID        string
						ResultStatus string
						Completed    bool
						Tasks        []taskOutput
					}{
						RunID:        result.RunID,
						ResultStatus: result.ResultStatus,
						Completed:    result.Completed,
						Tasks:        toTaskOutputs(promptResult.Tasks),
					}
					resultJson, err := json.Marshal(jsonOutput)
					if err != nil {
						return err
					}
					fmt.Println(string(resultJson))
				} else {
					jsonOutput := struct {
						RunID        string
						ResultStatus string
						Completed    bool
					}{
						RunID:        result.RunID,
						ResultStatus: result.ResultStatus,
						Completed:    result.Completed,
					}
					resultJson, err := json.Marshal(jsonOutput)
					if err != nil {
						return err
					}
					fmt.Println(string(resultJson))
				}
			} else {
				if runID == "" && result.Commit != "" {
					if head := service.GitClient.GetHead(); head != "" {
						if note := git.CommitMismatchNote(head, result.Commit); note != "" {
							fmt.Println(note)
						}
					}
				}
				if result.RunURL != "" {
					fmt.Printf("Run URL: %s\n", result.RunURL)
				}
				if result.Completed {
					fmt.Printf("Run result status: %s\n", result.ResultStatus)
				} else {
					fmt.Printf("Run status: %s (in progress)\n", result.ResultStatus)
				}

				promptResult, err := service.GetRunPrompt(cli.GetRunPromptConfig{
					RunID: result.RunID,
					All:   ResultsAll,
				})
				if err == nil {
					fmt.Printf("\n%s", promptResult.Prompt)
				}
			}

			if result.Completed && result.ResultStatus != "succeeded" {
				return HandledError
			}

			return nil
		},
	}
)

// taskOutput is used for JSON marshaling to ensure PascalCase keys,
// since api.RunPromptTask uses camelCase tags for server deserialization.
type taskOutput struct {
	Key    string
	Status string
}

func toTaskOutputs(tasks []api.RunPromptTask) []taskOutput {
	out := make([]taskOutput, len(tasks))
	for i, t := range tasks {
		out[i] = taskOutput{Key: t.Key, Status: t.Status}
	}
	return out
}

func init() {
	resultsCmd.Flags().BoolVar(&ResultsWait, "wait", false, "poll for the run to complete and report the result status")
	resultsCmd.Flags().BoolVar(&ResultsFailFast, "fail-fast", false, "stop waiting when failures are available (only has an effect when used with --wait)")
	resultsCmd.Flags().BoolVar(&ResultsAll, "all", false, "include all tasks in the run, not just failures")
}
