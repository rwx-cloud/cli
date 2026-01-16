package main

import (
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	GroupID: "results",
	Use:     "get",
	Short:   "Get the status of a resource",
}

func init() {
	getCmd.AddCommand(getRunCmd)
}
