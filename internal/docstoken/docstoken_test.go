package docstoken_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/config"
	"github.com/rwx-cloud/cli/internal/docstoken"
	"github.com/stretchr/testify/require"
)

func TestFileBackend_GetAndSet(t *testing.T) {
	t.Run("when no docs token has been stored", func(t *testing.T) {
		backend := docstoken.NewFileBackend(config.NewMemoryBackend())

		dt, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, docstoken.DocsToken{}, dt)
	})

	t.Run("when docs token has been stored with token and auth token", func(t *testing.T) {
		backend := docstoken.NewFileBackend(config.NewMemoryBackend())

		err := backend.Set(docstoken.DocsToken{
			Token:     "docs-token-123",
			AuthToken: "auth-token-456",
		})
		require.NoError(t, err)

		dt, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "docs-token-123", dt.Token)
		require.Equal(t, "auth-token-456", dt.AuthToken)
	})

	t.Run("when docs token has been stored with token only", func(t *testing.T) {
		backend := docstoken.NewFileBackend(config.NewMemoryBackend())

		err := backend.Set(docstoken.DocsToken{
			Token: "docs-token-123",
		})
		require.NoError(t, err)

		dt, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "docs-token-123", dt.Token)
		require.Equal(t, "", dt.AuthToken)
	})
}

func TestMemoryBackend_GetAndSet(t *testing.T) {
	t.Run("when no docs token has been stored", func(t *testing.T) {
		backend := docstoken.NewMemoryBackend()

		dt, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, docstoken.DocsToken{}, dt)
	})

	t.Run("when docs token has been stored", func(t *testing.T) {
		backend := docstoken.NewMemoryBackend()

		err := backend.Set(docstoken.DocsToken{
			Token:     "docs-token-123",
			AuthToken: "auth-token-456",
		})
		require.NoError(t, err)

		dt, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "docs-token-123", dt.Token)
		require.Equal(t, "auth-token-456", dt.AuthToken)
	})
}
