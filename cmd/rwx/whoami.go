package main

import (
	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	WhoamiJson   bool
	WhoamiOutput string

	whoamiCmd = &cobra.Command{
		GroupID: "setup",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			useJson := WhoamiOutput == "json" || WhoamiJson
			err := service.Whoami(cli.WhoamiConfig{Json: useJson})
			if err != nil {
				return err
			}

			return nil

		},
		Short: "Outputs details about the access token in use",
		Use:   "whoami [flags]",
	}
)

func init() {
	whoamiCmd.Flags().BoolVar(&WhoamiJson, "json", false, "output JSON instead of a textual representation")
	_ = whoamiCmd.Flags().MarkHidden("json")
	whoamiCmd.Flags().StringVar(&WhoamiOutput, "output", "text", "output format: text or json")
}
