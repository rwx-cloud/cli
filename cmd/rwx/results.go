package main

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	ResultsWait   bool
	ResultsJson   bool
	ResultsOutput string

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
			useJson := ResultsOutput == "json" || ResultsJson

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
					RunID        string `json:"run_id"`
					ResultStatus string `json:"result_status"`
					Completed    bool   `json:"completed"`
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
	resultsCmd.Flags().BoolVar(&ResultsJson, "json", false, "output json data to stdout")
	_ = resultsCmd.Flags().MarkHidden("json")
	resultsCmd.Flags().StringVar(&ResultsOutput, "output", "text", "output format: text or json")
}
