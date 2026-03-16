package main

import (
	"fmt"
	"path/filepath"

	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/rwx-cloud/rwx/internal/errors"

	"github.com/spf13/cobra"
)

var (
	LogsOutputDir   string
	LogsOutputFile  string
	LogsAutoExtract bool
	LogsOpen        bool
	LogsTaskKey     string

	logsCmd = &cobra.Command{
		GroupID: "outputs",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			taskKeySet := cmd.Flags().Changed("task")

			if taskKeySet {
				if len(args) > 1 {
					return errors.New("accepts at most 1 arg (run-id) when --task is used")
				}
			} else {
				if len(args) == 0 {
					return errors.New("a task ID or --task flag is required")
				}
				if len(args) > 1 {
					return errors.New("accepts at most 1 arg (task-id)")
				}
			}

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
					outputDir, err = cli.FindDefaultDownloadsDir()
					if err != nil {
						return errors.Wrap(err, "unable to determine default logs directory")
					}
				}
				absOutputDir, err = filepath.Abs(outputDir)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
				}
			}

			useJson := useJsonOutput()

			cfg := cli.DownloadLogsConfig{
				OutputDir:   absOutputDir,
				OutputFile:  absOutputFile,
				Json:        useJson,
				AutoExtract: LogsAutoExtract,
				Open:        LogsOpen,
			}

			if taskKeySet {
				var runID string
				var runIDExplicit bool
				if len(args) > 0 {
					runID = args[0]
					runIDExplicit = true
				} else {
					runID, err = service.ResolveRunIDFromGitContext()
					if err != nil {
						return err
					}
				}
				cfg.RunID = runID
				cfg.TaskKey = LogsTaskKey

				_, err = service.DownloadLogs(cfg)
				if err != nil {
					return handleTaskKeyError(err, runID, runIDExplicit)
				}
				return nil
			}

			cfg.TaskID = args[0]
			_, err = service.DownloadLogs(cfg)
			return err
		},
		Short: "Download logs for a task",
		Use:   "logs [task-id | run-id --task <key>] [flags]",
	}
)

func init() {
	logsCmd.Flags().StringVar(&LogsOutputDir, "output-dir", "", "output directory for the downloaded log file (defaults to .rwx/downloads folder)")
	logsCmd.Flags().StringVar(&LogsOutputFile, "output-file", "", "output file path for the downloaded log file")
	logsCmd.MarkFlagsMutuallyExclusive("output-dir", "output-file")
	logsCmd.Flags().BoolVar(&LogsAutoExtract, "auto-extract", false, "automatically extract zip archives")
	logsCmd.Flags().BoolVar(&LogsOpen, "open", false, "automatically open the downloaded file(s)")
	logsCmd.Flags().StringVar(&LogsTaskKey, "task", "", "task key (e.g., ci.checks.lint); resolves the task by key instead of ID")
}

// handleTaskKeyError formats task-key-specific errors for user display.
// For ambiguous keys, it shows matching keys. For not-found and other errors,
// it suggests using `rwx results --all` to discover available task keys.
// Sentinels are preserved so telemetry can classify the error.
func handleTaskKeyError(err error, runID string, runIDExplicit bool) error {
	var ambiguousErr *api.AmbiguousTaskKeyError
	if errors.As(err, &ambiguousErr) {
		msg := fmt.Sprintf("%s\n\nMatching keys:\n", ambiguousErr.Error())
		for _, key := range ambiguousErr.MatchingKeys {
			msg += fmt.Sprintf("  %s\n", key)
		}
		msg += "\nRetry with a fully-qualified key."
		return errors.WrapSentinel(errors.New(msg), errors.ErrAmbiguousTaskKey)
	}

	suggestion := "rwx results --all"
	if runIDExplicit {
		suggestion = fmt.Sprintf("rwx results %s --all", runID)
	}
	formatted := errors.Errorf("%s\n\nUse '%s' to see all available task keys.", err, suggestion)
	if errors.Is(err, api.ErrNotFound) {
		return errors.WrapSentinel(formatted, api.ErrNotFound)
	}
	return formatted
}
