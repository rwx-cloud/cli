package main

import (
	"github.com/rwx-cloud/cli/cmd/rwx/artifacts"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var artifactsCmd = &cobra.Command{
	GroupID: "results",
	Use:     "artifacts",
	Short:   "Manage task artifacts",
}

func init() {
	artifacts.InitDownload(requireAccessToken, func() cli.Service { return service })
	artifactsCmd.AddCommand(artifacts.DownloadCmd)
}
