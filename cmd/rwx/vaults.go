package main

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var vaultsCmd = &cobra.Command{
	GroupID: "api",
	Short:   "Manage vaults and secrets",
	Use:     "vaults",
}

var (
	Vault                  string
	File                   string
	VaultsSetSecretsJson   bool
	VaultsSetSecretsOutput string

	vaultsSetSecretsCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var secrets []string
			if len(args) >= 0 {
				secrets = args
			}

			useJson := VaultsSetSecretsOutput == "json" || VaultsSetSecretsJson
			return service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
				Vault:   Vault,
				File:    File,
				Secrets: secrets,
				Json:    useJson,
			})
		},
		Short: "Set secrets in a vault",
		Use:   "set-secrets [flags] [SECRETNAME=secretvalue]",
	}
)

func init() {
	vaultsSetSecretsCmd.Flags().StringVar(&Vault, "vault", "default", "the name of the vault to set the secrets in")
	vaultsSetSecretsCmd.Flags().StringVar(&File, "file", "", "the path to a file in dotenv format to read the secrets from")
	vaultsSetSecretsCmd.Flags().BoolVar(&VaultsSetSecretsJson, "json", false, "output JSON instead of text")
	_ = vaultsSetSecretsCmd.Flags().MarkHidden("json")
	vaultsSetSecretsCmd.Flags().StringVar(&VaultsSetSecretsOutput, "output", "text", "output format: text or json")
	vaultsCmd.AddCommand(vaultsSetSecretsCmd)
}
