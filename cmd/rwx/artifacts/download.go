package artifacts

import (
	"fmt"
	"path/filepath"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/spf13/cobra"
)

var (
	downloadOutputDir   string
	downloadOutputFile  string
	downloadAutoExtract bool
	downloadOpen        bool
	downloadAll         bool

	DownloadCmd *cobra.Command
)

func InitDownload(requireAccessToken func() error, getService func() cli.Service, useJsonOutput func() bool) {
	DownloadCmd = &cobra.Command{
		Args: func(cmd *cobra.Command, args []string) error {
			if downloadAll {
				if len(args) != 1 {
					return fmt.Errorf("accepts 1 arg(s) when --all is used, received %d", len(args))
				}
				return nil
			}
			if len(args) != 2 {
				return fmt.Errorf("accepts 2 arg(s), received %d", len(args))
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			outputDirSet := cmd.Flags().Changed("output-dir")
			outputFileSet := cmd.Flags().Changed("output-file")

			if downloadAll {
				if outputFileSet {
					return errors.New("--output-file cannot be used with --all")
				}

				var absOutputDir string
				var err error

				outputDir := downloadOutputDir
				if !outputDirSet {
					outputDir, err = cli.FindDefaultDownloadsDir()
					if err != nil {
						return errors.Wrap(err, "unable to determine default downloads directory")
					}
				}
				absOutputDir, err = filepath.Abs(outputDir)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
				}

				useJson := useJsonOutput()
				_, err = getService().DownloadAllArtifacts(cli.DownloadAllArtifactsConfig{
					TaskID:                 taskID,
					OutputDir:              absOutputDir,
					OutputDirExplicitlySet: outputDirSet,
					Json:                   useJson,
					AutoExtract:            downloadAutoExtract,
					Open:                   downloadOpen,
				})
				return err
			}

			artifactKey := args[1]

			if outputDirSet && outputFileSet {
				return errors.New("output-dir and output-file cannot be used together")
			}

			var absOutputDir string
			var absOutputFile string
			var err error

			if downloadOutputFile != "" {
				absOutputFile, err = filepath.Abs(downloadOutputFile)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", downloadOutputFile)
				}
			} else {
				outputDir := downloadOutputDir
				if !outputDirSet {
					outputDir, err = cli.FindDefaultDownloadsDir()
					if err != nil {
						return errors.Wrap(err, "unable to determine default downloads directory")
					}
				}
				absOutputDir, err = filepath.Abs(outputDir)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
				}
			}

			useJson := useJsonOutput()
			_, err = getService().DownloadArtifact(cli.DownloadArtifactConfig{
				TaskID:                 taskID,
				ArtifactKey:            artifactKey,
				OutputDir:              absOutputDir,
				OutputFile:             absOutputFile,
				OutputDirExplicitlySet: outputDirSet,
				Json:                   useJson,
				AutoExtract:            downloadAutoExtract,
				Open:                   downloadOpen,
			})
			return err
		},
		Short: "Download an artifact from a task",
		Use:   "download <task-id> [artifact-key] [flags]",
	}

	DownloadCmd.Flags().StringVar(&downloadOutputDir, "output-dir", "", "output directory for the downloaded artifact (defaults to .rwx/downloads folder)")
	DownloadCmd.Flags().StringVar(&downloadOutputFile, "output-file", "", "output file path for the downloaded artifact")
	DownloadCmd.MarkFlagsMutuallyExclusive("output-dir", "output-file")
	DownloadCmd.Flags().BoolVar(&downloadAutoExtract, "auto-extract", false, "automatically extract directory tar archives")
	DownloadCmd.Flags().BoolVar(&downloadOpen, "open", false, "automatically open the downloaded file(s)")
	DownloadCmd.Flags().BoolVar(&downloadAll, "all", false, "download all artifacts for the task")
}
