package image

import (
	"time"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var (
	pullTags    []string
	pullTimeout time.Duration

	PullCmd *cobra.Command
)

func InitPull(requireAccessToken func() error, getService func() cli.Service, rejectJSONOutput func() error) {
	PullCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectJSONOutput(); err != nil {
				return err
			}
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			config := cli.PullImageConfig{
				TaskID:  taskID,
				Tags:    pullTags,
				Timeout: pullTimeout,
			}

			return getService().PullImage(config)
		},
		Short: "Pull an existing RWX task as an OCI image",
		Use:   "pull <taskId>",
	}

	PullCmd.Flags().StringArrayVar(&pullTags, "tag", []string{}, "tag the pulled image (can be specified multiple times)")
	PullCmd.Flags().DurationVar(&pullTimeout, "timeout", 10*time.Minute, "timeout for pulling the image")
}
