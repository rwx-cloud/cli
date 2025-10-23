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
	// Map known commits to their remote ref
	cmd := exec.Command(c.Binary, "for-each-ref", "--format=%(objectname) %(refname)", "refs/remotes")
	cmd.Dir = c.Dir

	remoteRefs, err := cmd.Output()
	if err != nil {
		return ""
	}

	commitToRef := make(map[string][]string)
	for _, line := range strings.Split(string(remoteRefs), "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		commit := parts[0]
		ref := parts[1]

		commitToRef[commit] = append(commitToRef[commit], ref)
	}

	// Walk our log until we find a commit that matches
	cmd = exec.Command(c.Binary, "rev-list", "HEAD")
	cmd.Dir = c.Dir
	commits, err := cmd.Output()

	if err != nil {
		return ""
	}

	for _, commit := range strings.Split(strings.TrimSpace(string(commits)), "\n") {
		if _, ok := commitToRef[commit]; ok {
			return commit
		}
	}

	return ""
}

type PatchFile struct {
	Written        bool
	Path           string
	UntrackedFiles []string
	LFSChanges     bool
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

	for _, file := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		cmd := exec.Command(c.Binary, "check-attr", "filter", "--", file)
		cmd.Dir = c.Dir

		attrs, err := cmd.CombinedOutput()
		if err != nil {
			// We can't determine a patch
			return PatchFile{}
		}

		if strings.Contains(string(attrs), "filter: lfs") {
			// There are changes to LFS tracked files
			return PatchFile{
				LFSChanges: true,
			}
		}
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

	outputPath := filepath.Join(destDir, ".patches", sha)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		// We can't write a patch
		return PatchFile{}
	}

	if err = os.WriteFile(outputPath, patch, 0644); err != nil {
		// We can't write a patch
		return PatchFile{}
	}

	cmd = exec.Command(c.Binary, "ls-files", "--others", "--exclude-standard")
	cmd.Dir = c.Dir

	untracked, err := cmd.Output()
	if err != nil {
		// We can't determine untracked files
		return PatchFile{}
	}

	untrackedFiles := strings.Fields(string(untracked))

	return PatchFile{
		Written:        true,
		Path:           outputPath,
		UntrackedFiles: untrackedFiles,
	}
}
