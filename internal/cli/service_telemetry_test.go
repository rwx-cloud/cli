package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/stretchr/testify/require"
)

func TestTelemetry_SandboxStart(t *testing.T) {
	t.Run("records sandbox.start for new sandbox", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "sandbox.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{
				RunID:  "run-new-123",
				RunURL: "https://cloud.rwx.com/runs/run-new-123",
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "token-123"}, nil
		}

		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}

		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, "run-new-123", result.RunID)

		events := setup.drainEvents()
		startEvents := findEvents(events, "sandbox.start")
		require.Len(t, startEvents, 1)
		require.Equal(t, false, startEvents[0].Props["reuse"])
		require.Equal(t, false, startEvents[0].Props["config_hash_changed"])
	})

	t.Run("records sandbox.start with reuse=true for reattach", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-existing-456"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable: true,
				Polling:     api.PollingResult{Completed: false},
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "token-456"}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, runID, result.RunID)

		events := setup.drainEvents()
		startEvent := findEvent(events, "sandbox.start")
		require.NotNil(t, startEvent)
		require.Equal(t, true, startEvent.Props["reuse"])
	})
}

func TestTelemetry_SessionCreatedAt(t *testing.T) {
	t.Run("sets CreatedAt on new sandbox session", func(t *testing.T) {
		setup := setupTest(t)

		before := time.Now().UTC().Add(-1 * time.Second)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "sandbox.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{RunID: "run-new-ts", RunURL: "url"}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "t"}, nil
		}
		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		_, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})
		require.NoError(t, err)

		// GetCurrentGitBranch uses real git, so in a temp dir with no repo it returns "detached"
		branch := cli.GetCurrentGitBranch(setup.tmp)
		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		session, ok := storage.GetSession(setup.tmp, branch, ".rwx/sandbox.yml")
		require.True(t, ok)
		require.NotNil(t, session.CreatedAt)
		require.True(t, session.CreatedAt.After(before))
	})
}
