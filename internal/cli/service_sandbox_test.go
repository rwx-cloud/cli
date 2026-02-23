package cli_test

import (
	"io"
	"os"
	"path/filepath"
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
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 0, result.ExitCode)
		require.Equal(t, "", result.RunURL)
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
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 1, result.ExitCode)
		require.True(t, userCommandRan)
	})

	t.Run("shell-quotes command arguments for remote execution", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-quote-123"
		address := "192.168.1.1:22"
		var executedCommands []string

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
			executedCommands = append(executedCommands, cmd)
			return 0, nil
		}

		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"bash", "-c", "cat README.md"},
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)
		require.Contains(t, executedCommands, "bash -c 'cat README.md'")
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
		require.Equal(t, runID, result.RunID)
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
		require.Equal(t, runID, result.RunID)
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

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 0, result.ExitCode)
		require.False(t, syncPatchApplied)
	})

	t.Run("creates sync ref even when no local changes", func(t *testing.T) {
		// When there are no local changes, sync returns early but must still
		// create refs/rwx-sync so pull has a valid baseline to diff against.
		setup := setupTest(t)

		runID := "run-no-changes-ref"
		address := "192.168.1.1:22"
		createdSyncRef := false

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

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			if strings.Contains(cmd, "update-ref refs/rwx-sync HEAD") {
				createdSyncRef = true
			}
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
		require.True(t, createdSyncRef, "sync should create refs/rwx-sync even with no local changes")
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

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       false, // Enable warning output
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, runID, result.RunID)
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

	t.Run("syncs changes and reverts sandbox after pull", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-sync-123"
		address := "192.168.1.1:22"
		var commandOrder []string
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

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
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
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 0, result.ExitCode)
		require.Contains(t, commandOrder[0], "__rwx_sandbox_sync_start__")
		require.Contains(t, commandOrder, "echo hello")
		// git apply uses stdin method
		require.GreaterOrEqual(t, len(stdinCommandOrder), 1)
		require.Equal(t, "/usr/bin/git apply --allow-empty -", stdinCommandOrder[0])
		// Sandbox should be reverted after pull (git checkout + git clean)
		lastSyncEnd := -1
		for i, cmd := range commandOrder {
			if strings.Contains(cmd, "git checkout .") && strings.Contains(cmd, "git clean -fd") {
				lastSyncEnd = i
			}
		}
		require.NotEqual(t, -1, lastSyncEnd, "sandbox should be reverted after pull")
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
			if cmd == "test -d .git" {
				return 1, nil // .git directory does not exist
			}
			return 0, nil
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

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git diff refs/rwx-sync") {
				return 0, sandboxPatch, nil
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
		require.Equal(t, runID, result.RunID)
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

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git diff refs/rwx-sync") {
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
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 1, result.ExitCode)
		require.Contains(t, result.PulledFiles, "file.txt")
	})

	t.Run("pull uses stdout-only capture for git data commands", func(t *testing.T) {
		// Regression test for: "Fix sandbox pull erasing local uncommitted changes"
		//
		// Previously, pullChangesFromSandbox used ExecuteCommandWithCombinedOutput which
		// mixed stderr into stdout. When git commands produced warnings on stderr, the
		// patch data was corrupted. Since local files were already reset to HEAD before
		// applying the patch, corrupted patches caused local uncommitted changes to be erased.
		//
		// This test exercises the full pull path with untracked files on the sandbox,
		// verifying that git ls-files, git add -N, git diff refs/rwx-sync, and git reset HEAD
		// all go through ExecuteCommandWithOutput (stdout-only).
		setup := setupTest(t)

		runID := "run-pull-stdout-only"
		address := "192.168.1.1:22"
		var stdoutOnlyCommands []string

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

		// The sandbox has a tracked change and an untracked file
		sandboxPatch := "diff --git a/tracked.txt b/tracked.txt\nindex abc..def 100644\n--- a/tracked.txt\n+++ b/tracked.txt\n@@ -1 +1 @@\n-old\n+new\ndiff --git a/untracked.txt b/untracked.txt\nnew file mode 100644\nindex 0000000..abc1234\n--- /dev/null\n+++ b/untracked.txt\n@@ -0,0 +1 @@\n+new file\n"

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			stdoutOnlyCommands = append(stdoutOnlyCommands, cmd)
			if strings.Contains(cmd, "git ls-files --others") {
				return 0, "untracked.txt\n", nil
			}
			if strings.Contains(cmd, "git diff refs/rwx-sync") {
				return 0, sandboxPatch, nil
			}
			return 0, "", nil
		}

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
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
		require.Contains(t, result.PulledFiles, "tracked.txt")
		require.Contains(t, result.PulledFiles, "untracked.txt")

		// Verify that data-capturing git commands used stdout-only output
		// (these are called via ExecuteCommandWithOutput, not combined output)
		foundLsFiles := false
		foundAddN := false
		foundDiffRef := false
		foundReset := false
		for _, cmd := range stdoutOnlyCommands {
			if strings.Contains(cmd, "git ls-files --others") {
				foundLsFiles = true
			}
			if strings.Contains(cmd, "git add -N") {
				foundAddN = true
			}
			if strings.Contains(cmd, "git diff refs/rwx-sync") {
				foundDiffRef = true
			}
			if strings.Contains(cmd, "git reset HEAD") {
				foundReset = true
			}
		}
		require.True(t, foundLsFiles, "git ls-files should use stdout-only output")
		require.True(t, foundAddN, "git add -N should use stdout-only output")
		require.True(t, foundDiffRef, "git diff refs/rwx-sync should use stdout-only output")
		require.True(t, foundReset, "git reset HEAD should use stdout-only output")
	})

	t.Run("pull only includes sandbox exec-changed files", func(t *testing.T) {
		// With the sync snapshot commit, the sandbox diff only captures changes made
		// during exec (not local changes that were synced before exec). Local changes
		// are already present in the working tree and don't need to be pulled back.
		setup := setupTest(t)

		runID := "run-pull-local-changes"
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

		// Sandbox has changes to file-a.txt only (exec-only changes)
		sandboxPatch := "diff --git a/file-a.txt b/file-a.txt\nindex abc..def 100644\n--- a/file-a.txt\n+++ b/file-a.txt\n@@ -1 +1 @@\n-old\n+new\n"

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
			if strings.Contains(cmd, "git diff refs/rwx-sync") {
				return 0, sandboxPatch, nil
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
		// Only sandbox exec-changed files should be in the pulled files list
		require.Contains(t, result.PulledFiles, "file-a.txt")
		require.Equal(t, 1, len(result.PulledFiles))
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

		setup.mockSSH.MockExecuteCommandWithOutput = func(cmd string) (int, string, error) {
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
		require.Equal(t, runID, result.RunID)
		require.Equal(t, 0, result.ExitCode)
		require.Contains(t, setup.mockStderr.String(), "Warning: failed to pull changes from sandbox")
	})
}

func TestService_StartSandbox(t *testing.T) {
	t.Run("does not send a git patch", func(t *testing.T) {
		setup := setupTest(t)

		// Create .rwx directory and sandbox config file
		rwxDir := filepath.Join(setup.tmp, ".rwx")
		err := os.MkdirAll(rwxDir, 0o755)
		require.NoError(t, err)

		sandboxConfig := "tasks:\n  - key: sandbox\n    run: rwx-sandbox\n"
		err = os.WriteFile(filepath.Join(rwxDir, "sandbox.yml"), []byte(sandboxConfig), 0o644)
		require.NoError(t, err)

		// Mock git — set up a patch that would be sent if patching were enabled
		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "git@github.com:example/repo.git"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{
			Written:        true,
			UntrackedFiles: git.UntrackedFilesMetadata{Files: []string{"foo.txt"}, Count: 1},
		}

		// Mock API
		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{
				Image:  "ubuntu:24.04",
				Config: "rwx/base 1.0.0",
				Arch:   "x86_64",
			}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: make(map[string]string),
				LatestMinor: make(map[string]map[string]string),
			}, nil
		}

		var receivedPatch api.PatchMetadata
		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			receivedPatch = cfg.Patch
			return &api.InitiateRunResult{
				RunID:  "run-123",
				RunURL: "https://cloud.rwx.com/mint/runs/run-123",
			}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "test-token"}, nil
		}

		_, err = setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})

		require.NoError(t, err)
		require.False(t, receivedPatch.Sent, "sandbox start should not send a git patch")
	})

	t.Run("passes init params to InitiateRun", func(t *testing.T) {
		setup := setupTest(t)

		// Create .rwx directory and sandbox config file
		rwxDir := filepath.Join(setup.tmp, ".rwx")
		err := os.MkdirAll(rwxDir, 0o755)
		require.NoError(t, err)

		sandboxConfig := "tasks:\n  - key: sandbox\n    run: rwx-sandbox\n"
		err = os.WriteFile(filepath.Join(rwxDir, "sandbox.yml"), []byte(sandboxConfig), 0o644)
		require.NoError(t, err)

		// Mock git
		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "git@github.com:example/repo.git"

		// Mock API
		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{
				Image:  "ubuntu:24.04",
				Config: "rwx/base 1.0.0",
				Arch:   "x86_64",
			}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: make(map[string]string),
				LatestMinor: make(map[string]map[string]string),
			}, nil
		}

		var receivedInitParams []api.InitializationParameter
		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			receivedInitParams = cfg.InitializationParameters
			return &api.InitiateRunResult{
				RunID:  "run-init-123",
				RunURL: "https://cloud.rwx.com/mint/runs/run-init-123",
			}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "test-token"}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile:     ".rwx/sandbox.yml",
			Json:           true,
			InitParameters: map[string]string{"foo": "bar"},
		})

		require.NoError(t, err)
		require.Equal(t, "run-init-123", result.RunID)
		require.Equal(t, "https://cloud.rwx.com/mint/runs/run-init-123", result.RunURL)
		require.Len(t, receivedInitParams, 1)
		require.Equal(t, "foo", receivedInitParams[0].Key)
		require.Equal(t, "bar", receivedInitParams[0].Value)
	})
}

func TestService_ExecSandbox_InitParams(t *testing.T) {
	t.Run("lazy-create passes init params through to InitiateRun", func(t *testing.T) {
		setup := setupTest(t)

		// Create .rwx directory and sandbox config file
		rwxDir := filepath.Join(setup.tmp, ".rwx")
		err := os.MkdirAll(rwxDir, 0o755)
		require.NoError(t, err)

		sandboxConfig := "tasks:\n  - key: sandbox\n    run: rwx-sandbox\n"
		err = os.WriteFile(filepath.Join(rwxDir, "sandbox.yml"), []byte(sandboxConfig), 0o644)
		require.NoError(t, err)

		// Mock git
		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "git@github.com:example/repo.git"

		// Mock API
		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{
				Image:  "ubuntu:24.04",
				Config: "rwx/base 1.0.0",
				Arch:   "x86_64",
			}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: make(map[string]string),
				LatestMinor: make(map[string]map[string]string),
			}, nil
		}

		var receivedInitParams []api.InitializationParameter
		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			receivedInitParams = cfg.InitializationParameters
			return &api.InitiateRunResult{
				RunID:  "run-lazy-123",
				RunURL: "https://cloud.rwx.com/mint/runs/run-lazy-123",
			}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "test-token"}, nil
		}

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
		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		// No RunID provided - forces lazy-create path
		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			Command:        []string{"echo", "hello"},
			Json:           true,
			InitParameters: map[string]string{"foo": "bar"},
		})

		require.NoError(t, err)
		require.Equal(t, "run-lazy-123", result.RunID)
		require.Len(t, receivedInitParams, 1)
		require.Equal(t, "foo", receivedInitParams[0].Key)
		require.Equal(t, "bar", receivedInitParams[0].Value)
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

func TestService_ExecSandbox_RunURL(t *testing.T) {
	t.Run("returns empty RunURL when no server-provided URL is stored", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-no-url"
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

		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
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
		require.Equal(t, "", result.RunURL)
	})
}

func TestService_StartSandbox_RunURL(t *testing.T) {
	t.Run("reattach via --id returns empty RunURL", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-reattach-no-url"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable: true,
				Polling:     api.PollingResult{Completed: false},
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "test-token"}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, "", result.RunURL)
	})

	t.Run("normal start uses server-provided RunURL", func(t *testing.T) {
		setup := setupTest(t)

		// Create .rwx directory and sandbox config file
		rwxDir := filepath.Join(setup.tmp, ".rwx")
		err := os.MkdirAll(rwxDir, 0o755)
		require.NoError(t, err)

		sandboxConfig := "tasks:\n  - key: sandbox\n    run: rwx-sandbox\n"
		err = os.WriteFile(filepath.Join(rwxDir, "sandbox.yml"), []byte(sandboxConfig), 0o644)
		require.NoError(t, err)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "git@github.com:example/repo.git"

		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{
				Image:  "ubuntu:24.04",
				Config: "rwx/base 1.0.0",
				Arch:   "x86_64",
			}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: make(map[string]string),
				LatestMinor: make(map[string]map[string]string),
			}, nil
		}

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{
				RunID:  "run-server-url",
				RunURL: "https://cloud.rwx.com/mint/my-org/runs/run-server-url",
			}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "test-token"}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})

		require.NoError(t, err)
		// Should use the server-provided URL (which includes org slug), not a constructed one
		require.Equal(t, "https://cloud.rwx.com/mint/my-org/runs/run-server-url", result.RunURL)
	})
}
