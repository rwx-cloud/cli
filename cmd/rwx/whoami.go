package main

import (
	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	WhoamiJson bool

	whoamiCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := service.Whoami(cli.WhoamiConfig{Json: WhoamiJson})
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
}
