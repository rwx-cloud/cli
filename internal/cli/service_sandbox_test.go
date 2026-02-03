package cli_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	sandboxPrivateTestKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDiyT6ht8Z2XBEJpLR4/xmNouq5KDdn5G++cUcTH4EhzwAAAJhIWxlBSFsZ
QQAAAAtzc2gtZWQyNTUxOQAAACDiyT6ht8Z2XBEJpLR4/xmNouq5KDdn5G++cUcTH4Ehzw
AAAEC6442PQKevgYgeT0SIu9zwlnEMl6MF59ZgM+i0ByMv4eLJPqG3xnZcEQmktHj/GY2i
6rkoN2fkb75xRxMfgSHPAAAAEG1pbnQgQ0xJIHRlc3RpbmcBAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`
	sandboxPublicTestKey = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLJPqG3xnZcEQmktHj/GY2i6rkoN2fkb75xRxMfgSHP rwx CLI testing`
)

func TestService_ListSandboxes(t *testing.T) {
	t.Run("returns list without error", func(t *testing.T) {
		setup := setupTest(t)

		// Note: This test may return sandboxes from the user's actual storage file
		// since sandbox storage uses ~/.config/rwx/sandboxes.json
		result, err := setup.service.ListSandboxes(cli.ListSandboxesConfig{
			Json: true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Sandboxes)
	})
}

func TestService_ExecSandbox(t *testing.T) {
	t.Run("executes command in sandbox successfully", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-123"
		address := "192.168.1.1:22"
		connectedViaSSH := false
		commandExecuted := false
		executedCommand := ""

		// Mock run status to indicate run is active
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:   runID,
				Polling: api.PollingResult{Completed: false},
			}, nil
		}

		// Mock sandbox connection info
		setup.mockAPI.MockGetSandboxConnectionInfo = func(id string) (api.SandboxConnectionInfo, error) {
			require.Equal(t, runID, id)
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnectWithKey = func(addr string, _ ssh.ClientConfig, privateKeyPEM string) error {
			require.Equal(t, address, addr)
			connectedViaSSH = true
			return nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			commandExecuted = true
			executedCommand = cmd
			return 0, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.True(t, connectedViaSSH)
		require.True(t, commandExecuted)
		require.Equal(t, "echo hello", executedCommand)
	})

	t.Run("returns non-zero exit code from command", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-456"
		address := "192.168.1.1:22"

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:   runID,
				Polling: api.PollingResult{Completed: false},
			}, nil
		}

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnectWithKey = func(addr string, _ ssh.ClientConfig, privateKeyPEM string) error {
			return nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 1, nil // Non-zero exit code
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"false"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 1, result.ExitCode)
	})

	t.Run("returns error when run is no longer active", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-expired"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable: false,
				Polling:     api.PollingResult{Completed: true}, // Run has ended
			}, nil
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "completed before becoming ready")
	})
}

func TestService_ExecSandbox_Sync(t *testing.T) {
	// Note: The sync functionality with Sync=true uses rsync which requires
	// external SSH connectivity and cannot be easily unit tested.
	// These tests only verify the Sync=false path.

	t.Run("skips sync when Sync is false", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-sync-123"
		address := "192.168.1.1:22"
		rsyncCalled := false

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnectWithKey = func(addr string, _ ssh.ClientConfig, privateKeyPEM string) error {
			return nil
		}

		// If rsync were called, it would need GetConnectionDetails
		setup.mockSSH.MockGetConnectionDetails = func() (string, string, string) {
			rsyncCalled = true
			return address, "mint-cli", sandboxPrivateTestKey
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       false,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.False(t, rsyncCalled)
	})
}

func TestService_StopSandbox(t *testing.T) {
	t.Run("returns error when no sandbox exists for current directory", func(t *testing.T) {
		setup := setupTest(t)

		_, err := setup.service.StopSandbox(cli.StopSandboxConfig{
			Json: true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "No sandbox found")
	})

	t.Run("returns error when sandbox ID not found in storage", func(t *testing.T) {
		setup := setupTest(t)

		_, err := setup.service.StopSandbox(cli.StopSandboxConfig{
			RunID: "nonexistent-run",
			Json:  true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "not found in local storage")
	})
}
