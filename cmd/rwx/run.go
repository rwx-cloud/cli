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
	NoCache        bool
	Open           bool
	Debug          bool
	Title          string

	runCmd = &cobra.Command{
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

			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var targetedTasks []string
			if len(args) >= 0 {
				targetedTasks = args
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
				TargetedTasks:  targetedTasks,
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
				}{
					RunId:            runResult.RunId,
					RunURL:           runResult.RunURL,
					TargetedTaskKeys: runResult.TargetedTaskKeys,
					DefinitionPath:   runResult.DefinitionPath,
				}
				runResultJson, err := json.Marshal(jsonOutput)
				if err != nil {
					return err
				}

				fmt.Println(string(runResultJson))
			} else {
				fmt.Printf("Run is watchable at %s\n", runResult.RunURL)
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
		Short: "Start a new run on Mint",
		Use:   "run [flags] [task]",
	}
)

func init() {
	runCmd.Flags().BoolVar(&NoCache, "no-cache", false, "do not read or write to the cache")
	runCmd.Flags().StringArrayVar(&InitParameters, flagInit, []string{}, "initialization parameters for the run, available in the `init` context. Can be specified multiple times")
	runCmd.Flags().StringVarP(&MintFilePath, "file", "f", "", "a Mint config file to use for sourcing task definitions (required)")
	addRwxDirFlag(runCmd)
	runCmd.Flags().BoolVar(&Open, "open", false, "open the run in a browser")
	runCmd.Flags().BoolVar(&Debug, "debug", false, "start a remote debugging session once a breakpoint is hit")
	runCmd.Flags().StringVar(&Title, "title", "", "the title the UI will display for the Mint run")
	runCmd.Flags().BoolVar(&Json, "json", false, "output json data to stdout")
}

// parseInitParameters converts a list of `key=value` pairs to a map. It also reads any `MINT_INIT_` variables from the
// environment
func ParseInitParameters(params []string) (map[string]string, error) {
	parsedParams := make(map[string]string)

	parse := func(p string) error {
		fields := strings.Split(p, "=")
		if len(fields) < 2 {
			return errors.Errorf("unable to parse %q", p)
		}

		parsedParams[fields[0]] = strings.Join(fields[1:], "=")
		return nil
	}

	const prefix = "MINT_INIT_"
	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, prefix) {
			continue
		}

		if err := parse(strings.TrimPrefix(envVar, prefix)); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	// Parse flag parameters after the environment as they take precedence
	for _, param := range params {
		if err := parse(param); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return parsedParams, nil
}
