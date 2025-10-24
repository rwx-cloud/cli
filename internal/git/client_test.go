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
	t.Cleanup(func() {
		defer os.RemoveAll(tempDir)
	})

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
		client := &git.Client{Binary: "fake", Dir: ""}
		branch := client.GetBranch()
		require.Equal(t, "", branch)
	})

	t.Run("returns empty if we're not in a git repo", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gitrepo")
		defer os.RemoveAll(tempDir)

		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		client := &git.Client{Binary: "git", Dir: tempDir}
		branch := client.GetBranch()
		require.Equal(t, "", branch)
	})

	t.Run("returns empty if we're in detached HEAD state", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetBranch-detached-head")
		defer os.RemoveAll(repo)

		client := &git.Client{Binary: "git", Dir: repo}
		branch := client.GetBranch()
		require.Equal(t, expected, branch)
	})

	t.Run("returns a branch if we're on a branch", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetBranch-branch")
		defer os.RemoveAll(repo)

		client := &git.Client{Binary: "git", Dir: repo}
		branch := client.GetBranch()
		require.Equal(t, expected, branch)
	})
}

func TestGetCommit(t *testing.T) {
	t.Run("returns empty if git is not installed", func(t *testing.T) {
		client := &git.Client{Binary: "fake", Dir: ""}
		commit := client.GetCommit()
		require.Equal(t, "", commit)
	})

	t.Run("returns empty if we're not in a git repo", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gitrepo")
		defer os.RemoveAll(tempDir)

		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		client := &git.Client{Binary: "git", Dir: tempDir}
		commit := client.GetCommit()
		require.Equal(t, "", commit)
	})

	t.Run("returns empty if remote is not set", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-remote")
		defer os.RemoveAll(repo)

		client := &git.Client{Binary: "git", Dir: repo}
		commit := client.GetCommit()
		require.Equal(t, expected, commit)
	})

	t.Run("returns empty if remote origin is not set", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-remote-origin")
		defer os.RemoveAll(repo)

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		commit := client.GetCommit()
		require.Equal(t, expected, commit)
	})

	t.Run("returns empty if there is no common ancestor", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetCommit-no-common-ancestor")
		defer os.RemoveAll(repo)

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		commit := client.GetCommit()
		require.Equal(t, expected, commit)
	})

	t.Run("when we're in detatched HEAD state", func(t *testing.T) {
		t.Run("returns the common ancestor if we have the same HEAD", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-detached-head")
			defer os.RemoveAll(repo)

			client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
			commit := client.GetCommit()
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we've diverged", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-detached-head-diverged")
			defer os.RemoveAll(repo)

			client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
			commit := client.GetCommit()
			require.Equal(t, expected, commit)
		})
	})

	t.Run("when we have a branch checked out", func(t *testing.T) {
		t.Run("returns the common ancestor if we have the same HEAD", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch")
			defer os.RemoveAll(repo)

			client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
			commit := client.GetCommit()
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we have diverged", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch-diverged")
			defer os.RemoveAll(repo)

			client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
			commit := client.GetCommit()
			require.Equal(t, expected, commit)
		})

		t.Run("returns the common ancestor if we have diverged a lot", func(t *testing.T) {
			repo, expected := repoFixture(t, "testdata/GetCommit-branch-diverged-a-lot")
			defer os.RemoveAll(repo)

			client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
			commit := client.GetCommit()
			require.Equal(t, expected, commit)
		})
	})
}

func TestGetOriginUrl(t *testing.T) {
	t.Run("returns empty if git is not installed", func(t *testing.T) {
		client := &git.Client{Binary: "fake", Dir: ""}
		url := client.GetOriginUrl()
		require.Equal(t, "", url)
	})

	t.Run("returns empty if we're not in a git repo", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gitrepo")
		defer os.RemoveAll(tempDir)

		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		client := &git.Client{Binary: "git", Dir: tempDir}
		url := client.GetOriginUrl()

		require.Equal(t, "", url)
	})

	t.Run("returns empty if there are no remotes", func(t *testing.T) {
		repo, _ := repoFixture(t, "testdata/GetOriginUrl-no-remote")

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		url := client.GetOriginUrl()

		require.Equal(t, "", url)
	})

	t.Run("returns empty if there is no remote origin", func(t *testing.T) {
		repo, _ := repoFixture(t, "testdata/GetOriginUrl-no-remote-origin")

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		url := client.GetOriginUrl()

		require.Equal(t, "", url)
	})

	t.Run("returns origin url even if there are many remotes", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetOriginUrl-many-remotes")

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		url := client.GetOriginUrl()

		require.NotEqual(t, "", expected)
		require.Equal(t, expected, url)
	})

	t.Run("returns origin url", func(t *testing.T) {
		repo, expected := repoFixture(t, "testdata/GetOriginUrl")

		client := &git.Client{Binary: "git", Dir: filepath.Join(repo, "repo")}
		url := client.GetOriginUrl()

		require.NotEqual(t, "", expected)
		require.Equal(t, expected, url)
	})
}

func TestGeneratePatchFile(t *testing.T) {
	t.Run("does not write a patch file", func(t *testing.T) {
		t.Run("when git is not installed", func(t *testing.T) {
			client := &git.Client{Binary: "fake", Dir: ""}
			patchFile := client.GeneratePatchFile("")

			require.Equal(t, false, patchFile.Written)
		})

		t.Run("when we can't determine a diff", func(t *testing.T) {
			client := &git.Client{Binary: "git", Dir: ""}
			patchFile := client.GeneratePatchFile("")

			require.Equal(t, false, patchFile.Written)
		})

		t.Run("when there is no diff", func(t *testing.T) {
			tempDir, _ := repoFixture(t, "testdata/GeneratePatchFile-no-diff")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, false, patchFile.Written)
		})

		t.Run("when there are uncommitted changes to LFS tracked files", func(t *testing.T) {
			tempDir, _ := repoFixture(t, "testdata/GeneratePatchFile-lfs")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, false, patchFile.Written)
			require.Equal(t, true, patchFile.LFSChanges)
		})
	})

	t.Run("writes a patch file", func(t *testing.T) {
		t.Run("when there's an uncommitted diff", func(t *testing.T) {
			tempDir, sha := repoFixture(t, "testdata/GeneratePatchFile-diff")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(tempDir, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")
		})

		t.Run("when there's an uncommitted diff", func(t *testing.T) {
			tempDir, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-committed")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(tempDir, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")

			require.Equal(t, []string{}, patchFile.UntrackedFiles)
		})

		t.Run("including changes to binary files", func(t *testing.T) {
			tempDir, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-binary")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(tempDir, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "GIT binary patch")

			require.Equal(t, []string{}, patchFile.UntrackedFiles)
		})

		t.Run("without changes to untracked files", func(t *testing.T) {
			tempDir, sha := repoFixture(t, "testdata/GeneratePatchFile-diff-untracked")

			client := &git.Client{Binary: "git", Dir: filepath.Join(tempDir, "repo")}
			patchFile := client.GeneratePatchFile(tempDir)

			require.Equal(t, true, patchFile.Written)
			require.Equal(t, filepath.Join(tempDir, ".patches", sha), patchFile.Path)

			patch, err := os.ReadFile(patchFile.Path)
			require.NoError(t, err)
			require.Contains(t, string(patch), "new file mode 100644")

			require.Equal(t, []string{"bar.txt"}, patchFile.UntrackedFiles)
		})
	})
}
