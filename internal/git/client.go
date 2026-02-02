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
	if c.GetBranch() == "" {
		cmd := exec.Command(c.Binary, "rev-parse", "HEAD")
		cmd.Dir = c.Dir

		out, err := cmd.Output()
		if err != nil {
			return ""
		}

		return strings.TrimSpace(string(out))
	}

	// Map known commits to their remote ref
	cmd := exec.Command(c.Binary, "for-each-ref", "--format=%(objectname) %(refname)", "refs/remotes/origin")
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

type UnstagedFilesMetadata struct {
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

func (c *Client) GeneratePatchFile(destDir string, pathspec []string) PatchFile {
	sha := c.GetCommit()
	if sha == "" {
		// We can't determine a patch
		return PatchFile{}
	}

	diffArgs := []string{"diff", sha, "--name-only"}
	if len(pathspec) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, pathspec...)
	}
	cmd := exec.Command(c.Binary, diffArgs...)
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

	lsFilesArgs := []string{"ls-files", "--others", "--exclude-standard"}
	if len(pathspec) > 0 {
		lsFilesArgs = append(lsFilesArgs, "--")
		lsFilesArgs = append(lsFilesArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, lsFilesArgs...)
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

	patchArgs := []string{"diff", sha, "-p", "--binary"}
	if len(pathspec) > 0 {
		patchArgs = append(patchArgs, "--")
		patchArgs = append(patchArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, patchArgs...)
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

// GeneratePatch returns patch bytes for staged changes relative to HEAD.
// This is used for sandbox sync where the remote already has the committed code.
// Returns (nil, nil, nil, nil, nil) if no changes or unable to generate patch.
func (c *Client) GeneratePatch(pathspec []string) ([]byte, *UntrackedFilesMetadata, *UnstagedFilesMetadata, *LFSChangedFilesMetadata, error) {
	diffArgs := []string{"diff", "--cached", "HEAD", "--name-only"}
	if len(pathspec) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, pathspec...)
	}
	cmd := exec.Command(c.Binary, diffArgs...)
	cmd.Dir = c.Dir

	files, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, nil, nil
	}

	lfsChangedFiles := []string{}

	for _, file := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		if file == "" {
			continue
		}
		cmd := exec.Command(c.Binary, "check-attr", "filter", "--", file)
		cmd.Dir = c.Dir

		attrs, err := cmd.CombinedOutput()
		if err != nil {
			return nil, nil, nil, nil, nil
		}

		if strings.Contains(string(attrs), "filter: lfs") {
			parts := strings.SplitN(string(attrs), ":", 2)
			lfsFile := strings.TrimSpace(parts[0])
			lfsChangedFiles = append(lfsChangedFiles, string(lfsFile))
		}
	}

	if len(lfsChangedFiles) > 0 {
		lfsMetadata := &LFSChangedFilesMetadata{
			Files: lfsChangedFiles,
			Count: len(lfsChangedFiles),
		}
		return nil, nil, nil, lfsMetadata, nil
	}

	// Check for untracked files
	lsFilesArgs := []string{"ls-files", "--others", "--exclude-standard"}
	if len(pathspec) > 0 {
		lsFilesArgs = append(lsFilesArgs, "--")
		lsFilesArgs = append(lsFilesArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, lsFilesArgs...)
	cmd.Dir = c.Dir

	untracked, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, nil, nil
	}

	untrackedFiles := strings.Fields(string(untracked))
	var untrackedMetadata *UntrackedFilesMetadata
	if len(untrackedFiles) > 0 {
		untrackedMetadata = &UntrackedFilesMetadata{
			Files: untrackedFiles,
			Count: len(untrackedFiles),
		}
	}

	// Check for unstaged changes (tracked files with modifications not staged)
	unstagedArgs := []string{"diff", "HEAD", "--name-only"}
	if len(pathspec) > 0 {
		unstagedArgs = append(unstagedArgs, "--")
		unstagedArgs = append(unstagedArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, unstagedArgs...)
	cmd.Dir = c.Dir

	unstaged, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, nil, nil
	}

	unstagedFiles := strings.Fields(string(unstaged))
	var unstagedMetadata *UnstagedFilesMetadata
	if len(unstagedFiles) > 0 {
		unstagedMetadata = &UnstagedFilesMetadata{
			Files: unstagedFiles,
			Count: len(unstagedFiles),
		}
	}

	patchArgs := []string{"diff", "--cached", "HEAD", "-p", "--binary"}
	if len(pathspec) > 0 {
		patchArgs = append(patchArgs, "--")
		patchArgs = append(patchArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, patchArgs...)
	cmd.Dir = c.Dir

	patch, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, nil, nil
	}

	if len(patch) == 0 {
		return nil, untrackedMetadata, unstagedMetadata, nil, nil
	}

	return patch, untrackedMetadata, unstagedMetadata, nil, nil
}
