package main

import (
	"github.com/rwx-cloud/cli/cmd/rwx/artifacts"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var artifactsCmd = &cobra.Command{
	GroupID: "outputs",
	Use:     "artifacts",
	Short:   "Manage task artifacts",
}

func init() {
	artifacts.InitDownload(requireAccessToken, func() cli.Service { return service }, useJsonOutput)
	artifactsCmd.AddCommand(artifacts.DownloadCmd)
}
