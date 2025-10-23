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
		if err := os.MkdirAll(filepath.Dir(c.MockGeneratePatchFile.Path), 0755); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}

		if err := os.WriteFile(c.MockGeneratePatchFile.Path, []byte("patch"), 0644); err != nil {
			// We can't write a patch
			return git.PatchFile{}
		}
	}

	return c.MockGeneratePatchFile
}
