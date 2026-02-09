package git

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Client struct {
	Binary string
	Dir    string
	Stderr io.Writer
}

func (c *Client) logError(format string, args ...interface{}) {
	if c.Stderr != nil {
		fmt.Fprintf(c.Stderr, format+"\n", args...)
	}
}

func (c *Client) GetBranch() string {
	cmd := exec.Command(c.Binary, "branch", "--show-current")
	cmd.Dir = c.Dir

	out, err := cmd.Output()
	if err != nil {
		c.logError("Warning: Unable to determine git branch: %v", err)
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
			c.logError("Warning: Unable to determine git commit (rev-parse HEAD failed): %v", err)
			return ""
		}

		return strings.TrimSpace(string(out))
	}

	// Map known commits to their remote ref
	cmd := exec.Command(c.Binary, "for-each-ref", "--format=%(objectname) %(refname)", "refs/remotes/origin")
	cmd.Dir = c.Dir

	remoteRefs, err := cmd.Output()
	if err != nil {
		c.logError("Warning: Unable to determine git commit (failed to list remote refs): %v", err)
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

	if len(commitToRef) == 0 {
		c.logError("Warning: Unable to determine git commit (no remote refs found for origin)")
		return ""
	}

	// Walk our log until we find a commit that matches
	cmd = exec.Command(c.Binary, "rev-list", "HEAD")
	cmd.Dir = c.Dir
	commits, err := cmd.Output()

	if err != nil {
		c.logError("Warning: Unable to determine git commit (rev-list HEAD failed): %v", err)
		return ""
	}

	for _, commit := range strings.Split(strings.TrimSpace(string(commits)), "\n") {
		if _, ok := commitToRef[commit]; ok {
			return commit
		}
	}

	c.logError("Warning: Unable to determine git commit (no common ancestor found with origin)")
	return ""
}

func (c *Client) GetOriginUrl() string {
	cmd := exec.Command(c.Binary, "remote", "get-url", "origin")
	cmd.Dir = c.Dir

	url, err := cmd.Output()
	if err != nil {
		c.logError("Warning: Unable to determine git origin URL: %v", err)
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
