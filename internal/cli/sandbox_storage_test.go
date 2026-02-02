package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionKey(t *testing.T) {
	t.Run("creates key from cwd, branch, and config file", func(t *testing.T) {
		key := cli.SessionKey("/home/user/project", "main", ".rwx/sandbox.yml")
		require.Equal(t, "/home/user/project:main:.rwx/sandbox.yml", key)
	})

	t.Run("uses 'detached' when branch is empty", func(t *testing.T) {
		key := cli.SessionKey("/home/user/project", "", ".rwx/sandbox.yml")
		require.Equal(t, "/home/user/project:detached:.rwx/sandbox.yml", key)
	})

	t.Run("handles paths with colons", func(t *testing.T) {
		key := cli.SessionKey("/home/user/project:with:colons", "feature/test", "config.yml")
		require.Equal(t, "/home/user/project:with:colons:feature/test:config.yml", key)
	})
}

func TestParseSessionKey(t *testing.T) {
	t.Run("parses standard key", func(t *testing.T) {
		cwd, branch, configFile := cli.ParseSessionKey("/home/user/project:main:.rwx/sandbox.yml")
		require.Equal(t, "/home/user/project", cwd)
		require.Equal(t, "main", branch)
		require.Equal(t, ".rwx/sandbox.yml", configFile)
	})

	t.Run("parses key with detached branch", func(t *testing.T) {
		cwd, branch, configFile := cli.ParseSessionKey("/home/user/project:detached:.rwx/sandbox.yml")
		require.Equal(t, "/home/user/project", cwd)
		require.Equal(t, "detached", branch)
		require.Equal(t, ".rwx/sandbox.yml", configFile)
	})

	t.Run("handles paths with colons in cwd", func(t *testing.T) {
		cwd, branch, configFile := cli.ParseSessionKey("/home/user/project:with:colons:main:config.yml")
		require.Equal(t, "/home/user/project:with:colons", cwd)
		require.Equal(t, "main", branch)
		require.Equal(t, "config.yml", configFile)
	})

	t.Run("round-trips with SessionKey", func(t *testing.T) {
		originalCwd := "/home/user/my-project"
		originalBranch := "feature/new-feature"
		originalConfig := ".rwx/sandbox.yml"

		key := cli.SessionKey(originalCwd, originalBranch, originalConfig)
		cwd, branch, configFile := cli.ParseSessionKey(key)

		require.Equal(t, originalCwd, cwd)
		require.Equal(t, originalBranch, branch)
		require.Equal(t, originalConfig, configFile)
	})

	t.Run("handles key with no colons", func(t *testing.T) {
		cwd, branch, configFile := cli.ParseSessionKey("invalid")
		require.Equal(t, "invalid", cwd)
		require.Equal(t, "", branch)
		require.Equal(t, "", configFile)
	})

	t.Run("handles key with single colon", func(t *testing.T) {
		cwd, branch, configFile := cli.ParseSessionKey("path:config")
		require.Equal(t, "path", cwd)
		require.Equal(t, "", branch)
		require.Equal(t, "config", configFile)
	})
}

func TestSandboxStorage_SessionOperations(t *testing.T) {
	t.Run("SetSession and GetSession", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		session := cli.SandboxSession{
			RunID:      "run-123",
			ConfigFile: ".rwx/sandbox.yml",
		}

		storage.SetSession("/home/user/project", "main", ".rwx/sandbox.yml", session)

		retrieved, found := storage.GetSession("/home/user/project", "main", ".rwx/sandbox.yml")
		require.True(t, found)
		require.Equal(t, "run-123", retrieved.RunID)
		require.Equal(t, ".rwx/sandbox.yml", retrieved.ConfigFile)
	})

	t.Run("GetSession returns false when not found", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		_, found := storage.GetSession("/home/user/project", "main", ".rwx/sandbox.yml")
		require.False(t, found)
	})

	t.Run("DeleteSession removes session", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		session := cli.SandboxSession{RunID: "run-123", ConfigFile: ".rwx/sandbox.yml"}
		storage.SetSession("/home/user/project", "main", ".rwx/sandbox.yml", session)

		storage.DeleteSession("/home/user/project", "main", ".rwx/sandbox.yml")

		_, found := storage.GetSession("/home/user/project", "main", ".rwx/sandbox.yml")
		require.False(t, found)
	})

	t.Run("DeleteSession is no-op when session does not exist", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		// Should not panic
		storage.DeleteSession("/home/user/project", "main", ".rwx/sandbox.yml")
		require.Empty(t, storage.Sandboxes)
	})
}

func TestSandboxStorage_GetSessionsForCwdBranch(t *testing.T) {
	t.Run("returns all sessions matching cwd and branch", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "main", "config1.yml", cli.SandboxSession{RunID: "run-1"})
		storage.SetSession("/home/user/project", "main", "config2.yml", cli.SandboxSession{RunID: "run-2"})
		storage.SetSession("/home/user/project", "develop", "config1.yml", cli.SandboxSession{RunID: "run-3"})
		storage.SetSession("/home/user/other", "main", "config1.yml", cli.SandboxSession{RunID: "run-4"})

		sessions := storage.GetSessionsForCwdBranch("/home/user/project", "main")
		require.Len(t, sessions, 2)

		runIDs := make([]string, len(sessions))
		for i, s := range sessions {
			runIDs[i] = s.RunID
		}
		require.ElementsMatch(t, []string{"run-1", "run-2"}, runIDs)
	})

	t.Run("returns empty slice when no matches", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "main", "config.yml", cli.SandboxSession{RunID: "run-1"})

		sessions := storage.GetSessionsForCwdBranch("/home/user/project", "develop")
		require.Empty(t, sessions)
	})

	t.Run("handles detached branch", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "", "config.yml", cli.SandboxSession{RunID: "run-1"})

		sessions := storage.GetSessionsForCwdBranch("/home/user/project", "")
		require.Len(t, sessions, 1)
		require.Equal(t, "run-1", sessions[0].RunID)
	})
}

func TestSandboxStorage_FindByRunID(t *testing.T) {
	t.Run("finds session by run ID", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "main", "config.yml", cli.SandboxSession{
			RunID:      "run-123",
			ConfigFile: "config.yml",
		})

		session, key, found := storage.FindByRunID("run-123")
		require.True(t, found)
		require.Equal(t, "run-123", session.RunID)
		require.Equal(t, "/home/user/project:main:config.yml", key)
	})

	t.Run("returns false when run ID not found", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "main", "config.yml", cli.SandboxSession{RunID: "run-123"})

		_, _, found := storage.FindByRunID("run-456")
		require.False(t, found)
	})
}

func TestSandboxStorage_DeleteSessionByRunID(t *testing.T) {
	t.Run("deletes session and returns true", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/home/user/project", "main", "config.yml", cli.SandboxSession{RunID: "run-123"})

		deleted := storage.DeleteSessionByRunID("run-123")
		require.True(t, deleted)

		_, found := storage.GetSession("/home/user/project", "main", "config.yml")
		require.False(t, found)
	})

	t.Run("returns false when run ID not found", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		deleted := storage.DeleteSessionByRunID("run-456")
		require.False(t, deleted)
	})
}

func TestSandboxStorage_AllSessions(t *testing.T) {
	t.Run("returns all sessions", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		storage.SetSession("/project1", "main", "config.yml", cli.SandboxSession{RunID: "run-1"})
		storage.SetSession("/project2", "develop", "config.yml", cli.SandboxSession{RunID: "run-2"})

		all := storage.AllSessions()
		require.Len(t, all, 2)
	})

	t.Run("returns empty map when no sessions", func(t *testing.T) {
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		all := storage.AllSessions()
		require.Empty(t, all)
	})
}

func setupTestStorageDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "sandbox-storage-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Override HOME so LoadSandboxStorage uses our temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })

	return tmpDir
}

func TestSandboxStorage_LoadAndSave(t *testing.T) {
	t.Run("returns empty storage when file does not exist", func(t *testing.T) {
		setupTestStorageDir(t)

		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		require.NotNil(t, storage)
		require.Empty(t, storage.Sandboxes)
	})

	t.Run("saves and loads storage", func(t *testing.T) {
		tmpDir := setupTestStorageDir(t)

		// Create and save storage
		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}
		storage.SetSession("/home/user/project", "main", ".rwx/sandbox.yml", cli.SandboxSession{
			RunID:      "run-123",
			ConfigFile: ".rwx/sandbox.yml",
		})

		err := storage.Save()
		require.NoError(t, err)

		// Verify file was created
		storagePath := filepath.Join(tmpDir, ".config", "rwx", "sandboxes.json")
		_, err = os.Stat(storagePath)
		require.NoError(t, err)

		// Load and verify
		loaded, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		require.Len(t, loaded.Sandboxes, 1)

		session, found := loaded.GetSession("/home/user/project", "main", ".rwx/sandbox.yml")
		require.True(t, found)
		require.Equal(t, "run-123", session.RunID)
	})

	t.Run("creates directory structure if it does not exist", func(t *testing.T) {
		tmpDir := setupTestStorageDir(t)

		storage := &cli.SandboxStorage{
			Sandboxes: make(map[string]cli.SandboxSession),
		}

		err := storage.Save()
		require.NoError(t, err)

		// Verify directory was created
		configDir := filepath.Join(tmpDir, ".config", "rwx")
		info, err := os.Stat(configDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})

	t.Run("handles nil Sandboxes map in stored file", func(t *testing.T) {
		tmpDir := setupTestStorageDir(t)

		// Write a file with null sandboxes
		configDir := filepath.Join(tmpDir, ".config", "rwx")
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		storagePath := filepath.Join(configDir, "sandboxes.json")
		require.NoError(t, os.WriteFile(storagePath, []byte(`{"sandboxes": null}`), 0o644))

		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		require.NotNil(t, storage.Sandboxes)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := setupTestStorageDir(t)

		configDir := filepath.Join(tmpDir, ".config", "rwx")
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		storagePath := filepath.Join(configDir, "sandboxes.json")
		require.NoError(t, os.WriteFile(storagePath, []byte(`{invalid json`), 0o644))

		_, err := cli.LoadSandboxStorage()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to parse")
	})
}
