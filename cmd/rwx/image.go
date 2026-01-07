package main

import (
	"github.com/rwx-cloud/cli/cmd/rwx/image"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	GroupID: "execution",
	Use:     "image",
	Short:   "Manage OCI images",
}

// for backcompat
var pushCmd *cobra.Command

func init() {
	image.InitPush(requireAccessToken, func() cli.Service { return service })
	image.InitBuild(requireAccessToken, ParseInitParameters, func() cli.Service { return service })
	image.InitPull(requireAccessToken, func() cli.Service { return service })
	imageCmd.AddCommand(image.PushCmd)
	imageCmd.AddCommand(image.BuildCmd)
	imageCmd.AddCommand(image.PullCmd)

	// for backcompat
	pushCmd = &cobra.Command{
		GroupID: "execution",
		Args:    image.PushCmd.Args,
		PreRunE: image.PushCmd.PreRunE,
		RunE:    image.PushCmd.RunE,
		Short:   image.PushCmd.Short,
		Use:     image.PushCmd.Use,
		Hidden:  true,
	}
	pushCmd.Flags().AddFlagSet(image.PushCmd.Flags())
}
