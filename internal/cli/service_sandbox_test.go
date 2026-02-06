package cli_test

import (
	"io"
	"strings"
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/git"
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
		var executedCommands []string

		// Mock run status to indicate run is active
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:   runID,
				Polling: api.PollingResult{Completed: false},
			}, nil
		}

		// Mock sandbox connection info
		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			require.Equal(t, runID, id)
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			require.Equal(t, address, addr)
			connectedViaSSH = true
			return nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			executedCommands = append(executedCommands, cmd)
			return 0, nil
		}

		// Pull mocks (no changes on sandbox)
		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
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
		require.Contains(t, executedCommands, "echo hello")
	})

	t.Run("returns non-zero exit code from command", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-456"
		address := "192.168.1.1:22"
		userCommandRan := false

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:   runID,
				Polling: api.PollingResult{Completed: false},
			}, nil
		}

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			if cmd == "false" {
				userCommandRan = true
				return 1, nil // Non-zero exit code
			}
			return 0, nil // sync markers
		}

		// Pull mocks (no changes on sandbox)
		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"false"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 1, result.ExitCode)
		require.True(t, userCommandRan)
	})

	t.Run("returns error when run is no longer active", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-expired"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
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
	t.Run("syncs changes when Sync is true", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-sync-123"
		address := "192.168.1.1:22"
		patchApplied := false
		var appliedPatch []byte

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("diff --git a/file.txt b/file.txt\n"), nil, nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			// Return empty for git diff --name-only and ls-files (no dirty files on sandbox)
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			require.Equal(t, "/usr/bin/git apply --allow-empty -", command)
			appliedPatch, _ = io.ReadAll(stdin)
			patchApplied = true
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.True(t, patchApplied)
		require.Equal(t, "diff --git a/file.txt b/file.txt\n", string(appliedPatch))
	})

	t.Run("skips sync when Sync is false", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-sync-123"
		address := "192.168.1.1:22"
		syncPatchApplied := false

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockSSH.MockExecuteCommandWithStdin = func(command string, stdin io.Reader) (int, error) {
			syncPatchApplied = true
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		// Pull mocks (no changes on sandbox)
		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
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
		require.False(t, syncPatchApplied)
	})

	t.Run("skips sync when no changes", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-changes-123"
		address := "192.168.1.1:22"
		syncPatchApplied := false

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil // No changes
		}

		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithStdin = func(command string, stdin io.Reader) (int, error) {
			syncPatchApplied = true
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		// Pull mocks (no changes on sandbox)
		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.False(t, syncPatchApplied)
	})

	t.Run("warns and skips sync for LFS files", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-lfs-123"
		address := "192.168.1.1:22"
		syncPatchApplied := false
		generatePatchCallCount := 0

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			generatePatchCallCount++
			if generatePatchCallCount == 1 {
				// First call: sync phase - return LFS metadata
				return nil, &git.LFSChangedFilesMetadata{Files: []string{"large.bin"}, Count: 1}, nil
			}
			// Second call: pull phase - no local changes
			return nil, nil, nil
		}

		setup.mockSSH.MockExecuteCommandWithStdin = func(command string, stdin io.Reader) (int, error) {
			syncPatchApplied = true
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		// Pull mocks (no changes on sandbox)
		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       false, // Enable warning output
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.False(t, syncPatchApplied)
		require.Contains(t, setup.mockStderr.String(), "LFS file(s) changed")
	})

	t.Run("returns error when git apply fails", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-apply-fail-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("invalid patch"), nil, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			return 1, "error: patch failed", nil // git apply failed
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "git apply failed")
	})

	t.Run("syncs changes by resetting dirty files and applying patch", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-sync-123"
		address := "192.168.1.1:22"
		var commandOrder []string
		var combinedOutputOrder []string
		var stdinCommandOrder []string

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("diff --git a/file.txt b/file.txt\n"), nil, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			commandOrder = append(commandOrder, cmd)
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			combinedOutputOrder = append(combinedOutputOrder, cmd)
			// Return empty for git diff and ls-files (no dirty files on sandbox)
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			// Return empty for git diff --name-only and ls-files (no dirty files on sandbox)
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			stdinCommandOrder = append(stdinCommandOrder, command)
			return 0, "", nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		// Should have: reset for file.txt (from sync patch), user command, and pull sync marker
		require.GreaterOrEqual(t, len(commandOrder), 2)
		require.Contains(t, commandOrder[0], "git checkout HEAD -- 'file.txt'")
		require.Equal(t, "echo hello", commandOrder[1])
		// Sync markers and git diff/ls-files use ExecuteCommandWithCombinedOutput
		require.Equal(t, "__rwx_sandbox_sync_start__", combinedOutputOrder[0])
		// git apply uses stdin method
		require.GreaterOrEqual(t, len(stdinCommandOrder), 1)
		require.Equal(t, "/usr/bin/git apply --allow-empty -", stdinCommandOrder[0])
	})

	t.Run("returns helpful error when git is not installed", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-git-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("patch"), nil, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			return 127, "", nil // command not found
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "git is not installed")
	})

	t.Run("returns helpful error when .git directory is missing", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-git-dir-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("patch"), nil, nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			if cmd == "test -d .git" {
				return 1, "", nil // .git directory does not exist
			}
			return 0, "", nil
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "no .git directory")
		require.Contains(t, err.Error(), "preserve-git-dir: true")
	})

}

func TestService_ExecSandbox_Pull(t *testing.T) {
	t.Run("pulls changes from sandbox after command execution", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-pull-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		// Sandbox has a change to file.txt
		sandboxPatch := "diff --git a/file.txt b/file.txt\nindex abc..def 100644\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git diff HEAD") {
				return 0, sandboxPatch, nil
			}
			if strings.Contains(cmd, "git ls-files") {
				return 0, "", nil // No untracked files
			}
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}

		// No local changes
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.Contains(t, result.PulledFiles, "file.txt")
	})

	t.Run("pulls changes even when command exits non-zero", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-pull-nonzero-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		sandboxPatch := "diff --git a/file.txt b/file.txt\nindex abc..def 100644\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git diff HEAD") {
				return 0, sandboxPatch, nil
			}
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			if cmd == "make test" {
				return 1, nil // Command fails
			}
			return 0, nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"make", "test"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 1, result.ExitCode)
		require.Contains(t, result.PulledFiles, "file.txt")
	})

	t.Run("treats pull errors as warnings", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-pull-err-123"
		address := "192.168.1.1:22"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        address,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockSSH.MockExecuteCommandWithCombinedOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git ls-files") {
				return 1, "fatal: not a git repository", nil // Pull fails
			}
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
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
		require.Contains(t, setup.mockStderr.String(), "Warning: failed to pull changes from sandbox")
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
