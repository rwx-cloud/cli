package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Client struct {
	Binary string
	Dir    string
}

func (c *Client) GetBranch() string {
	cmd := exec.Command(c.Binary, "branch", "--show-current")
	cmd.Dir = c.Dir

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(out))
	return branch
}

func (c *Client) GetCommit() string {
	cmd := exec.Command(c.Binary, "rev-list", "HEAD")
	cmd.Dir = c.Dir

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, commit := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if commit == "" {
			continue
		}

		cmd = exec.Command(c.Binary, "fetch", "origin", commit, "--depth=1")
		cmd.Dir = c.Dir

		if cmd.Run() == nil {
			return commit
		}
	}

	return ""
}

func (c *Client) GetOriginUrl() string {
	cmd := exec.Command(c.Binary, "remote", "get-url", "origin")
	cmd.Dir = c.Dir

	url, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(url))
}

type UntrackedFilesMetadata struct {
	Files []string
	Count int
}

type LFSChangedFilesMetadata struct {
	Files []string
	Count int
}

type PatchFile struct {
	Written         bool
	Path            string
	UntrackedFiles  UntrackedFilesMetadata
	LFSChangedFiles LFSChangedFilesMetadata
}

func (c *Client) GeneratePatchFile(destDir string) PatchFile {
	sha := c.GetCommit()
	if sha == "" {
		// We can't determine a patch
		return PatchFile{}
	}

	cmd := exec.Command(c.Binary, "diff", sha, "--name-only")
	cmd.Dir = c.Dir

	files, err := cmd.Output()
	if err != nil {
		// We can't determine a patch
		return PatchFile{}
	}

	lfsChangedFiles := []string{}

	for _, file := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		cmd := exec.Command(c.Binary, "check-attr", "filter", "--", file)
		cmd.Dir = c.Dir

		attrs, err := cmd.CombinedOutput()
		if err != nil {
			// We can't determine a patch
			return PatchFile{}
		}

		if strings.Contains(string(attrs), "filter: lfs") {
			parts := strings.SplitN(string(attrs), ":", 2)
			lfsFile := strings.TrimSpace(parts[0])
			lfsChangedFiles = append(lfsChangedFiles, string(lfsFile))
		}
	}

	if len(lfsChangedFiles) > 0 {
		// There are changes to LFS tracked files
		lfsMetadata := LFSChangedFilesMetadata{
			Files: lfsChangedFiles,
			Count: len(lfsChangedFiles),
		}

		return PatchFile{
			LFSChangedFiles: lfsMetadata,
		}
	}

	cmd = exec.Command(c.Binary, "ls-files", "--others", "--exclude-standard")
	cmd.Dir = c.Dir

	untracked, err := cmd.Output()
	if err != nil {
		// We can't determine untracked files
		return PatchFile{}
	}

	untrackedFiles := strings.Fields(string(untracked))
	untrackedMetadata := UntrackedFilesMetadata{
		Files: untrackedFiles,
		Count: len(untrackedFiles),
	}

	cmd = exec.Command(c.Binary, "diff", sha, "-p", "--binary")
	cmd.Dir = c.Dir

	patch, err := cmd.Output()
	if err != nil {
		// We can't determine a patch
		return PatchFile{}
	}

	if string(patch) == "" {
		// There is no patch
		return PatchFile{}
	}

	outputPath := filepath.Join(destDir, sha)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		// We can't write a patch
		return PatchFile{}
	}

	if err = os.WriteFile(outputPath, patch, 0644); err != nil {
		// We can't write a patch
		return PatchFile{}
	}

	return PatchFile{
		Written:        true,
		Path:           outputPath,
		UntrackedFiles: untrackedMetadata,
	}
}
