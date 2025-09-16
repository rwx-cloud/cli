package oci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWWWAuthenticate(t *testing.T) {
	t.Run("extracts the scheme and params for basic auth", func(t *testing.T) {
		header, err := parseWWWAuthenticate(`Basic realm="my-realm"`)
		require.NoError(t, err)
		require.Equal(t, SchemeBasic, header.Scheme)
		require.Equal(t, map[string]string{"realm": "my-realm"}, header.Params)
	})

	t.Run("is case insensitive for the basic scheme", func(t *testing.T) {
		header, err := parseWWWAuthenticate(`bAsIc realm="my-realm"`)
		require.NoError(t, err)
		require.Equal(t, SchemeBasic, header.Scheme)
		require.Equal(t, map[string]string{"realm": "my-realm"}, header.Params)
	})

	t.Run("extracts the scheme and params for bearer auth", func(t *testing.T) {
		header, err := parseWWWAuthenticate(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/alpine:pull"`)
		require.NoError(t, err)
		require.Equal(t, SchemeBearer, header.Scheme)
		require.Equal(t, map[string]string{
			"realm":   "https://auth.docker.io/token",
			"service": "registry.docker.io",
			"scope":   "repository:library/alpine:pull",
		}, header.Params)
	})

	t.Run("is case insensitive for the bearer scheme", func(t *testing.T) {
		header, err := parseWWWAuthenticate(`bEaReR realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/alpine:pull"`)
		require.NoError(t, err)
		require.Equal(t, SchemeBearer, header.Scheme)
		require.Equal(t, map[string]string{
			"realm":   "https://auth.docker.io/token",
			"service": "registry.docker.io",
			"scope":   "repository:library/alpine:pull",
		}, header.Params)
	})

	t.Run("returns an error for unsupported schemes", func(t *testing.T) {
		_, err := parseWWWAuthenticate(`Digest realm="my-realm"`)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported authentication scheme")
	})

	t.Run("returns an error for invalid headers", func(t *testing.T) {
		_, err := parseWWWAuthenticate(`InvalidHeader`)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid WWW-Authenticate header")
	})

	t.Run("returns an error for invalid params", func(t *testing.T) {
		_, err := parseWWWAuthenticate(`Basic realm`)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid parameter")
	})
}
