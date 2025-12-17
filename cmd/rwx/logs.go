package main

import (
	"os"
	"path/filepath"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/spf13/cobra"
)

var (
	LogsOutput string

	logsCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("task ID is required")
			}

			taskId := args[0]

			outputDir := LogsOutput
			if outputDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "unable to determine current working directory")
				}
				outputDir = filepath.Join(cwd, ".rwx", "logs")
			}

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return errors.Wrapf(err, "unable to create output directory %s", outputDir)
			}

			absOutputDir, err := filepath.Abs(outputDir)
			if err != nil {
				return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
			}

			err = service.DownloadLogs(cli.DownloadLogsConfig{
				TaskID:    taskId,
				OutputDir: absOutputDir,
			})
			if err != nil {
				return err
			}

			return nil
		},
		Short: "Download logs for a task",
		Use:   "logs <taskId> [flags]",
	}
)

func init() {
	logsCmd.Flags().StringVarP(&LogsOutput, "output", "o", "", "output directory for the downloaded log file (defaults to .rwx/logs/)")
}
