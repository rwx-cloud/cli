package main

import (
	"os"
	"path/filepath"

	"github.com/rwx-research/mint-cli/cmd/mint/config"
	"github.com/rwx-research/mint-cli/internal/accesstoken"
	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/errors"
	"github.com/rwx-research/mint-cli/internal/ssh"

	"github.com/spf13/cobra"
)

var (
	AccessToken string
	Verbose     bool

	mintHost           string
	service            cli.Service
	accessTokenBackend accesstoken.Backend

	// rootCmd represents the main `mint` command
	rootCmd = &cobra.Command{
		Use:           "mint",
		Short:         "A CLI client from www.rwx.com/mint",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       config.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			accessTokenBackend, err = accesstoken.NewFileBackend([]string{
				filepath.Join("~", ".config", "rwx"),
				filepath.Join("~", ".mint"),
			})
			if err != nil {
				return errors.Wrap(err, "unable to initialize access token backend")
			}

			c, err := api.NewClient(api.Config{AccessToken: AccessToken, Host: mintHost, AccessTokenBackend: accessTokenBackend})
			if err != nil {
				return errors.Wrap(err, "unable to initialize API client")
			}

			service, err = cli.NewService(cli.Config{APIClient: c, SSHClient: new(ssh.Client), Stdout: os.Stdout, Stderr: os.Stderr})
			if err != nil {
				return errors.Wrap(err, "unable to initialize CLI")
			}

			return nil
		},
	}
)

func addRwxDirFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&RwxDirectory, "dir", "d", "", "the directory your Mint files are located in, typically `.mint`. By default, the CLI traverses up until it finds a `.mint` directory.")
}

func init() {
	// A different host can only be set over the environment
	mintHost = os.Getenv("MINT_HOST")
	if mintHost == "" {
		mintHost = "cloud.rwx.com"
	}

	rootCmd.PersistentFlags().StringVar(&AccessToken, "access-token", os.Getenv("RWX_ACCESS_TOKEN"), "the access token for Mint")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "enable debug output")
	_ = rootCmd.PersistentFlags().MarkHidden("verbose")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(dispatchCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(vaultsCmd)
	rootCmd.AddCommand(leavesCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(resolveCmd)
	rootCmd.AddCommand(updateCmd)
}
