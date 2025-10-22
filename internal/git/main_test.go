package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwx-cloud/cli/internal/git"
	"github.com/stretchr/testify/require"
)

func repoFixture(t *testing.T, fixturePath string) (string, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "gitrepo")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	fixtureInfo, err := os.Stat(fixturePath)
	if err != nil {
		t.Fatalf("could not find fixture: %v", err)
	}
	// Cache clear if the fixture file changes
	_ = fixtureInfo.ModTime()

	base := filepath.Base(fixturePath)

	cmd := exec.Command("cp", fixturePath, tempDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to copy fixture %s to tempDir %s: %v", fixturePath, tempDir, err)
	}

	cmd = exec.Command("bash", base)
	cmd.Dir = tempDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to initialize fixture %s: %v\nOutput:%s", fixturePath, err, out)
	}

	return tempDir, strings.TrimSpace(string(out))
}

func TestGetBranch(t *testing.T) {
	t.Run("returns empty if git is not installed", func(t *testing.T) {
		branch := git.GetBranch("", "fake-git")
		require.Equal(t, "", branch)
	})

	t.Run("returns empty if we're not in a git repo", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gitrepo")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		branch := git.GetBranch(tempDir)
		require.Equal(t, "", branch)
	})

	t.Run("returns empty if we're in detached HEAD state", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetBranch-detached-head")
		defer os.RemoveAll(repo)

		branch := git.GetBranch(repo)
		require.Equal(t, expected, branch)
	})

	t.Run("returns a branch if we're on a branch", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetBranch-branch")
		defer os.RemoveAll(repo)

		branch := git.GetBranch(repo)
		require.Equal(t, expected, branch)
	})
}

func TestGetCommit(t *testing.T) {
	t.Run("returns empty if git is not installed", func(t *testing.T) {
		commit := git.GetCommit("", "fake-git")
		require.Equal(t, "", commit)
	})

	t.Run("returns empty if we're not in a git repo", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gitrepo")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		commit := git.GetCommit(tempDir)
		require.Equal(t, "", commit)
	})

	t.Run("returns empty if remote is not set", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-remote")
		defer os.RemoveAll(repo)

		commit := git.GetCommit(repo)
		require.Equal(t, expected, commit)
	})

	t.Run("returns empty if remote origin is not set", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-remote-origin")
		defer os.RemoveAll(repo)

		commit := git.GetCommit(filepath.Join(repo, "repo"))
		require.Equal(t, expected, commit)
	})

	t.Run("returns empty if there is no common ancestor", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-common-ancestor")
		defer os.RemoveAll(repo)

		commit := git.GetCommit(repo)
		require.Equal(t, expected, commit)
	})

	t.Run("when we're in detatched HEAD state", func(t *testing.T) {
		t.Run("returns the common ancestor if we have the same HEAD", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-detached-head")
			defer os.RemoveAll(repo)

			commit := git.GetCommit(filepath.Join(repo, "repo"))
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we've diverged", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-detached-head-diverged")
			defer os.RemoveAll(repo)

			commit := git.GetCommit(filepath.Join(repo, "repo"))
			require.Equal(t, expected, commit)
		})
	})

	t.Run("when we have a branch checked out", func(t *testing.T) {
		t.Run("returns the common ancestor if we have the same HEAD", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch")
			defer os.RemoveAll(repo)

			commit := git.GetCommit(filepath.Join(repo, "repo"))
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we have diverged", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch-diverged")
			defer os.RemoveAll(repo)

			commit := git.GetCommit(filepath.Join(repo, "repo"))
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we have diverged a lot", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch-diverged-a-lot")
			defer os.RemoveAll(repo)

			commit := git.GetCommit(filepath.Join(repo, "repo"))
			require.Equal(t, expected, commit)
		})
	})
}

func TestGeneratePatchFile(t *testing.T) {
	t.Run("does not write a patch file", func(t *testing.T) {
		t.Run("when git is not installed", func(t *testing.T) {
			source := ""
			destination := ""
			patchFile := git.GeneratePatchFile(source, destination, "fake-git")
			require.Equal(t, false, patchFile.Written)
		})

		t.Run("when we can't determine a diff", func(t *testing.T) {
			source := ""
			destination := ""
			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, false, patchFile.Written)
		})

		t.Run("when there is no diff", func(t *testing.T) {
			repo, _ := repoFixture(t, "testdata/GeneratePatchFile-no-diff")
			defer os.RemoveAll(repo)

			source := filepath.Join(repo, "repo")
			destination := repo

			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, false, patchFile.Written)
		})
	})

	t.Run("writes a patch file", func(t *testing.T) {
		t.Run("when there's an uncommitted diff", func(t *testing.T) {
			repo, sha := repoFixture(t, "testdata/GeneratePatchFile-diff")
			defer os.RemoveAll(repo)

			source := filepath.Join(repo, "repo")
			destination := repo

			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(repo, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")
		})

		t.Run("when there's an uncommitted diff", func(t *testing.T) {
			repo, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-committed")
			defer os.RemoveAll(repo)

			source := filepath.Join(repo, "repo")
			destination := repo

			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(repo, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")

			require.Equal(t, []string{}, patchFile.UntrackedFiles)
		})

		t.Run("including changes to binary files", func(t *testing.T) {
			repo, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-binary")
			defer os.RemoveAll(repo)

			source := filepath.Join(repo, "repo")
			destination := repo

			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(repo, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "GIT binary patch")

			require.Equal(t, []string{}, patchFile.UntrackedFiles)
		})

		t.Run("without changes to untracked files", func(t *testing.T) {
			repo, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-untracked")
			defer os.RemoveAll(repo)

			source := filepath.Join(repo, "repo")
			destination := repo

			patchFile := git.GeneratePatchFile(source, destination)
			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(repo, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")

			require.Equal(t, []string{"bar.txt"}, patchFile.UntrackedFiles)
		})
	})
}
