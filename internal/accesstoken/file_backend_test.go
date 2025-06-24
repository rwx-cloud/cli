package accesstoken_test

import (
	"io"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/stretchr/testify/require"
)

func TestFileBackend_Get(t *testing.T) {
	t.Run("when there is only a single directory", func(t *testing.T) {
		t.Run("when the access token file does not exist", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "", token)
		})

		t.Run("when the access token file is otherwise unable to be opened", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			require.NoError(t, os.Chmod(primaryTmpDir, 0o000))

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to open")
			require.ErrorIs(t, err, fs.ErrPermission)
			require.Equal(t, "", token)
		})

		t.Run("when the access token file is present and has contents", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("the-token"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)
		})

		t.Run("when the access token file includes leading or trailing whitespace", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)
		})
	})

	t.Run("when there are multiple directories", func(t *testing.T) {
		t.Run("when the access token file does not exist in either directory", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "", token)
		})

		t.Run("when the access token file exists in the primary but not the fallback", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("the-token"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)

			_, err = os.Stat(path.Join(fallbackTmpDir, "accesstoken"))
			require.True(t, os.IsNotExist(err))
		})

		t.Run("when the access token file exists in the fallback but not the primary", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(fallbackTmpDir, "accesstoken"), []byte("the-token"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)

			file, err := os.Open(path.Join(primaryTmpDir, "accesstoken"))
			require.NoError(t, err)
			defer file.Close()
			bytes, err := io.ReadAll(file)
			require.NoError(t, err)
			require.Equal(t, "the-token", string(bytes))
		})

		t.Run("when the access token file in the primary dir is otherwise unable to be opened", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			require.NoError(t, os.Chmod(primaryTmpDir, 0o000))

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to open")
			require.ErrorIs(t, err, fs.ErrPermission)
			require.Equal(t, "", token)
		})

		t.Run("when the access token file in the primary dir includes leading or trailing whitespace", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)
		})

		t.Run("when the access token file in the fallback dir is otherwise unable to be opened", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			require.NoError(t, os.Chmod(fallbackTmpDir, 0o000))

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to open")
			require.ErrorIs(t, err, fs.ErrPermission)
			require.Equal(t, "", token)
		})

		t.Run("when the access token file in the fallback dir includes leading or trailing whitespace", func(t *testing.T) {
			primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

			fallbackTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-fallback")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(fallbackTmpDir) })

			err = os.WriteFile(path.Join(fallbackTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
			require.NoError(t, err)

			backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
			require.NoError(t, err)

			token, err := backend.Get()
			require.NoError(t, err)
			require.Equal(t, "the-token", token)
		})
	})
}

func TestFileBackend_Set(t *testing.T) {
	t.Run("when creating the file errors", func(t *testing.T) {
		primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

		require.NoError(t, os.Chmod(primaryTmpDir, 0o400))

		backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
		require.NoError(t, err)

		err = backend.Set("the-token")
		require.Contains(t, err.Error(), "permission denied")
		require.ErrorIs(t, err, fs.ErrPermission)
	})

	t.Run("when the file is created", func(t *testing.T) {
		primaryTmpDir, err := os.MkdirTemp(os.TempDir(), "file-backend-primary")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(primaryTmpDir) })

		backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
		require.NoError(t, err)

		err = backend.Set("the-token")
		require.NoError(t, err)

		file, err := os.Open(path.Join(primaryTmpDir, "accesstoken"))
		require.NoError(t, err)

		bytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "the-token", string(bytes))
	})
}
