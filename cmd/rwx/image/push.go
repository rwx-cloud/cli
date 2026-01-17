package image

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/skratchdot/open-golang/open"

	"github.com/spf13/cobra"
)

var (
	pushImageReferences []string
	pushImageJSON       bool
	pushImageOutput     string
	pushImageNoWait     bool
	pushImageOpen       bool

	PushCmd *cobra.Command
)

func InitPush(requireAccessToken func() error, getService func() cli.Service) {
	PushCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			openURL := open.Run
			if !pushImageOpen {
				openURL = func(input string) error { return nil }
			}

			useJson := pushImageOutput == "json" || pushImageJSON
			config, err := cli.NewImagePushConfig(args[0], pushImageReferences, useJson, !pushImageNoWait, openURL)
			if err != nil {
				return err
			}

			_, err = getService().ImagePush(config)
			return err
		},
		Short: "Push an RWX task to an OCI reference",
		Use:   "push <task-id> --to <reference> [--to <reference>] [--output json] [--open] [--no-wait]",
	}

	PushCmd.Flags().StringArrayVar(&pushImageReferences, "to", []string{}, "the qualified OCI reference to push the image to (can be specified multiple times)")
	PushCmd.Flags().BoolVar(&pushImageJSON, "json", false, "output JSON instead of human-readable text")
	_ = PushCmd.Flags().MarkHidden("json")
	PushCmd.Flags().StringVar(&pushImageOutput, "output", "text", "output format: text or json")
	PushCmd.Flags().BoolVar(&pushImageNoWait, "no-wait", false, "do not wait for the push to complete")
	PushCmd.Flags().BoolVar(&pushImageOpen, "open", false, "open the run URL in the default browser once the push starts")
}
