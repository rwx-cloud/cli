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

	base := filepath.Base(fixturePath)

	cmd := exec.Command("cp", fixturePath, tempDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to copy fixture %s to tempDir %s: %v", fixturePath, tempDir, err)
	}

	cmd = exec.Command("bash", base)
	cmd.Dir = tempDir

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to initialize fixture %s: %v", fixturePath, err)
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
