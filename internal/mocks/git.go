package mocks

import (
	"github.com/rwx-cloud/cli/internal/git"
)

type Git struct {
	MockGetBranch         func() string
	MockGetCommit         func() string
	MockGeneratePatchFile func(destDir string) git.PatchFile
}

func (c *Git) GetBranch() string {
	if c.MockGetBranch != nil {
		return c.MockGetBranch()
	}

	return ""
}

func (c *Git) GetCommit() string {
	if c.MockGetCommit != nil {
		return c.MockGetCommit()
	}

	return ""
}

func (c *Git) GeneratePatchFile(destDir string) git.PatchFile {
	if c.MockGeneratePatchFile != nil {
		return c.MockGeneratePatchFile(destDir)
	}

	return git.PatchFile{}
}
