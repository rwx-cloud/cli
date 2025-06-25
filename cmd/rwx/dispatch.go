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

var (
	DispatchParams []string
	DispatchJson   bool
	DispatchOpen   bool
	DispatchDebug  bool
	DispatchTitle  string
	DispatchRef    string

	dispatchCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dispatchKey := args[0]

			params, err := ParseParams(DispatchParams)
			if err != nil {
				return errors.Wrap(err, "unable to parse params")
			}

			dispatchResult, err := service.InitiateDispatch(cli.InitiateDispatchConfig{
				DispatchKey: dispatchKey,
				Params:      params,
				Json:        DispatchJson,
				Title:       DispatchTitle,
				Ref:         DispatchRef,
			})
			if err != nil {
				return err
			}

			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			var runs []cli.GetDispatchRun

			for range ticker.C {
				runs, err = service.GetDispatch(cli.GetDispatchConfig{DispatchId: dispatchResult.DispatchId})
				if errors.Is(err, errors.ErrRetry) {
					continue
				}

				if err != nil {
					return err
				}

				break
			}

			if DispatchJson {
				dispatchResultJson, err := json.Marshal(runs)
				if err != nil {
					return err
				}

				fmt.Println(string(dispatchResultJson))
			} else {
				fmt.Printf("Run is watchable at %s\n", runs[0].RunUrl)
			}

			if DispatchOpen {
				if err := open.Run(runs[0].RunUrl); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
				}
			}

			if DispatchDebug {
				fmt.Println("\nWaiting for run to hit a breakpoint...")

				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for range ticker.C {
					err := service.DebugTask(cli.DebugTaskConfig{DebugKey: runs[0].RunId})
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
		Short: "Dispatch a run",
		Use:   "dispatch <dispatch-key> [flags]",
	}
)

func init() {
	dispatchCmd.Flags().StringArrayVar(&DispatchParams, "param", []string{}, "dispatch params for the run in form `key=value`, available in the `event.dispatch.params` context. Can be specified multiple times")
	dispatchCmd.Flags().StringVar(&DispatchRef, "ref", "", "the git ref to use for the run")
	dispatchCmd.Flags().BoolVar(&DispatchOpen, "open", false, "open the run in a browser")
	dispatchCmd.Flags().BoolVar(&DispatchDebug, "debug", false, "start a remote debugging session once a breakpoint is hit")
	dispatchCmd.Flags().StringVar(&DispatchTitle, "title", "", "the title the UI will display for the run")
	dispatchCmd.Flags().BoolVar(&DispatchJson, "json", false, "output json data to stdout")
	dispatchCmd.Flags().SortFlags = false
}

// ParseParams converts a list of `key=value` pairs to a map.
func ParseParams(params []string) (map[string]string, error) {
	parsedParams := make(map[string]string)

	parse := func(p string) error {
		fields := strings.Split(p, "=")
		if len(fields) < 2 {
			return errors.Errorf("unable to parse %q", p)
		}

		parsedParams[fields[0]] = strings.Join(fields[1:], "=")
		return nil
	}

	for _, param := range params {
		if err := parse(param); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return parsedParams, nil
}
