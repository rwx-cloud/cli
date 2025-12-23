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
	LogsOutputDir   string
	LogsOutputFile  string
	LogsJson        bool
	LogsAutoExtract bool
	LogsOpen        bool

	logsCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("task ID is required")
			}

			taskId := args[0]

			outputDirSet := cmd.Flags().Changed("output-dir")
			outputFileSet := cmd.Flags().Changed("output-file")
			if outputDirSet && outputFileSet {
				return errors.New("output-dir and output-file cannot be used together")
			}

			var absOutputDir string
			var absOutputFile string
			var err error

			if LogsOutputFile != "" {
				absOutputFile, err = filepath.Abs(LogsOutputFile)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", LogsOutputFile)
				}
			} else {
				outputDir := LogsOutputDir
				if !outputDirSet {
					outputDir, err = getDefaultLogsDir()
					if err != nil {
						return errors.Wrap(err, "unable to determine default logs directory")
					}
				}
				absOutputDir, err = filepath.Abs(outputDir)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
				}
			}

			err = service.DownloadLogs(cli.DownloadLogsConfig{
				TaskID:      taskId,
				OutputDir:   absOutputDir,
				OutputFile:  absOutputFile,
				Json:        LogsJson,
				AutoExtract: LogsAutoExtract,
				Open:        LogsOpen,
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
	logsCmd.Flags().StringVar(&LogsOutputDir, "output-dir", "", "output directory for the downloaded log file (defaults to Downloads folder)")
	logsCmd.Flags().StringVar(&LogsOutputFile, "output-file", "", "output file path for the downloaded log file")
	logsCmd.MarkFlagsMutuallyExclusive("output-dir", "output-file")
	logsCmd.Flags().BoolVar(&LogsJson, "json", false, "output result as JSON")
	logsCmd.Flags().BoolVar(&LogsAutoExtract, "auto-extract", false, "automatically extract zip archives")
	logsCmd.Flags().BoolVar(&LogsOpen, "open", false, "automatically open the downloaded file(s)")
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
