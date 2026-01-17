package cli_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_SettingSecrets(t *testing.T) {
	t.Run("when unable to set secrets", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
			require.Equal(t, "default", ssivc.VaultName)
			require.Equal(t, "ABC", ssivc.Secrets[0].Name)
			require.Equal(t, "123", ssivc.Secrets[0].Secret)
			return nil, errors.New("error setting secret")
		}

		result, err := s.service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
			Vault:   "default",
			Secrets: []string{"ABC=123"},
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error setting secret")
	})

	t.Run("with secrets set", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
			require.Equal(t, "default", ssivc.VaultName)
			require.Equal(t, "ABC", ssivc.Secrets[0].Name)
			require.Equal(t, "123", ssivc.Secrets[0].Secret)
			require.Equal(t, "DEF", ssivc.Secrets[1].Name)
			require.Equal(t, `"xyz"`, ssivc.Secrets[1].Secret)
			return &api.SetSecretsInVaultResult{
				SetSecrets: []string{"ABC", "DEF"},
			}, nil
		}

		result, err := s.service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
			Vault:   "default",
			Secrets: []string{"ABC=123", `DEF="xyz"`},
		})

		require.NoError(t, err)
		require.Equal(t, []string{"ABC", "DEF"}, result.SetSecrets)
		require.Equal(t, "\nSuccessfully set the following secrets: ABC, DEF", s.mockStdout.String())
	})

	t.Run("when reading secrets from a file", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
			sort.Slice(ssivc.Secrets, func(i, j int) bool {
				return ssivc.Secrets[i].Name < ssivc.Secrets[j].Name
			})
			require.Equal(t, "default", ssivc.VaultName)
			require.Equal(t, "A", ssivc.Secrets[0].Name)
			require.Equal(t, "123", ssivc.Secrets[0].Secret)
			require.Equal(t, "B", ssivc.Secrets[1].Name)
			require.Equal(t, "xyz", ssivc.Secrets[1].Secret)
			require.Equal(t, "C", ssivc.Secrets[2].Name)
			require.Equal(t, "q\\nqq", ssivc.Secrets[2].Secret)
			require.Equal(t, "D", ssivc.Secrets[3].Name)
			require.Equal(t, "a multiline\nstring\nspanning lines", ssivc.Secrets[3].Secret)
			return &api.SetSecretsInVaultResult{
				SetSecrets: []string{"A", "B", "C", "D"},
			}, nil
		}

		secretsFile := filepath.Join(s.tmp, "secrets.txt")
		err := os.WriteFile(secretsFile, []byte("A=123\nB=\"xyz\"\nC='q\\nqq'\nD=\"a multiline\nstring\nspanning lines\""), 0o644)
		require.NoError(t, err)

		result, err := s.service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
			Vault:   "default",
			Secrets: []string{},
			File:    secretsFile,
		})

		require.NoError(t, err)
		require.Equal(t, []string{"A", "B", "C", "D"}, result.SetSecrets)
		require.Equal(t, "\nSuccessfully set the following secrets: A, B, C, D", s.mockStdout.String())
	})
}
