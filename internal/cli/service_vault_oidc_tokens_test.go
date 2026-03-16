package cli_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_CreateVaultOidcToken(t *testing.T) {
	t.Run("creates token with explicit name and audience", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockCreateVaultOidcToken = func(cfg api.CreateVaultOidcTokenConfig) (*api.CreateVaultOidcTokenResult, error) {
			require.Equal(t, "my-vault", cfg.VaultName)
			require.Equal(t, "my-token", cfg.Name)
			require.Equal(t, "sts.amazonaws.com", cfg.Audience)
			require.Empty(t, cfg.Provider)
			return &api.CreateVaultOidcTokenResult{
				Audience:         "sts.amazonaws.com",
				Subject:          "org:my-org:vault:my-vault",
				Expression:       "${{ vaults.my-vault.oidc_token.my-token }}",
				DocumentationURL: "https://www.rwx.com/docs/oidc",
			}, nil
		}

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Name:     "my-token",
			Audience: "sts.amazonaws.com",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, s.mockStdout.String(), `Created OIDC token "my-token" in vault "my-vault".`)
		require.Contains(t, s.mockStdout.String(), "Audience:   sts.amazonaws.com")
		require.Contains(t, s.mockStdout.String(), "Subject:    org:my-org:vault:my-vault")
		require.Contains(t, s.mockStdout.String(), "Expression: ${{ vaults.my-vault.oidc_token.my-token }}")
		require.Contains(t, s.mockStdout.String(), "For more information on configuring your identity provider and using your OIDC token, see: https://www.rwx.com/docs/oidc")
	})

	t.Run("passes provider through to the API", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockCreateVaultOidcToken = func(cfg api.CreateVaultOidcTokenConfig) (*api.CreateVaultOidcTokenResult, error) {
			require.Equal(t, "my-vault", cfg.VaultName)
			require.Equal(t, "aws", cfg.Provider)
			require.Empty(t, cfg.Name)
			require.Empty(t, cfg.Audience)
			return &api.CreateVaultOidcTokenResult{
				Audience:         "sts.amazonaws.com",
				Subject:          "org:my-org:vault:my-vault",
				Expression:       "${{ vaults.my-vault.oidc_token.aws }}",
				DocumentationURL: "https://www.rwx.com/docs/oidc-aws",
			}, nil
		}

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Provider: "aws",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("passes provider with explicit overrides to the API", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockCreateVaultOidcToken = func(cfg api.CreateVaultOidcTokenConfig) (*api.CreateVaultOidcTokenResult, error) {
			require.Equal(t, "aws", cfg.Provider)
			require.Equal(t, "custom", cfg.Name)
			require.Equal(t, "custom-aud", cfg.Audience)
			return &api.CreateVaultOidcTokenResult{
				Audience:         "custom-aud",
				Subject:          "org:my-org:vault:my-vault",
				Expression:       "${{ vaults.my-vault.oidc_token.custom }}",
				DocumentationURL: "https://www.rwx.com/docs/oidc-aws",
			}, nil
		}

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Provider: "aws",
			Name:     "custom",
			Audience: "custom-aud",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("errors when name is missing without provider", func(t *testing.T) {
		s := setupTest(t)

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Audience: "sts.amazonaws.com",
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "--name is required")
	})

	t.Run("errors when audience is missing without provider", func(t *testing.T) {
		s := setupTest(t)

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault: "my-vault",
			Name:  "my-token",
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "--audience is required")
	})

	t.Run("when API returns an error", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockCreateVaultOidcToken = func(cfg api.CreateVaultOidcTokenConfig) (*api.CreateVaultOidcTokenResult, error) {
			return nil, errors.New("vault not found")
		}

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Name:     "my-token",
			Audience: "sts.amazonaws.com",
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "vault not found")
	})

	t.Run("with json output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockCreateVaultOidcToken = func(cfg api.CreateVaultOidcTokenConfig) (*api.CreateVaultOidcTokenResult, error) {
			return &api.CreateVaultOidcTokenResult{
				Audience:         "sts.amazonaws.com",
				Subject:          "org:my-org:vault:my-vault",
				Expression:       "${{ vaults.my-vault.oidc_token.my-token }}",
				DocumentationURL: "https://www.rwx.com/docs/oidc",
			}, nil
		}

		result, err := s.service.CreateVaultOidcToken(cli.CreateVaultOidcTokenConfig{
			Vault:    "my-vault",
			Name:     "my-token",
			Audience: "sts.amazonaws.com",
			Json:     true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, s.mockStdout.String(), `"Audience":"sts.amazonaws.com"`)
		require.Contains(t, s.mockStdout.String(), `"Subject":"org:my-org:vault:my-vault"`)
		require.Contains(t, s.mockStdout.String(), `"Expression":"${{ vaults.my-vault.oidc_token.my-token }}"`)
		require.Contains(t, s.mockStdout.String(), `"DocumentationURL":"https://www.rwx.com/docs/oidc"`)
	})
}
