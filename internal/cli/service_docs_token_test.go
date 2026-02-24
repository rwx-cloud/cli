package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/docstoken"
	"github.com/stretchr/testify/require"
)

func newDocsServer(t *testing.T, assertHeaders func(headers map[string]string)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := map[string]string{
			"X-Rwx-Docs-Token": r.Header.Get("X-RWX-Docs-Token"),
		}
		assertHeaders(headers)
		fmt.Fprint(w, "# Article")
	}))
	t.Cleanup(server.Close)
	return server
}

func TestResolveDocsToken(t *testing.T) {
	t.Run("returns empty when backends are nil", func(t *testing.T) {
		s := setupTest(t)

		server := newDocsServer(t, func(headers map[string]string) {
			require.Empty(t, headers["X-Rwx-Docs-Token"])
		})

		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)
	})

	t.Run("returns empty when no auth token is available", func(t *testing.T) {
		s := setupTest(t)

		s.config.DocsTokenBackend = docstoken.NewMemoryBackend()
		s.config.AccessTokenBackend = accesstoken.NewMemoryBackend()
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		// AccessTokenBackend returns empty string — no docs token header should be sent
		server := newDocsServer(t, func(headers map[string]string) {
			require.Empty(t, headers["X-Rwx-Docs-Token"])
		})

		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)
	})

	t.Run("uses cached docs token when auth token matches", func(t *testing.T) {
		s := setupTest(t)

		docsTokenBackend := docstoken.NewMemoryBackend()
		require.NoError(t, docsTokenBackend.Set(docstoken.DocsToken{
			Token:     "cached-docs-token",
			AuthToken: "current-auth-token",
		}))

		tokenBackend := accesstoken.NewMemoryBackend()
		require.NoError(t, tokenBackend.Set("current-auth-token"))

		server := newDocsServer(t, func(headers map[string]string) {
			require.Equal(t, "cached-docs-token", headers["X-Rwx-Docs-Token"])
		})

		s.config.DocsTokenBackend = docsTokenBackend
		s.config.AccessTokenBackend = tokenBackend
		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)
	})

	t.Run("fetches new docs token when auth token has changed", func(t *testing.T) {
		s := setupTest(t)

		docsTokenBackend := docstoken.NewMemoryBackend()
		require.NoError(t, docsTokenBackend.Set(docstoken.DocsToken{
			Token:     "old-docs-token",
			AuthToken: "old-auth-token",
		}))

		tokenBackend := accesstoken.NewMemoryBackend()
		require.NoError(t, tokenBackend.Set("new-auth-token"))

		s.mockAPI.MockCreateDocsToken = func() (*api.DocsTokenResult, error) {
			return &api.DocsTokenResult{Token: "fresh-docs-token"}, nil
		}

		server := newDocsServer(t, func(headers map[string]string) {
			require.Equal(t, "fresh-docs-token", headers["X-Rwx-Docs-Token"])
		})

		s.config.DocsTokenBackend = docsTokenBackend
		s.config.AccessTokenBackend = tokenBackend
		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)

		// Verify it was cached with the new auth token
		cached, err := docsTokenBackend.Get()
		require.NoError(t, err)
		require.Equal(t, "fresh-docs-token", cached.Token)
		require.Equal(t, "new-auth-token", cached.AuthToken)
	})

	t.Run("returns empty when API call fails", func(t *testing.T) {
		s := setupTest(t)

		tokenBackend := accesstoken.NewMemoryBackend()
		require.NoError(t, tokenBackend.Set("some-auth-token"))

		s.mockAPI.MockCreateDocsToken = func() (*api.DocsTokenResult, error) {
			return nil, fmt.Errorf("API error")
		}

		server := newDocsServer(t, func(headers map[string]string) {
			require.Empty(t, headers["X-Rwx-Docs-Token"])
		})

		s.config.DocsTokenBackend = docstoken.NewMemoryBackend()
		s.config.AccessTokenBackend = tokenBackend
		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)
	})

	t.Run("fetches new docs token when no cached token exists", func(t *testing.T) {
		s := setupTest(t)

		tokenBackend := accesstoken.NewMemoryBackend()
		require.NoError(t, tokenBackend.Set("my-auth-token"))

		s.mockAPI.MockCreateDocsToken = func() (*api.DocsTokenResult, error) {
			return &api.DocsTokenResult{Token: "new-docs-token"}, nil
		}

		server := newDocsServer(t, func(headers map[string]string) {
			require.Equal(t, "new-docs-token", headers["X-Rwx-Docs-Token"])
		})

		docsTokenBackend := docstoken.NewMemoryBackend()
		s.config.DocsTokenBackend = docsTokenBackend
		s.config.AccessTokenBackend = tokenBackend
		s.config.DocsClient.Host = server.Listener.Addr().String()
		s.config.DocsClient.Scheme = "http"
		var err error
		s.service, err = cli.NewService(s.config)
		require.NoError(t, err)

		_, err = s.service.DocsPull(cli.DocsPullConfig{URL: "/docs/test", Json: true})
		require.NoError(t, err)

		// Verify it was cached
		cached, err := docsTokenBackend.Get()
		require.NoError(t, err)
		require.Equal(t, "new-docs-token", cached.Token)
		require.Equal(t, "my-auth-token", cached.AuthToken)
	})
}
