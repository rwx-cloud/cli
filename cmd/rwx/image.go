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

func init() {
	image.Init(requireAccessToken, service)
	imageCmd.AddCommand(image.PushCmd)
}
