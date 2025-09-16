package main

import (
	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	pushOCIReferences []string
)

var pushCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := cli.NewPushOCIImageConfig(args[0], pushOCIReferences)
		if err != nil {
			return err
		}

		return service.PushOCIImage(config)
	},
	Short: "Push an OCI image",
	Use:   "push <task-id> --to <reference> [--to <reference>]",
}

func init() {
	pushCmd.Flags().StringArrayVar(&pushOCIReferences, "to", []string{}, "the qualified OCI reference to push the image to (can be specified multiple times)")
	pushCmd.AddCommand(pushLayerCmd)
}
