package artifacts

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/spf13/cobra"
)

var (
	downloadOutputDir   string
	downloadOutputFile  string
	downloadJSON        bool
	downloadOutput      string
	downloadAutoExtract bool
	downloadOpen        bool

	DownloadCmd *cobra.Command
)

func InitDownload(requireAccessToken func() error, getService func() cli.Service) {
	DownloadCmd = &cobra.Command{
		Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			artifactKey := args[1]

			outputDirSet := cmd.Flags().Changed("output-dir")
			outputFileSet := cmd.Flags().Changed("output-file")
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
					outputDir, err = getDefaultDownloadsDir()
					if err != nil {
						return errors.Wrap(err, "unable to determine default downloads directory")
					}
				}
				absOutputDir, err = filepath.Abs(outputDir)
				if err != nil {
					return errors.Wrapf(err, "unable to resolve absolute path for %s", outputDir)
				}
			}

			useJson := downloadOutput == "json" || downloadJSON
			err = getService().DownloadArtifact(cli.DownloadArtifactConfig{
				TaskID:                 taskID,
				ArtifactKey:            artifactKey,
				OutputDir:              absOutputDir,
				OutputFile:             absOutputFile,
				OutputDirExplicitlySet: outputDirSet,
				Json:                   useJson,
				AutoExtract:            downloadAutoExtract,
				Open:                   downloadOpen,
			})
			if err != nil {
				return err
			}

			return nil
		},
		Short: "Download an artifact from a task",
		Use:   "download <task-id> <artifact-key> [flags]",
	}

	DownloadCmd.Flags().StringVar(&downloadOutputDir, "output-dir", "", "output directory for the downloaded artifact (defaults to Downloads folder)")
	DownloadCmd.Flags().StringVar(&downloadOutputFile, "output-file", "", "output file path for the downloaded artifact")
	DownloadCmd.MarkFlagsMutuallyExclusive("output-dir", "output-file")
	DownloadCmd.Flags().BoolVar(&downloadJSON, "json", false, "output file locations as JSON")
	_ = DownloadCmd.Flags().MarkHidden("json")
	DownloadCmd.Flags().StringVar(&downloadOutput, "output", "text", "output format: text or json")
	DownloadCmd.Flags().BoolVar(&downloadAutoExtract, "auto-extract", false, "automatically extract directory tar archives")
	DownloadCmd.Flags().BoolVar(&downloadOpen, "open", false, "automatically open the downloaded file(s)")
}

func getDefaultDownloadsDir() (string, error) {
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
