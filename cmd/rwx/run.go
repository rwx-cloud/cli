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
	Output         string
	RwxDirectory   string
	MintFilePath   string
	TargetedTasks  []string
	NoCache        bool
	Open           bool
	Debug          bool
	Wait           bool
	Title          string

	runCmd = &cobra.Command{
		GroupID: "execution",
		PreRunE: func(cmd *cobra.Command, args []string) error {
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

			useJson := Output == "json" || Json
			runResult, err := service.InitiateRun(cli.InitiateRunConfig{
				InitParameters: initParams,
				Json:           useJson,
				RwxDirectory:   RwxDirectory,
				MintFilePath:   MintFilePath,
				NoCache:        NoCache,
				TargetedTasks:  TargetedTasks,
				Title:          Title,
			})
			if err != nil {
				return err
			}

			if useJson {
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

			if Wait {
				waitResult, err := service.WaitForRun(cli.WaitForRunConfig{
					RunID: runResult.RunId,
					Json:  useJson,
				})
				if err != nil {
					return err
				}

				if useJson {
					waitResultJson, err := json.Marshal(waitResult)
					if err != nil {
						return err
					}
					fmt.Println(string(waitResultJson))
				} else {
					fmt.Printf("Run completed with status: %s\n", waitResult.Status)
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
		Short: "Start a new run",
		Use:   "run <file> [flags]",
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
	runCmd.Flags().BoolVar(&Wait, "wait", false, "wait for the run to complete and report the status")
	runCmd.Flags().StringVar(&Title, "title", "", "the title the UI will display for the run")
	runCmd.Flags().BoolVar(&Json, "json", false, "output json data to stdout")
	_ = runCmd.Flags().MarkHidden("json")
	runCmd.Flags().StringVar(&Output, "output", "text", "output format: text or json")
}
