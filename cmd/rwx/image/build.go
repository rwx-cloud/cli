package image

import (
	"time"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var (
	buildInitParameters []string
	buildRwxDirectory   string
	buildMintFilePath   string
	buildNoCache        bool
	buildTargetTaskKey  string
	buildTags           []string
	buildTimeout        time.Duration

	BuildCmd *cobra.Command
)

func InitBuild(requireAccessToken func() error, parseInitParameters func([]string) (map[string]string, error), getService func() cli.Service) {
	BuildCmd = &cobra.Command{
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				buildMintFilePath = args[0]
			}

			initParams, err := parseInitParameters(buildInitParameters)
			if err != nil {
				return err
			}

			config := cli.BuildImageConfig{
				InitParameters: initParams,
				RwxDirectory:   buildRwxDirectory,
				MintFilePath:   buildMintFilePath,
				NoCache:        buildNoCache,
				TargetTaskKey:  buildTargetTaskKey,
				Tags:           buildTags,
				Timeout:        buildTimeout,
			}

			return getService().BuildImage(config)
		},
		Short: "Launch a targeted RWX run and pull its result as an OCI image",
		Use:   "build <file> --target <task-key> [flags]",
	}

	BuildCmd.Flags().StringArrayVar(&buildInitParameters, "init", []string{}, "initialization parameters for the run, available in the `init` context. Can be specified multiple times")
	BuildCmd.Flags().StringVarP(&buildMintFilePath, "file", "f", "", "an RWX config file to use for sourcing task definitions (required)")
	BuildCmd.Flags().StringVarP(&buildRwxDirectory, "dir", "d", "", "the directory your RWX configuration files are located in, typically `.rwx`. By default, the CLI traverses up until it finds a `.rwx` directory.")
	BuildCmd.Flags().BoolVar(&buildNoCache, "no-cache", false, "do not read or write to the cache")
	BuildCmd.Flags().StringVar(&buildTargetTaskKey, "target", "", "task key to build (required)")
	BuildCmd.Flags().StringArrayVar(&buildTags, "tag", []string{}, "tag the built image (can be specified multiple times)")
	BuildCmd.Flags().DurationVar(&buildTimeout, "timeout", 30*time.Minute, "timeout for waiting for the build to complete and image to pull")

	_ = BuildCmd.MarkFlagRequired("target")
}
