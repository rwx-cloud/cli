package accesstoken_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend_GetSet(t *testing.T) {
	t.Run("sets and gets tokens", func(t *testing.T) {
		backend, err := accesstoken.NewMemoryBackend()
		require.NoError(t, err)

		token, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "", token)

		err = backend.Set("the-token")
		require.NoError(t, err)

		token, err = backend.Get()
		require.NoError(t, err)
		require.Equal(t, "the-token", token)
	})
}
