package docs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_resolveURL(t *testing.T) {
	c := Client{Host: "www.rwx.com", Scheme: "https"}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare path",
			input:    "/docs/rwx/guides/ci",
			expected: "https://www.rwx.com/docs/rwx/guides/ci",
		},
		{
			name:     "full URL with https scheme",
			input:    "https://www.rwx.com/docs/rwx/guides/ci",
			expected: "https://www.rwx.com/docs/rwx/guides/ci",
		},
		{
			name:     "full URL with http scheme",
			input:    "http://www.rwx.com/docs/rwx/guides/ci",
			expected: "http://www.rwx.com/docs/rwx/guides/ci",
		},
		{
			name:     "host with www but no scheme",
			input:    "www.rwx.com/docs/rwx/guides/ci",
			expected: "https://www.rwx.com/docs/rwx/guides/ci",
		},
		{
			name:     "host without www or scheme",
			input:    "rwx.com/docs/rwx/guides/ci",
			expected: "https://rwx.com/docs/rwx/guides/ci",
		},
		{
			name:     "bare path without docs prefix",
			input:    "/rwx/get-started",
			expected: "https://www.rwx.com/docs/rwx/get-started",
		},
		{
			name:     "bare root path without docs prefix",
			input:    "/rwx/guides/ci",
			expected: "https://www.rwx.com/docs/rwx/guides/ci",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.resolveURL(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_resolveURL_customHost(t *testing.T) {
	c := Client{Host: "docs.example.com", Scheme: "http"}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare path with custom host prepends docs prefix",
			input:    "/some/article",
			expected: "http://docs.example.com/docs/some/article",
		},
		{
			name:     "full URL ignores custom host",
			input:    "https://other.com/some/article",
			expected: "https://other.com/some/article",
		},
		{
			name:     "custom host without scheme",
			input:    "docs.example.com/some/article",
			expected: "http://docs.example.com/some/article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.resolveURL(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_Search_sendsDocsTokenHeader(t *testing.T) {
	t.Run("when docs token is set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "my-docs-token", r.Header.Get("X-RWX-Docs-Token"))
			fmt.Fprint(w, `{"query":"test","totalHits":0,"results":[]}`)
		}))
		t.Cleanup(server.Close)

		c := Client{
			Host:      server.Listener.Addr().String(),
			Scheme:    "http",
			DocsToken: "my-docs-token",
		}

		_, err := c.Search("test", 5)
		require.NoError(t, err)
	})

	t.Run("when docs token is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Empty(t, r.Header.Get("X-RWX-Docs-Token"))
			fmt.Fprint(w, `{"query":"test","totalHits":0,"results":[]}`)
		}))
		t.Cleanup(server.Close)

		c := Client{Host: server.Listener.Addr().String(), Scheme: "http"}

		_, err := c.Search("test", 5)
		require.NoError(t, err)
	})
}

func TestClient_FetchArticle_sendsDocsTokenHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "my-docs-token", r.Header.Get("X-RWX-Docs-Token"))
		fmt.Fprint(w, "# Article")
	}))
	t.Cleanup(server.Close)

	c := Client{
		Host:      server.Listener.Addr().String(),
		Scheme:    "http",
		DocsToken: "my-docs-token",
	}

	body, err := c.FetchArticle("/docs/test")
	require.NoError(t, err)
	require.Equal(t, "# Article", body)
}

func TestClient_FetchArticle_withFullURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/rwx/guides/ci", r.URL.Path)
		require.Equal(t, "text/markdown", r.Header.Get("Accept"))
		fmt.Fprint(w, "# CI Guide\n\nWelcome.")
	}))
	t.Cleanup(server.Close)

	c := Client{Host: server.Listener.Addr().String(), Scheme: "http"}

	// Pass a full URL pointing at the test server
	fullURL := fmt.Sprintf("http://%s/docs/rwx/guides/ci", server.Listener.Addr().String())
	body, err := c.FetchArticle(fullURL)
	require.NoError(t, err)
	require.Equal(t, "# CI Guide\n\nWelcome.", body)
}

func TestClient_FetchArticle_withPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/rwx/guides/ci", r.URL.Path)
		require.Equal(t, "text/markdown", r.Header.Get("Accept"))
		fmt.Fprint(w, "# CI Guide\n\nWelcome.")
	}))
	t.Cleanup(server.Close)

	c := Client{Host: server.Listener.Addr().String(), Scheme: "http"}

	body, err := c.FetchArticle("/docs/rwx/guides/ci")
	require.NoError(t, err)
	require.Equal(t, "# CI Guide\n\nWelcome.", body)
}

func TestClient_FetchArticle_withPathWithoutDocsPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/rwx/get-started", r.URL.Path)
		require.Equal(t, "text/markdown", r.Header.Get("Accept"))
		fmt.Fprint(w, "# Get Started\n\nWelcome to RWX.")
	}))
	t.Cleanup(server.Close)

	c := Client{Host: server.Listener.Addr().String(), Scheme: "http"}

	body, err := c.FetchArticle("/rwx/get-started")
	require.NoError(t, err)
	require.Equal(t, "# Get Started\n\nWelcome to RWX.", body)
}

func TestClient_FetchArticle_withHostNoScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/docs/rwx/guides/ci", r.URL.Path)
		fmt.Fprint(w, "# CI Guide\n\nWelcome.")
	}))
	t.Cleanup(server.Close)

	c := Client{Host: server.Listener.Addr().String(), Scheme: "http"}

	// Pass host-prefixed input (without scheme)
	hostPrefixed := fmt.Sprintf("%s/docs/rwx/guides/ci", server.Listener.Addr().String())
	body, err := c.FetchArticle(hostPrefixed)
	require.NoError(t, err)
	require.Equal(t, "# CI Guide\n\nWelcome.", body)
}
