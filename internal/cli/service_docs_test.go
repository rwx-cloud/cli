package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/docs"
	"github.com/stretchr/testify/require"
)

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

func setupDocsServer(t *testing.T, handler http.Handler) *testSetup {
	t.Helper()
	s := setupTest(t)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	s.config.DocsClient = docs.Client{Host: server.Listener.Addr().String(), Scheme: "http"}
	var err error
	s.service, err = cli.NewService(s.config)
	require.NoError(t, err)

	return s
}

func TestService_DocsSearch(t *testing.T) {
	t.Run("when no results are found", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/docs/api/search", r.URL.Path)
			require.Equal(t, "nothing", r.URL.Query().Get("q"))

			writeJSON(t, w, docs.SearchResponse{
				Query:     "nothing",
				TotalHits: 0,
				Results:   []docs.SearchResult{},
			})
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "nothing",
			Limit: 5,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.TotalHits)
		require.Empty(t, result.Results)
		require.Contains(t, s.mockStdout.String(), `No results found for "nothing"`)
	})

	t.Run("when a single result is returned with body", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, docs.SearchResponse{
				Query:     "flaky tests",
				TotalHits: 1,
				Results: []docs.SearchResult{
					{
						Title: "Flaky Test Detection",
						URL:   "https://www.rwx.com/docs/captain/flaky-tests",
						Path:  "/captain/flaky-tests",
						Body:  "# Flaky Test Detection\n\nLearn how to detect flaky tests.",
					},
				},
			})
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "flaky tests",
			Limit: 5,
		})

		require.NoError(t, err)
		require.Equal(t, 1, result.TotalHits)
		require.Len(t, result.Results, 1)
		require.Contains(t, s.mockStdout.String(), "# Flaky Test Detection")
		require.Contains(t, s.mockStdout.String(), "Learn how to detect flaky tests.")
	})

	t.Run("when a single result is returned without body, fetches article", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/docs/api/search":
				writeJSON(t, w, docs.SearchResponse{
					Query:     "flaky tests",
					TotalHits: 1,
					Results: []docs.SearchResult{
						{
							Title: "Flaky Test Detection",
							URL:   "https://www.rwx.com/docs/captain/flaky-tests",
							Path:  "/captain/flaky-tests",
							Body:  "",
						},
					},
				})
			case "/captain/flaky-tests":
				require.Equal(t, "text/markdown", r.Header.Get("Accept"))
				fmt.Fprint(w, "# Flaky Test Detection\n\nFull article body.")
			default:
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "flaky tests",
			Limit: 5,
		})

		require.NoError(t, err)
		require.Len(t, result.Results, 1)
		require.Contains(t, s.mockStdout.String(), "Full article body.")
	})

	t.Run("when a single result is returned with json output", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, docs.SearchResponse{
				Query:     "flaky tests",
				TotalHits: 1,
				Results: []docs.SearchResult{
					{
						Title: "Flaky Test Detection",
						URL:   "https://www.rwx.com/docs/captain/flaky-tests",
						Path:  "/captain/flaky-tests",
						Body:  "# Flaky Test Detection",
					},
				},
			})
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "flaky tests",
			Limit: 5,
			Json:  true,
		})

		require.NoError(t, err)
		require.Equal(t, 1, result.TotalHits)
		require.Len(t, result.Results, 1)
		// JSON mode: service should not print body to stdout
		require.Empty(t, s.mockStdout.String())
	})

	t.Run("when multiple results are returned in non-TTY mode", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, docs.SearchResponse{
				Query:     "caching",
				TotalHits: 2,
				Results: []docs.SearchResult{
					{
						Title:   "Content Caching",
						URL:     "https://www.rwx.com/docs/mint/caching",
						Path:    "/mint/caching",
						Snippet: "Learn about content-based caching.",
					},
					{
						Title:   "Cache Configuration",
						URL:     "https://www.rwx.com/docs/mint/cache-config",
						Path:    "/mint/cache-config",
						Snippet: "Configure cache behavior.",
					},
				},
			})
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query:       "caching",
			Limit:       5,
			StdoutIsTTY: false,
		})

		require.NoError(t, err)
		require.Equal(t, 2, result.TotalHits)
		require.Len(t, result.Results, 2)

		output := s.mockStdout.String()
		require.Contains(t, output, "Content Caching")
		require.Contains(t, output, "https://www.rwx.com/docs/mint/caching")
		require.Contains(t, output, "Learn about content-based caching.")
		require.Contains(t, output, "Cache Configuration")
		require.Contains(t, output, "https://www.rwx.com/docs/mint/cache-config")
		require.Contains(t, output, "Configure cache behavior.")
	})

	t.Run("when multiple results are returned in non-TTY json mode", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, docs.SearchResponse{
				Query:     "caching",
				TotalHits: 2,
				Results: []docs.SearchResult{
					{
						Title:   "Content Caching",
						URL:     "https://www.rwx.com/docs/mint/caching",
						Path:    "/mint/caching",
						Snippet: "Learn about content-based caching.",
					},
					{
						Title:   "Cache Configuration",
						URL:     "https://www.rwx.com/docs/mint/cache-config",
						Path:    "/mint/cache-config",
						Snippet: "Configure cache behavior.",
					},
				},
			})
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query:       "caching",
			Limit:       5,
			Json:        true,
			StdoutIsTTY: false,
		})

		require.NoError(t, err)
		require.Equal(t, 2, result.TotalHits)
		require.Len(t, result.Results, 2)
		// JSON mode: service should not print list to stdout
		require.Empty(t, s.mockStdout.String())
	})

	t.Run("when multiple results are returned in TTY mode and user selects one", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/docs/api/search":
				writeJSON(t, w, docs.SearchResponse{
					Query:     "caching",
					TotalHits: 2,
					Results: []docs.SearchResult{
						{
							Title:   "Content Caching",
							URL:     "https://www.rwx.com/docs/mint/caching",
							Path:    "/mint/caching",
							Snippet: "Learn about content-based caching.",
						},
						{
							Title:   "Cache Configuration",
							URL:     "https://www.rwx.com/docs/mint/cache-config",
							Path:    "/mint/cache-config",
							Snippet: "Configure cache behavior.",
						},
					},
				})
			case "/mint/cache-config":
				fmt.Fprint(w, "# Cache Configuration\n\nFull article about cache config.")
			default:
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
		}))

		s.mockStdin.WriteString("2\n")

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query:       "caching",
			Limit:       5,
			StdoutIsTTY: true,
		})

		require.NoError(t, err)
		require.Equal(t, 2, result.TotalHits)
		require.Len(t, result.Results, 2)

		output := s.mockStdout.String()
		require.Contains(t, output, "1. Content Caching")
		require.Contains(t, output, "2. Cache Configuration")
		require.Contains(t, output, "Full article about cache config.")
	})

	t.Run("when the search API returns an error", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "internal server error")
		}))

		result, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "test",
			Limit: 5,
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "500")
	})

	t.Run("when the limit query parameter is passed through", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "3", r.URL.Query().Get("limit"))

			writeJSON(t, w, docs.SearchResponse{
				Query:     "test",
				TotalHits: 0,
				Results:   []docs.SearchResult{},
			})
		}))

		_, err := s.service.DocsSearch(cli.DocsSearchConfig{
			Query: "test",
			Limit: 3,
		})

		require.NoError(t, err)
	})
}

func TestService_DocsPull(t *testing.T) {
	t.Run("when the article is found", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/captain/getting-started", r.URL.Path)
			require.Equal(t, "text/markdown", r.Header.Get("Accept"))

			fmt.Fprint(w, "# Getting Started\n\nWelcome to Captain.")
		}))

		result, err := s.service.DocsPull(cli.DocsPullConfig{
			URL: "/captain/getting-started",
		})

		require.NoError(t, err)
		require.Equal(t, "/captain/getting-started", result.URL)
		require.Contains(t, s.mockStdout.String(), "# Getting Started")
		require.Contains(t, s.mockStdout.String(), "Welcome to Captain.")
	})

	t.Run("when the article is found with json output", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "# Getting Started\n\nWelcome to Captain.")
		}))

		result, err := s.service.DocsPull(cli.DocsPullConfig{
			URL:  "/captain/getting-started",
			Json: true,
		})

		require.NoError(t, err)
		require.Equal(t, "# Getting Started\n\nWelcome to Captain.", result.Body)
		// JSON mode: service should not print body to stdout
		require.Empty(t, s.mockStdout.String())
	})

	t.Run("when the article API returns an error", func(t *testing.T) {
		s := setupDocsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "not found")
		}))

		result, err := s.service.DocsPull(cli.DocsPullConfig{
			URL: "/nonexistent",
		})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "article not found")
	})
}
