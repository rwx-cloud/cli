package main

import (
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/spf13/cobra"
)

var vaultsCmd = &cobra.Command{
	GroupID: "api",
	Short:   "Manage vaults, secrets, and vars",
	Use:     "vaults",
}

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

// --- secrets subcommand group ---

var vaultsSecretsCmd = &cobra.Command{
	Short: "Manage secrets in a vault",
	Use:   "secrets",
}

var (
	secretsSetVault string
	secretsSetFile  string

	vaultsSecretsSetCmd = &cobra.Command{
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
				Vault:   secretsSetVault,
				File:    secretsSetFile,
				Secrets: secrets,
				Json:    useJson,
			})
			return err
		},
		Short: "Set secrets in a vault",
		Use:   "set [flags] [SECRETNAME=secretvalue]",
	}
)

var (
	secretsDeleteVault string

	vaultsSecretsDeleteCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAccessToken()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			useJson := useJsonOutput()
			_, err := service.DeleteSecret(cli.DeleteSecretConfig{
				SecretName: args[0],
				Vault:      secretsDeleteVault,
				Json:       useJson,
			})
			return err
		},
		Short: "Delete a secret from a vault",
		Use:   "delete NAME [flags]",
	}
)

// --- set-secrets alias (backwards compatibility) ---

var (
	setSecretsVault string
	setSecretsFile  string

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
				Vault:   setSecretsVault,
				File:    setSecretsFile,
				Secrets: secrets,
				Json:    useJson,
			})
			return err
		},
		Hidden: true,
		Short:  "Set secrets in a vault",
		Use:    "set-secrets [flags] [SECRETNAME=secretvalue]",
	}
)

func init() {
	// vaults create
	vaultsCreateCmd.Flags().StringVar(&createVaultName, "name", "", "the name of the vault to create")
	_ = vaultsCreateCmd.MarkFlagRequired("name")
	vaultsCreateCmd.Flags().BoolVar(&createVaultUnlocked, "unlocked", false, "whether the vault should be unlocked")
	vaultsCreateCmd.Flags().StringSliceVar(&createVaultRepoPerms, "repository-permission", nil, "repository permission in the format REPO_SLUG:BRANCH_PATTERN (repeatable)")
	vaultsCmd.AddCommand(vaultsCreateCmd)

	// vaults secrets set
	vaultsSecretsSetCmd.Flags().StringVar(&secretsSetVault, "vault", "default", "the name of the vault to set the secrets in")
	vaultsSecretsSetCmd.Flags().StringVar(&secretsSetFile, "file", "", "the path to a file in dotenv format to read the secrets from")
	vaultsSecretsCmd.AddCommand(vaultsSecretsSetCmd)

	// vaults secrets delete
	vaultsSecretsDeleteCmd.Flags().StringVar(&secretsDeleteVault, "vault", "default", "the name of the vault to delete the secret from")
	vaultsSecretsCmd.AddCommand(vaultsSecretsDeleteCmd)

	vaultsCmd.AddCommand(vaultsSecretsCmd)

	// vaults set-secrets (alias for backwards compatibility)
	vaultsSetSecretsCmd.Flags().StringVar(&setSecretsVault, "vault", "default", "the name of the vault to set the secrets in")
	vaultsSetSecretsCmd.Flags().StringVar(&setSecretsFile, "file", "", "the path to a file in dotenv format to read the secrets from")
	vaultsCmd.AddCommand(vaultsSetSecretsCmd)
}
