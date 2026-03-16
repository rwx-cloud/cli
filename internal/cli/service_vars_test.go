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

func TestService_ShowVar(t *testing.T) {
	t.Run("shows a var", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockShowVar = func(cfg api.ShowVarConfig) (*api.ShowVarResult, error) {
			require.Equal(t, "MY_VAR", cfg.VarName)
			require.Equal(t, "default", cfg.VaultName)
			return &api.ShowVarResult{
				Name:  "MY_VAR",
				Value: "hello-world",
			}, nil
		}

		result, err := s.service.ShowVar(cli.ShowVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
		})

		require.NoError(t, err)
		require.Equal(t, "MY_VAR", result.Name)
		require.Equal(t, "hello-world", result.Value)
		require.Equal(t, "hello-world\n", s.mockStdout.String())
	})

	t.Run("when var not found", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockShowVar = func(cfg api.ShowVarConfig) (*api.ShowVarResult, error) {
			return nil, errors.New("not found")
		}

		result, err := s.service.ShowVar(cli.ShowVarConfig{
			VarName: "MISSING",
			Vault:   "default",
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("with json output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockShowVar = func(cfg api.ShowVarConfig) (*api.ShowVarResult, error) {
			return &api.ShowVarResult{
				Name:  "MY_VAR",
				Value: "hello-world",
			}, nil
		}

		result, err := s.service.ShowVar(cli.ShowVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
			Json:    true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, s.mockStdout.String(), `"Name":"MY_VAR"`)
		require.Contains(t, s.mockStdout.String(), `"Value":"hello-world"`)
	})
}

func TestService_DeleteVar(t *testing.T) {
	t.Run("deletes a var", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockDeleteVar = func(cfg api.DeleteVarConfig) (*api.DeleteVarResult, error) {
			require.Equal(t, "MY_VAR", cfg.VarName)
			require.Equal(t, "default", cfg.VaultName)
			return &api.DeleteVarResult{}, nil
		}

		result, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
			Yes:     true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "Deleted var \"MY_VAR\" from vault \"default\".\n", s.mockStdout.String())
	})

	t.Run("when unable to delete var", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockDeleteVar = func(cfg api.DeleteVarConfig) (*api.DeleteVarResult, error) {
			return nil, errors.New("not found")
		}

		result, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MISSING",
			Vault:   "default",
			Yes:     true,
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("with json output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockDeleteVar = func(cfg api.DeleteVarConfig) (*api.DeleteVarResult, error) {
			return &api.DeleteVarResult{}, nil
		}

		result, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
			Json:    true,
			Yes:     true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, s.mockStdout.String(), `"Var":"MY_VAR"`)
		require.Contains(t, s.mockStdout.String(), `"Vault":"default"`)
	})

	t.Run("requires --yes in non-interactive environments", func(t *testing.T) {
		s := setupTest(t)

		_, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "use --yes to confirm")
	})

	t.Run("prompts for confirmation in TTY", func(t *testing.T) {
		s := setupTestWithTTY(t)
		s.mockStdin.WriteString("y\n")

		s.mockAPI.MockDeleteVar = func(cfg api.DeleteVarConfig) (*api.DeleteVarResult, error) {
			return &api.DeleteVarResult{}, nil
		}

		result, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, s.mockStderr.String(), `Delete var "MY_VAR" from vault "default"?`)
	})

	t.Run("aborts when user declines confirmation", func(t *testing.T) {
		s := setupTestWithTTY(t)
		s.mockStdin.WriteString("n\n")

		_, err := s.service.DeleteVar(cli.DeleteVarConfig{
			VarName: "MY_VAR",
			Vault:   "default",
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "aborted")
	})
}
