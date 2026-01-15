package cli_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/stretchr/testify/require"
)

func TestService_GetRunPrompt(t *testing.T) {
	t.Run("returns prompt from API", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(runID string) (string, error) {
			require.Equal(t, "run-123", runID)
			return "prompt text", nil
		}

		result, err := setup.service.GetRunPrompt("run-123")

		require.NoError(t, err)
		require.Equal(t, "prompt text", result)
	})

	t.Run("returns error when API fails", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(runID string) (string, error) {
			return "", api.ErrNotFound
		}

		_, err := setup.service.GetRunPrompt("run-123")

		require.Error(t, err)
	})
}
