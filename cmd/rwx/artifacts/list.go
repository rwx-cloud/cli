package artifacts

import (
	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var ListCmd *cobra.Command

func InitList(requireAccessToken func() error, getService func() cli.Service, useJsonOutput func() bool) {
	ListCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			useJson := useJsonOutput()

			_, err := getService().ListArtifacts(cli.ListArtifactsConfig{
				TaskID: taskID,
				Json:   useJson,
			})
			return err
		},
		Short: "List artifacts for a task",
		Use:   "list <task-id> [flags]",
	}
}
