package config_test

import (
	"io"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/rwx-cloud/cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestFileBackend_Get(t *testing.T) {
	t.Run("when there is only a single directory", func(t *testing.T) {
		t.Run("when the file does not exist", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			backend, err := config.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "", value)
		})

		t.Run("when the file is otherwise unable to be opened", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			require.NoError(t, os.Chmod(primaryTmpDir, 0o000))

			backend, err := config.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to open")
			require.ErrorIs(t, err, fs.ErrPermission)
			require.Equal(t, "", value)
		})

		t.Run("when the file is present and has contents", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "testfile"), []byte("the-value"), 0o644)
			require.NoError(t, err)

			backend, err := config.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "the-value", value)
		})

		t.Run("when the file includes leading or trailing whitespace", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "testfile"), []byte("\n  \t  the-value\t  \n \n"), 0o644)
			require.NoError(t, err)

			backend, err := config.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "the-value", value)
		})
	})

	t.Run("when there are multiple directories", func(t *testing.T) {
		t.Run("when the file does not exist in either directory", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			backend, err := config.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "", value)
		})

		t.Run("when the file exists in the primary but not the fallback", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "testfile"), []byte("the-value"), 0o644)
			require.NoError(t, err)

			backend, err := config.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "the-value", value)

			_, err = os.Stat(path.Join(fallbackTmpDir, "testfile"))
			require.True(t, os.IsNotExist(err))
		})

		t.Run("when the file exists in the fallback but not the primary", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(fallbackTmpDir, "testfile"), []byte("the-value"), 0o644)
			require.NoError(t, err)

			backend, err := config.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			value, err := backend.Get("testfile")
			require.NoError(t, err)
			require.Equal(t, "the-value", value)

			file, err := os.Open(path.Join(primaryTmpDir, "testfile"))
			require.NoError(t, err)
			defer file.Close()
			bytes, err := io.ReadAll(file)
			require.NoError(t, err)
			require.Equal(t, "the-value", string(bytes))
		})
	})
}

func TestFileBackend_Set(t *testing.T) {
	t.Run("when creating the file errors", func(t *testing.T) {
		primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

		require.NoError(t, os.Chmod(primaryTmpDir, 0o400))

		backend, err := config.NewFileBackend([]string{primaryTmpDir})
		require.NoError(t, err)

		err = backend.Set("testfile", "the-value")
		require.Contains(t, err.Error(), "permission denied")
		require.ErrorIs(t, err, fs.ErrPermission)
	})

	t.Run("when the file is created", func(t *testing.T) {
		primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

		backend, err := config.NewFileBackend([]string{primaryTmpDir})
		require.NoError(t, err)

		err = backend.Set("testfile", "the-value")
		require.NoError(t, err)

		file, err := os.Open(path.Join(primaryTmpDir, "testfile"))
		require.NoError(t, err)

		bytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "the-value", string(bytes))
	})
}
