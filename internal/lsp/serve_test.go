package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindNode_ReturnsErrorWhenNotOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := findNode()
	require.Error(t, err)
	require.Contains(t, err.Error(), "node is required but was not found on PATH")
}

func TestBundleHash_ReturnsDeterministicNonEmptyHash(t *testing.T) {
	hash1, err := bundleHash()
	require.NoError(t, err)
	require.NotEmpty(t, hash1)

	hash2, err := bundleHash()
	require.NoError(t, err)
	require.Equal(t, hash1, hash2)
}

func TestCleanStaleBundles_RemovesOldVersions(t *testing.T) {
	parentDir := t.TempDir()

	// Create a "current" bundle directory and two stale ones
	currentName := "v1.0.0-currenthash"
	require.NoError(t, os.MkdirAll(filepath.Join(parentDir, currentName), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(parentDir, "v0.9.0-oldhash"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(parentDir, "v0.8.0-olderhash"), 0o755))

	cleanStaleBundles(parentDir, currentName)

	entries, err := os.ReadDir(parentDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, currentName, entries[0].Name())
}

func TestCleanStaleBundles_NoErrorOnMissingDir(t *testing.T) {
	// Should not panic or error when the parent directory doesn't exist
	cleanStaleBundles("/nonexistent/path", "current")
}

func TestEnsureBundle_ExtractsAndCachesBundle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	serverJS, err := ensureBundle()
	require.NoError(t, err)
	require.Contains(t, serverJS, "server.js")

	// serverJS is at <cache-dir>/bundle/server.js — go up 3 levels
	cacheDir := filepath.Dir(filepath.Dir(serverJS))
	markerFile := filepath.Join(cacheDir, ".extracted")
	_, err = os.Stat(markerFile)
	require.NoError(t, err, "marker file should exist after extraction")

	// Second call should be a cache hit (no error, same result)
	serverJS2, err := ensureBundle()
	require.NoError(t, err)
	require.Equal(t, serverJS, serverJS2)
}
