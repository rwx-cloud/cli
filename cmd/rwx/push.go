package main

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/skratchdot/open-golang/open"

	"github.com/spf13/cobra"
)

var (
	pushImageReferences []string
	pushImageJSON       bool
	pushImageNoWait     bool
	pushImageOpen       bool
)

var pushCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		openURL := open.Run
		if !pushImageOpen {
			openURL = func(input string) error { return nil }
		}

		config, err := cli.NewPushImageConfig(args[0], pushImageReferences, pushImageJSON, !pushImageNoWait, openURL)
		if err != nil {
			return err
		}

		return service.PushImage(config)
	},
	Short:  "Push an OCI image",
	Use:    "push <task-id> --to <reference> [--to <reference>] [--json] [--open] [--no-wait]",
	Hidden: true, // for now, until official release and some testing
}

func init() {
	pushCmd.Flags().StringArrayVar(&pushImageReferences, "to", []string{}, "the qualified OCI reference to push the image to (can be specified multiple times)")
	pushCmd.Flags().BoolVar(&pushImageJSON, "json", false, "output JSON instead of human-readable text")
	pushCmd.Flags().BoolVar(&pushImageNoWait, "no-wait", false, "do not wait for the push to complete")
	pushCmd.Flags().BoolVar(&pushImageOpen, "open", false, "open the run URL in the default browser once the push starts")
}
