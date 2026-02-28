package main

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"

	"github.com/spf13/cobra"
)

var (
	ResultsWait bool

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
			} else {
				if result.RunURL != "" {
					fmt.Printf("Run URL: %s\n", result.RunURL)
				}
				if result.Completed {
					fmt.Printf("Run result status: %s\n", result.ResultStatus)
				} else {
					fmt.Printf("Run status: %s (in progress)\n", result.ResultStatus)
				}

				promptResult, err := service.GetRunPrompt(result.RunID)
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

func init() {
	resultsCmd.Flags().BoolVar(&ResultsWait, "wait", false, "wait for the run to complete")
}
