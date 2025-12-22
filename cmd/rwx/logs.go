package main

import (
	"os"
	"path/filepath"
	"runtime"

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
				var err error
				outputDir, err = getDefaultLogsDir()
				if err != nil {
					return errors.Wrap(err, "unable to determine default logs directory")
				}
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
	logsCmd.Flags().StringVarP(&LogsOutput, "output", "o", "", "output directory for the downloaded log file (defaults to Downloads folder)")
}

func getDefaultLogsDir() (string, error) {
	if runtime.GOOS == "linux" {
		if xdgDownload := os.Getenv("XDG_DOWNLOAD_DIR"); xdgDownload != "" {
			return xdgDownload, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "unable to determine user home directory")
	}

	return filepath.Join(homeDir, "Downloads"), nil
}
