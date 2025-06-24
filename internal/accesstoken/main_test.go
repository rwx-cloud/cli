package accesstoken_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	t.Run("prefers the provided access token", func(t *testing.T) {
		backend, err := accesstoken.NewMemoryBackend()
		require.NoError(t, err)

		err = backend.Set("other-token")
		require.NoError(t, err)

		token, err := accesstoken.Get(backend, "provided-token")
		require.NoError(t, err)
		require.Equal(t, "provided-token", token)
	})

	t.Run("falls back to the stored access token", func(t *testing.T) {
		backend, err := accesstoken.NewMemoryBackend()
		require.NoError(t, err)

		err = backend.Set("other-token")
		require.NoError(t, err)

		token, err := accesstoken.Get(backend, "")
		require.NoError(t, err)
		require.Equal(t, "other-token", token)
	})
}

func TestSet(t *testing.T) {
	t.Run("stores the token in the backend", func(t *testing.T) {
		backend, err := accesstoken.NewMemoryBackend()
		require.NoError(t, err)

		err = accesstoken.Set(backend, "some-token")
		require.NoError(t, err)

		token, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "some-token", token)
	})
}
