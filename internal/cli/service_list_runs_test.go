package cli_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_ListRuns(t *testing.T) {
	t.Run("calls API client with correct config", func(t *testing.T) {
		s := setupTest(t)

		expectedRuns := []api.RunOverview{
			{
				ID:             "run-1",
				RepositoryName: stringPtr("test-repo"),
				Branch:         stringPtr("main"),
				Title:          stringPtr("Test Run"),
			},
		}

		s.mockAPI.MockListRuns = func(cfg api.ListRunsConfig) (*api.ListRunsResult, error) {
			require.Equal(t, []string{"repo1"}, cfg.RepositoryNames)
			require.Equal(t, []string{"main"}, cfg.BranchNames)
			require.Equal(t, []string{"author1"}, cfg.Authors)
			require.Equal(t, "2024-01-01", cfg.StartDate)
			require.True(t, cfg.MyRuns)

			return &api.ListRunsResult{Runs: expectedRuns}, nil
		}

		result, err := s.service.ListRuns(cli.ListRunsConfig{
			RepositoryNames: []string{"repo1"},
			BranchNames:     []string{"main"},
			Authors:         []string{"author1"},
			StartDate:       "2024-01-01",
			MyRuns:          true,
		})

		require.NoError(t, err)
		require.Len(t, result.Runs, 1)
		require.Equal(t, "run-1", result.Runs[0].ID)
	})

	t.Run("handles API errors", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockListRuns = func(cfg api.ListRunsConfig) (*api.ListRunsResult, error) {
			return nil, api.ErrNotFound
		}

		_, err := s.service.ListRuns(cli.ListRunsConfig{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to list runs")
	})

	t.Run("handles empty results", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockListRuns = func(cfg api.ListRunsConfig) (*api.ListRunsResult, error) {
			return &api.ListRunsResult{Runs: []api.RunOverview{}}, nil
		}

		result, err := s.service.ListRuns(cli.ListRunsConfig{})
		require.NoError(t, err)
		require.Len(t, result.Runs, 0)
	})
}

func stringPtr(s string) *string {
	return &s
}
