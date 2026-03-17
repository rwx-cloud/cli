package cli_test

import (
	"testing"

	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/stretchr/testify/require"
)

func TestResolveRunIDFromGitContext(t *testing.T) {
	t.Run("resolves run ID from branch and repository", func(t *testing.T) {
		setup := setupTest(t)
		setup.mockGit.MockGetBranch = "my-branch"
		setup.mockGit.MockGetOriginUrl = "git@github.com:rwx-cloud/rwx.git"
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			require.Equal(t, "my-branch", cfg.BranchName)
			require.Equal(t, "rwx", cfg.RepositoryName)
			return api.RunStatusResult{RunID: "run-abc123"}, nil
		}

		runID, err := setup.service.ResolveRunIDFromGitContext()
		require.NoError(t, err)
		require.Equal(t, "run-abc123", runID)
	})

	t.Run("returns error when branch is empty", func(t *testing.T) {
		setup := setupTest(t)
		setup.mockGit.MockGetBranch = ""
		setup.mockGit.MockGetOriginUrl = "git@github.com:rwx-cloud/rwx.git"

		_, err := setup.service.ResolveRunIDFromGitContext()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to determine the current branch and repository from git")
	})

	t.Run("returns error when origin URL is empty", func(t *testing.T) {
		setup := setupTest(t)
		setup.mockGit.MockGetBranch = "my-branch"
		setup.mockGit.MockGetOriginUrl = ""

		_, err := setup.service.ResolveRunIDFromGitContext()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to determine the current branch and repository from git")
	})

	t.Run("returns error when no run found", func(t *testing.T) {
		setup := setupTest(t)
		setup.mockGit.MockGetBranch = "my-branch"
		setup.mockGit.MockGetOriginUrl = "git@github.com:rwx-cloud/rwx.git"
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{}, api.ErrNotFound
		}

		_, err := setup.service.ResolveRunIDFromGitContext()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no run found for rwx repository on branch my-branch")
	})

	t.Run("returns error when run ID is empty in response", func(t *testing.T) {
		setup := setupTest(t)
		setup.mockGit.MockGetBranch = "my-branch"
		setup.mockGit.MockGetOriginUrl = "git@github.com:rwx-cloud/rwx.git"
		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{RunID: ""}, nil
		}

		_, err := setup.service.ResolveRunIDFromGitContext()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no run found for rwx repository on branch my-branch")
	})
}
