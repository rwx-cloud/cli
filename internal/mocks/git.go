package mocks

import (
	"github.com/rwx-cloud/cli/internal/git"
)

type Git struct {
	MockGetBranch         func(dir string) string
	MockGetCommit         func(dir string) string
	MockGeneratePatchFile func(sourceDir string, destDir string) git.PatchFile
}

func (c *Git) GetBranch(dir string, gitBinary ...string) string {
	if c.MockGetBranch != nil {
		return c.MockGetBranch(dir)
	}

	return ""
}

func (c *Git) GetCommit(dir string, gitBinary ...string) string {
	if c.MockGetCommit != nil {
		return c.MockGetCommit(dir)
	}

	return ""
}

func (c *Git) GeneratePatchFile(sourceDir string, destDir string, gitBinary ...string) git.PatchFile {
	if c.MockGeneratePatchFile != nil {
		return c.MockGeneratePatchFile(sourceDir, destDir)
	}

	return git.PatchFile{}
}
