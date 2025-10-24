package mocks

import (
	"os"
	"path/filepath"

	"github.com/rwx-cloud/cli/internal/git"
)

type Git struct {
	MockGetBranch         string
	MockGetCommit         string
	MockGeneratePatchFile git.PatchFile
}

func (c *Git) GetBranch() string {
	return c.MockGetBranch
}

func (c *Git) GetCommit() string {
	return c.MockGetCommit
}

func (c *Git) GeneratePatchFile(destDir string) git.PatchFile {
	if c.MockGeneratePatchFile.Written {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}

		path := filepath.Join(destDir, c.GetCommit())
		if err := os.WriteFile(path, []byte("patch"), 0644); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}

		return git.PatchFile{
			Written: c.MockGeneratePatchFile.Written,
			Path:    path,
		}
	}

	return c.MockGeneratePatchFile
}
