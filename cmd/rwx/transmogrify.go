package main

import (
	"os"

	"github.com/rwx-cloud/cli/internal/cli"

	"github.com/spf13/cobra"
)

var (
	transmogrifyToPath string
)

var transmogrifyCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		transmogrifyFrom, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer transmogrifyFrom.Close()

		transmogrifyTo, err := os.Create(transmogrifyToPath)
		if err != nil {
			return err
		}
		defer transmogrifyTo.Close()

		config, err := cli.NewTransmogrifyDockerfileConfig(transmogrifyFrom, transmogrifyTo)
		if err != nil {
			return err
		}

		return service.TransmogrifyDockerfile(config)
	},
	Short:  "Transmogrifies a Dockerfile into an RWX run definition",
	Use:    "transmogrify ./path/to/Dockerfile -o .rwx/image.yml",
	Hidden: true, // for now, until official release and some testing
}

func init() {
	transmogrifyCmd.Flags().StringVarP(&transmogrifyToPath, "out", "o", "", "the file to which the RWX run definition will be written")
}
