package mocks

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rwx-cloud/cli/internal/git"
)

type Git struct {
	MockGetBranch         string
	MockGetCommit         string
	MockGetCommitError    error
	MockGetOriginUrl      string
	MockGeneratePatchFile git.PatchFile
	MockGeneratePatch     func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error)
}

func (c *Git) GetBranch() string {
	return c.MockGetBranch
}

func (c *Git) GetCommit() (string, error) {
	return c.MockGetCommit, c.MockGetCommitError
}

func (c *Git) GetOriginUrl() string {
	return c.MockGetOriginUrl
}

func (c *Git) GeneratePatchFile(destDir string, pathspec []string) git.PatchFile {
	if c.MockGeneratePatchFile.Written {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}

		sha, _ := c.GetCommit()
		path := filepath.Join(destDir, sha)
		if err := os.WriteFile(path, []byte("patch"), 0644); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}

		return git.PatchFile{
			Written:         c.MockGeneratePatchFile.Written,
			Path:            path,
			UntrackedFiles:  c.MockGeneratePatchFile.UntrackedFiles,
			LFSChangedFiles: c.MockGeneratePatchFile.LFSChangedFiles,
		}
	}

	return c.MockGeneratePatchFile
}

func (c *Git) GeneratePatch(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
	if c.MockGeneratePatch != nil {
		return c.MockGeneratePatch(pathspec)
	}
	return nil, nil, nil
}

func (c *Git) ApplyPatch(patch []byte) *exec.Cmd {
	// Return a no-op command for testing
	return exec.Command("true")
}
