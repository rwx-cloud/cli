package cli_test

import (
	"testing"

	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_GetRunPrompt(t *testing.T) {
	t.Run("returns prompt from API", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(cfg api.GetRunPromptConfig) (api.GetRunPromptResult, error) {
			require.Equal(t, "run-123", cfg.RunID)
			return api.GetRunPromptResult{Prompt: "prompt text"}, nil
		}

		result, err := setup.service.GetRunPrompt(cli.GetRunPromptConfig{RunID: "run-123"})

		require.NoError(t, err)
		require.Equal(t, "prompt text", result.Prompt)
	})

	t.Run("returns error when API fails", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(cfg api.GetRunPromptConfig) (api.GetRunPromptResult, error) {
			return api.GetRunPromptResult{}, api.ErrNotFound
		}

		result, err := setup.service.GetRunPrompt(cli.GetRunPromptConfig{RunID: "run-123"})

		require.Nil(t, result)
		require.Error(t, err)
	})

	t.Run("passes all and json flags to API", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(cfg api.GetRunPromptConfig) (api.GetRunPromptResult, error) {
			require.Equal(t, "run-456", cfg.RunID)
			require.True(t, cfg.All)
			require.True(t, cfg.Json)
			return api.GetRunPromptResult{
				Tasks: []api.RunPromptTask{
					{Key: "ci.lint", Status: "succeeded"},
					{Key: "ci.test", Status: "failed"},
				},
			}, nil
		}

		result, err := setup.service.GetRunPrompt(cli.GetRunPromptConfig{
			RunID: "run-456",
			All:   true,
			Json:  true,
		})

		require.NoError(t, err)
		require.Len(t, result.Tasks, 2)
		require.Equal(t, "ci.lint", result.Tasks[0].Key)
		require.Equal(t, "succeeded", result.Tasks[0].Status)
	})

	t.Run("passes page to API and returns pagination fields", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetRunPrompt = func(cfg api.GetRunPromptConfig) (api.GetRunPromptResult, error) {
			require.Equal(t, 3, cfg.Page)
			return api.GetRunPromptResult{
				Tasks:   []api.RunPromptTask{{Key: "ci.lint", Status: "succeeded"}},
				Page:    3,
				HasMore: true,
			}, nil
		}

		result, err := setup.service.GetRunPrompt(cli.GetRunPromptConfig{
			RunID: "run-789",
			All:   true,
			Json:  true,
			Page:  3,
		})

		require.NoError(t, err)
		require.Equal(t, 3, result.Page)
		require.True(t, result.HasMore)
	})
}
