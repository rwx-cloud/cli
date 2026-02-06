package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/rwx-cloud/cli/internal/mocks"
	"github.com/stretchr/testify/require"
)

var _ cli.APIClient = (*mocks.API)(nil)

type initiateRunResult struct {
	rwxDir []api.RwxDirectoryEntry
	stderr string
}

func initiateRun(t *testing.T, patchFile git.PatchFile, expectedPatchMetadata api.PatchMetadata) initiateRunResult {
	s := setupTest(t)
	s.mockGit.MockGetCommit = "3e76c8295cd0ce4decbf7b56253c902ce296cb25"
	s.mockGit.MockGeneratePatchFile = patchFile

	var receivedRwxDir []api.RwxDirectoryEntry

	runConfig := cli.InitiateRunConfig{}

	rwxDir := filepath.Join(s.tmp, ".rwx")
	err := os.MkdirAll(rwxDir, 0o755)
	require.NoError(t, err)

	runConfig.RwxDirectory = rwxDir

	definitionsFile := filepath.Join(rwxDir, "rwx.yml")
	runConfig.MintFilePath = definitionsFile

	definition := "on:\n  cli:\n    init:\n      sha: ${{ event.git.sha }}\n\nbase:\n  os: ubuntu 24.04\n  tag: 1.0\n\ntasks:\n  - key: foo\n    run: echo 'bar'\n"

	err = os.WriteFile(definitionsFile, []byte(definition), 0o644)
	require.NoError(t, err)

	s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
		return &api.PackageVersionsResult{
			LatestMajor: make(map[string]string),
			LatestMinor: make(map[string]map[string]string),
		}, nil
	}
	s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
		require.Equal(t, expectedPatchMetadata.Sent, cfg.Patch.Sent)
		require.Equal(t, expectedPatchMetadata.UntrackedFiles, cfg.Patch.UntrackedFiles)
		require.Equal(t, expectedPatchMetadata.LFSFiles, cfg.Patch.LFSFiles)
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
	return initiateRunResult{rwxDir: receivedRwxDir, stderr: s.mockStderr.String()}
}

func TestService_InitiatingRunPatch(t *testing.T) {
	t.Run("when the run is not patchable", func(t *testing.T) {
		// it launches a run but does not patch
		result := initiateRun(t, git.PatchFile{}, api.PatchMetadata{})

		for _, entry := range result.rwxDir {
			require.False(t, strings.HasPrefix(entry.Path, ".patches/"))
		}
	})

	t.Run("patch logging", func(t *testing.T) {
		t.Run("when no patch is written", func(t *testing.T) {
			result := initiateRun(t, git.PatchFile{}, api.PatchMetadata{})
			require.NotContains(t, result.stderr, "Included a git patch")
		})

		t.Run("when a patch is written with no untracked files", func(t *testing.T) {
			patchFile := git.PatchFile{Written: true}
			expectedPatch := api.PatchMetadata{Sent: true}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "Included a git patch for uncommitted changes")
			require.NotContains(t, result.stderr, "untracked file")
		})

		t.Run("when a patch is written with 1 untracked file", func(t *testing.T) {
			patchFile := git.PatchFile{
				Written:        true,
				UntrackedFiles: git.UntrackedFilesMetadata{Files: []string{"foo.txt"}, Count: 1},
			}
			expectedPatch := api.PatchMetadata{Sent: true, UntrackedFiles: []string{"foo.txt"}, UntrackedCount: 1}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "The patch did not include the following untracked file. Add it with git add to use it in the run:")
			require.Contains(t, result.stderr, "  foo.txt")
		})

		t.Run("when a patch is written with 5 untracked files", func(t *testing.T) {
			files := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"}
			patchFile := git.PatchFile{
				Written:        true,
				UntrackedFiles: git.UntrackedFilesMetadata{Files: files, Count: 5},
			}
			expectedPatch := api.PatchMetadata{Sent: true, UntrackedFiles: files, UntrackedCount: 5}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "The patch did not include the following untracked files. Add them with git add to use them in the run:")
			for _, file := range files {
				require.Contains(t, result.stderr, "  "+file)
			}
			require.NotContains(t, result.stderr, "and ")
		})

		t.Run("when untracked files are all in the .rwx directory", func(t *testing.T) {
			patchFile := git.PatchFile{
				Written:        true,
				UntrackedFiles: git.UntrackedFilesMetadata{Files: []string{".rwx/sandbox.yml", ".rwx/config.yml"}, Count: 2},
			}
			expectedPatch := api.PatchMetadata{Sent: true}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "Included a git patch for uncommitted changes")
			require.NotContains(t, result.stderr, "untracked file")
		})

		t.Run("when untracked files include some in the .rwx directory", func(t *testing.T) {
			patchFile := git.PatchFile{
				Written:        true,
				UntrackedFiles: git.UntrackedFilesMetadata{Files: []string{".rwx/sandbox.yml", "foo.txt"}, Count: 2},
			}
			expectedPatch := api.PatchMetadata{Sent: true, UntrackedFiles: []string{"foo.txt"}, UntrackedCount: 1}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "The patch did not include the following untracked file. Add it with git add to use it in the run:")
			require.Contains(t, result.stderr, "  foo.txt")
			require.NotContains(t, result.stderr, ".rwx/sandbox.yml")
		})

		t.Run("when a patch is written with more than 5 untracked files", func(t *testing.T) {
			files := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt", "g.txt"}
			patchFile := git.PatchFile{
				Written:        true,
				UntrackedFiles: git.UntrackedFilesMetadata{Files: files, Count: 7},
			}
			expectedPatch := api.PatchMetadata{Sent: true, UntrackedFiles: files, UntrackedCount: 7}
			result := initiateRun(t, patchFile, expectedPatch)
			require.Contains(t, result.stderr, "The patch did not include the following untracked files. Add them with git add to use them in the run:")
			for _, file := range files[:5] {
				require.Contains(t, result.stderr, "  "+file)
			}
			require.NotContains(t, result.stderr, "  f.txt")
			require.NotContains(t, result.stderr, "  g.txt")
			require.Contains(t, result.stderr, "and 2 more")
		})
	})

	t.Run("when the run is patchable", func(t *testing.T) {
		untrackedFiles := git.UntrackedFilesMetadata{
			Files: []string{"foo.txt"},
			Count: 1,
		}
		lfsChangedFiles := git.LFSChangedFilesMetadata{
			Files: []string{"bar.txt"},
			Count: 1,
		}

		patchFile := git.PatchFile{
			Written:         true,
			UntrackedFiles:  untrackedFiles,
			LFSChangedFiles: lfsChangedFiles,
		}

		t.Run("when env RWX_DISABLE_GIT_PATCH is set", func(t *testing.T) {
			t.Setenv("RWX_DISABLE_GIT_PATCH", "1")

			// it launches a run but does not patch
			result := initiateRun(t, patchFile, api.PatchMetadata{})

			for _, entry := range result.rwxDir {
				require.False(t, strings.Contains(entry.Path, ".patches/"))
			}
		})

		t.Run("by default", func(t *testing.T) {
			expectedPatchMetadata := api.PatchMetadata{
				Sent:           true,
				UntrackedFiles: untrackedFiles.Files,
				UntrackedCount: untrackedFiles.Count,
				LFSFiles:       lfsChangedFiles.Files,
				LFSCount:       lfsChangedFiles.Count,
			}

			result := initiateRun(t, patchFile, expectedPatchMetadata)

			patched := false
			for _, entry := range result.rwxDir {
				if strings.Contains(entry.Path, ".patches/") {
					patched = true
				}
			}

			require.True(t, patched)
		})

		t.Run("when init params match git params", func(t *testing.T) {
			s := setupTest(t)
			s.mockGit.MockGetCommit = "3e76c8295cd0ce4decbf7b56253c902ce296cb25"
			s.mockGit.MockGeneratePatchFile = patchFile

			rwxDir := filepath.Join(s.tmp, ".rwx")
			err := os.MkdirAll(rwxDir, 0o755)
			require.NoError(t, err)

			definitionsFile := filepath.Join(rwxDir, "rwx.yml")

			definition := "on:\n  github:\n    push:\n      init:\n        sha: ${{ event.git.sha }}\n\nbase:\n  os: ubuntu 24.04\n  tag: 1.0\n\ntasks:\n  - key: foo\n    run: echo 'bar'\n"

			err = os.WriteFile(definitionsFile, []byte(definition), 0o644)
			require.NoError(t, err)

			runConfig := cli.InitiateRunConfig{
				RwxDirectory: rwxDir,
				MintFilePath: definitionsFile,
				InitParameters: map[string]string{
					"sha": "3e76c8295cd0ce4decbf7b56253c902ce296cb25", // a git param is passed by --init
				},
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: make(map[string]string),
					LatestMinor: make(map[string]map[string]string),
				}, nil
			}
			s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
				require.False(t, cfg.Patch.Sent) // so we skip the patch
				return &api.InitiateRunResult{
					RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					TargetedTaskKeys: []string{},
					DefinitionPath:   ".mint/mint.yml",
				}, nil
			}

			_, err = s.service.InitiateRun(runConfig)
			require.NoError(t, err)
		})
	})
}
