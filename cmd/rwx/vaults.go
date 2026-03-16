package main

import (
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/spf13/cobra"
)

var vaultsCmd = &cobra.Command{
	GroupID: "api",
	Short:   "Manage vaults and secrets",
	Use:     "vaults",
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

			useJson := useJsonOutput()
			_, err := service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
				Vault:   Vault,
				File:    File,
				Secrets: secrets,
				Json:    useJson,
			})
			return err
		},
		Short: "Set secrets in a vault",
		Use:   "set-secrets [flags] [SECRETNAME=secretvalue]",
	}
)

// --- vaults create ---

var (
	createVaultName      string
	createVaultUnlocked  bool
	createVaultRepoPerms []string

	vaultsCreateCmd = &cobra.Command{
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			useJson := useJsonOutput()
			_, err := service.CreateVault(cli.CreateVaultConfig{
				Name:                  createVaultName,
				Unlocked:              createVaultUnlocked,
				RepositoryPermissions: createVaultRepoPerms,
				Json:                  useJson,
			})
			return err
		},
		Short: "Create a new vault",
		Use:   "create [flags]",
	}
)

func init() {
	// vaults create
	vaultsCreateCmd.Flags().StringVar(&createVaultName, "name", "", "the name of the vault to create")
	_ = vaultsCreateCmd.MarkFlagRequired("name")
	vaultsCreateCmd.Flags().BoolVar(&createVaultUnlocked, "unlocked", false, "whether the vault should be unlocked")
	vaultsCreateCmd.Flags().StringSliceVar(&createVaultRepoPerms, "repository-permission", nil, "repository permission in the format REPO_SLUG:BRANCH_PATTERN (repeatable)")
	vaultsCmd.AddCommand(vaultsCreateCmd)

	// vaults set-secrets
	vaultsSetSecretsCmd.Flags().StringVar(&Vault, "vault", "default", "the name of the vault to set the secrets in")
	vaultsSetSecretsCmd.Flags().StringVar(&File, "file", "", "the path to a file in dotenv format to read the secrets from")
	vaultsCmd.AddCommand(vaultsSetSecretsCmd)
}
