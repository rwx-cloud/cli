package main

import (
	"github.com/rwx-cloud/cli/cmd/rwx/image"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:    "image",
	Short:  "Manage OCI images",
	Hidden: true,
}

// for backcompat
var pushCmd *cobra.Command

func init() {
	image.InitPush(requireAccessToken, service)
	image.InitBuild(requireAccessToken, ParseInitParameters, service)
	imageCmd.AddCommand(image.PushCmd)
	imageCmd.AddCommand(image.BuildCmd)

	// for backcompat
	pushCmd = &cobra.Command{
		Args:    image.PushCmd.Args,
		PreRunE: image.PushCmd.PreRunE,
		RunE:    image.PushCmd.RunE,
		Short:   image.PushCmd.Short,
		Use:     image.PushCmd.Use,
		Hidden:  true,
	}
	pushCmd.Flags().AddFlagSet(image.PushCmd.Flags())
}
