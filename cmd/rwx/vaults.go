package main

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var vaultsCmd = &cobra.Command{
	Short: "Manage Mint vaults and secrets",
	Use:   "vaults",
}

var (
	Vault string
	File  string

	vaultsSetSecretsCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var secrets []string
			if len(args) >= 0 {
				secrets = args
			}

			return service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
				Vault:   Vault,
				File:    File,
				Secrets: secrets,
			})
		},
		Short: "Set secrets in a vault",
		Use:   "set-secrets [flags] [SECRETNAME=secretvalue]",
	}
)

func init() {
	vaultsSetSecretsCmd.Flags().StringVar(&Vault, "vault", "default", "the name of the vault to set the secrets in")
	vaultsSetSecretsCmd.Flags().StringVar(&File, "file", "", "the path to a file in dotenv format to read the secrets from")
	vaultsCmd.AddCommand(vaultsSetSecretsCmd)
}
