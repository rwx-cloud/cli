package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/rwx-cloud/cli/internal/mocks"
	"github.com/stretchr/testify/require"
)

var _ cli.APIClient = (*mocks.API)(nil)

func initiateRun(t *testing.T, mockGit *mocks.Git) []api.RwxDirectoryEntry {
	s := setupTest(t)
	s.mockGit = mockGit

	var receivedRwxDir []api.RwxDirectoryEntry

	runConfig := cli.InitiateRunConfig{}
	runConfig.MintFilePath = "mint.yml"
	runConfig.RwxDirectory = ""

	definition := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n\ntasks:\n  - key: foo\n    run: echo 'bar'\n"

	err := os.WriteFile("mint.yml", []byte(definition), 0o644)
	require.NoError(t, err)

	s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
		return &api.PackageVersionsResult{
			LatestMajor: make(map[string]string),
			LatestMinor: make(map[string]map[string]string),
		}, nil
	}
	s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
		receivedRwxDir = cfg.RwxDirectory
		return &api.InitiateRunResult{
			RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
			RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
			TargetedTaskKeys: []string{},
			DefinitionPath:   ".mint/mint.yml",
		}, nil
	}
	_, err = s.service.InitiateRun(runConfig)
	require.NoError(t, err)
	return receivedRwxDir
}

func TestService_InitiatingRunPatch(t *testing.T) {
	t.Run("when the run is not patchable", func(t *testing.T) {
		// it launches a run but does not patch
		rwxDir := initiateRun(t, nil)

		for _, entry := range rwxDir {
			require.False(t, strings.HasPrefix(entry.Path, ".patches/"))
		}
	})

	t.Run("when the run is patchable", func(t *testing.T) {
		mockGit := new(mocks.Git)
		mockGit.MockGeneratePatchFile = git.PatchFile{
			Written: true,
			Path:    ".patches/3e76c8295cd0ce4decbf7b56253c902ce296cb25",
		}

		t.Run("when env CI is set", func(t *testing.T) {
			t.Setenv("CI", "1")

			// it launches a run but does not patch
			rwxDir := initiateRun(t, mockGit)

			for _, entry := range rwxDir {
				require.False(t, strings.HasPrefix(entry.Path, mockGit.MockGeneratePatchFile.Path))
			}
		})

		t.Run("when env RWX_DISABLE_SYNC_LOCAL_CHANGES is set", func(t *testing.T) {
			t.Setenv("RWX_DISABLE_SYNC_LOCAL_CHANGES", "1")

			// it launches a run but does not patch
			rwxDir := initiateRun(t, mockGit)

			for _, entry := range rwxDir {
				require.False(t, strings.HasPrefix(entry.Path, mockGit.MockGeneratePatchFile.Path))
			}
		})

		t.Run("when opted in to run patching", func(t *testing.T) {
			// TODO: Before release, we're going to have an initial prompt that
			// points you toward the opt out env vars and otherwise configures
			// the CLI to support run patching. For now, just assume absence
			// of the env vars is opt-in.
			rwxDir := initiateRun(t, mockGit)

			patched := false
			for _, entry := range rwxDir {
				if strings.HasPrefix(entry.Path, mockGit.MockGeneratePatchFile.Path) {
					patched = true
				}
			}

			require.True(t, patched)
		})
	})
}
