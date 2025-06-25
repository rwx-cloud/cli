package main

import (
	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.DebugTask(cli.DebugTaskConfig{DebugKey: args[0]})
	},
	Short: "Debug a task",
	Use:   "debug [flags] [debugKey]",
}

func init() {
}
