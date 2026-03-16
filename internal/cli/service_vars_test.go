package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_SetVars(t *testing.T) {
	t.Run("when unable to set vars", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetVar = func(cfg api.SetVarConfig) (*api.SetVarResult, error) {
			return nil, errors.New("error setting var")
		}

		result, err := s.service.SetVars(cli.SetVarsConfig{
			Vault: "default",
			Vars:  []string{"ABC=123"},
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "error setting var")
		require.Empty(t, result.SetVars)
	})

	t.Run("with vars set", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetVar = func(cfg api.SetVarConfig) (*api.SetVarResult, error) {
			require.Equal(t, "default", cfg.VaultName)
			return &api.SetVarResult{}, nil
		}

		result, err := s.service.SetVars(cli.SetVarsConfig{
			Vault: "default",
			Vars:  []string{"ABC=123", "DEF=xyz"},
		})

		require.NoError(t, err)
		require.Equal(t, []string{"ABC", "DEF"}, result.SetVars)
		require.Equal(t, "Set vars: ABC, DEF\n", s.mockStdout.String())
	})

	t.Run("when reading vars from a file", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetVar = func(cfg api.SetVarConfig) (*api.SetVarResult, error) {
			require.Equal(t, "default", cfg.VaultName)
			return &api.SetVarResult{}, nil
		}

		varsFile := filepath.Join(s.tmp, "vars.txt")
		err := os.WriteFile(varsFile, []byte("A=123\nB=xyz"), 0o644)
		require.NoError(t, err)

		result, err := s.service.SetVars(cli.SetVarsConfig{
			Vault: "default",
			Vars:  []string{},
			File:  varsFile,
		})

		require.NoError(t, err)
		require.Len(t, result.SetVars, 2)
	})

	t.Run("with invalid var format", func(t *testing.T) {
		s := setupTest(t)

		result, err := s.service.SetVars(cli.SetVarsConfig{
			Vault: "default",
			Vars:  []string{"BADFORMAT"},
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "KEY=value")
	})

	t.Run("with json output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockSetVar = func(cfg api.SetVarConfig) (*api.SetVarResult, error) {
			return &api.SetVarResult{}, nil
		}

		result, err := s.service.SetVars(cli.SetVarsConfig{
			Vault: "default",
			Vars:  []string{"ABC=123"},
			Json:  true,
		})

		require.NoError(t, err)
		require.Equal(t, []string{"ABC"}, result.SetVars)
		require.Contains(t, s.mockStdout.String(), `"SetVars"`)
	})
}
