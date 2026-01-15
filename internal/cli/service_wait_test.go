package cli_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_WaitForRun(t *testing.T) {
	t.Run("returns result when run completes immediately", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			require.Equal(t, "run-123", cfg.RunID)
			return api.RunStatusResult{
				Status:  &api.RunStatus{Result: "succeeded"},
				RunID:   "run-123",
				Polling: api.PollingResult{Completed: true},
			}, nil
		}

		result, err := setup.service.WaitForRun(cli.WaitForRunConfig{
			RunID: "run-123",
			Json:  false,
		})

		require.NoError(t, err)
		require.Equal(t, "run-123", result.RunID)
		require.Equal(t, "succeeded", result.ResultStatus)
	})

	t.Run("polls until run completes", func(t *testing.T) {
		setup := setupTest(t)

		callCount := 0
		backoffMs := 0
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			callCount++
			if callCount < 3 {
				return api.RunStatusResult{
					Status:  &api.RunStatus{Result: "in_progress"},
					RunID:   "run-456",
					Polling: api.PollingResult{Completed: false, BackoffMs: &backoffMs},
				}, nil
			}
			return api.RunStatusResult{
				Status:  &api.RunStatus{Result: "failed"},
				RunID:   "run-456",
				Polling: api.PollingResult{Completed: true},
			}, nil
		}

		result, err := setup.service.WaitForRun(cli.WaitForRunConfig{
			RunID: "run-456",
			Json:  false,
		})

		require.NoError(t, err)
		require.Equal(t, 3, callCount)
		require.Equal(t, "run-456", result.RunID)
		require.Equal(t, "failed", result.ResultStatus)
	})

	t.Run("returns error when backoff is nil and polling not completed", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				Status:  &api.RunStatus{Result: "in_progress"},
				RunID:   "run-789",
				Polling: api.PollingResult{Completed: false, BackoffMs: nil},
			}, nil
		}

		_, err := setup.service.WaitForRun(cli.WaitForRunConfig{
			RunID: "run-789",
			Json:  false,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to wait for run")
	})

	t.Run("returns empty status when run not found", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				Status:  nil,
				Polling: api.PollingResult{Completed: true},
			}, nil
		}

		result, err := setup.service.WaitForRun(cli.WaitForRunConfig{
			RunID: "nonexistent",
			Json:  false,
		})

		require.NoError(t, err)
		require.Equal(t, "nonexistent", result.RunID)
		require.Equal(t, "", result.ResultStatus)
	})

	t.Run("returns error when API call fails", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{}, api.ErrNotFound
		}

		_, err := setup.service.WaitForRun(cli.WaitForRunConfig{
			RunID: "run-123",
			Json:  false,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to get run status")
	})
}
