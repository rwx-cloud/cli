package main

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	GetRunWait   bool
	GetRunJson   bool
	GetRunOutput string

	getRunCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			useJson := GetRunOutput == "json" || GetRunJson
			useLLM := GetRunOutput == "llm"

			result, err := service.GetRunStatus(cli.GetRunStatusConfig{
				RunID: runID,
				Wait:  GetRunWait,
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
			}

			if useLLM && result.Completed {
				prompt, err := service.GetRunPrompt(runID)
				if err == nil {
					fmt.Print(prompt)
				}
			}

			return nil
		},
		Short: "Get the status of a run",
		Use:   "run <run-id>",
	}
)

func init() {
	getRunCmd.Flags().BoolVar(&GetRunWait, "wait", false, "wait for the run to complete")
	getRunCmd.Flags().BoolVar(&GetRunJson, "json", false, "output json data to stdout")
	_ = getRunCmd.Flags().MarkHidden("json")
	getRunCmd.Flags().StringVar(&GetRunOutput, "output", "text", "output format: text, json, or llm")
}
