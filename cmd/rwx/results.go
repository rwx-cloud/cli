package main

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	ResultsWait bool

	resultsCmd = &cobra.Command{
		GroupID: "outputs",
		Use:     "results <run-id>",
		Short:   "Get results for a run",
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			useJson := useJsonOutput()

			result, err := service.GetRunStatus(cli.GetRunStatusConfig{
				RunID: runID,
				Wait:  ResultsWait,
				Json:  useJson,
			})
			if err != nil {
				return err
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
				if result.Completed {
					fmt.Printf("Run result status: %s\n", result.ResultStatus)
				} else {
					fmt.Printf("Run status: %s (in progress)\n", result.ResultStatus)
				}

				promptResult, err := service.GetRunPrompt(runID)
				if err == nil {
					fmt.Print(promptResult.Prompt)
				}
			}

			return nil
		},
	}
)

func init() {
	resultsCmd.Flags().BoolVar(&ResultsWait, "wait", false, "wait for the run to complete")
}
