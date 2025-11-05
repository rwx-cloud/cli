package main

import (
	"os"
	"path/filepath"

	"github.com/rwx-cloud/cli/cmd/rwx/config"
	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/rwx-cloud/cli/internal/ssh"
	"golang.org/x/term"

	"github.com/spf13/cobra"
)

var (
	AccessToken string
	Verbose     bool

	rwxHost            string
	service            cli.Service
	accessTokenBackend accesstoken.Backend

	// rootCmd represents the main `rwx` command
	rootCmd = &cobra.Command{
		Use:           "rwx",
		Short:         "A CLI client from www.rwx.com",
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

			c, err := api.NewClient(api.Config{AccessToken: AccessToken, Host: rwxHost, AccessTokenBackend: accessTokenBackend})
			if err != nil {
				return errors.Wrap(err, "unable to initialize API client")
			}

			dir, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "unable to initialize CLI")
			}

			service, err = cli.NewService(cli.Config{
				APIClient: c,
				SSHClient: new(ssh.Client),
				GitClient: &git.Client{
					Binary: "git",
					Dir:    dir,
				},
				Stdout:      os.Stdout,
				StdoutIsTTY: term.IsTerminal(int(os.Stdout.Fd())),
				Stderr:      os.Stderr,
				StderrIsTTY: term.IsTerminal(int(os.Stderr.Fd())),
			})
			if err != nil {
				return errors.Wrap(err, "unable to initialize CLI")
			}

			return nil
		},
	}
)

func addRwxDirFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&RwxDirectory, "dir", "d", "", "the directory your RWX configuration files are located in, typically `.rwx`. By default, the CLI traverses up until it finds a `.rwx` directory.")
}

func init() {
	// A different host can only be set over the environment
	mintHostEnv := os.Getenv("MINT_HOST")
	rwxHostEnv := os.Getenv("RWX_HOST")

	if mintHostEnv == "" && rwxHostEnv == "" {
		rwxHost = "cloud.rwx.com"
	} else if mintHostEnv != "" {
		rwxHost = mintHostEnv
	} else {
		rwxHost = rwxHostEnv
	}

	rootCmd.PersistentFlags().StringVar(&AccessToken, "access-token", "$RWX_ACCESS_TOKEN", "the access token for RWX")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "enable debug output")
	_ = rootCmd.PersistentFlags().MarkHidden("verbose")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(dispatchCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(vaultsCmd)
	rootCmd.AddCommand(packagesCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(resolveCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(pushCmd)

	cobra.OnInitialize(func() {
		if AccessToken == "$RWX_ACCESS_TOKEN" {
			AccessToken = os.Getenv("RWX_ACCESS_TOKEN")
		}
	})
}
