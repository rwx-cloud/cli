package git

import (
	"bytes"
	"fmt"
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

func (c *Client) GetCommit() (string, error) {
	if c.GetBranch() == "" {
		cmd := exec.Command(c.Binary, "rev-parse", "HEAD")
		cmd.Dir = c.Dir

		out, err := cmd.Output()
		if err != nil {
			// Not a git repository - return empty, no error (silent no-op)
			return "", nil
		}

		return strings.TrimSpace(string(out)), nil
	}

	// Find commits on HEAD that aren't on any origin ref, with boundary markers
	cmd := exec.Command(c.Binary, "rev-list", "HEAD", "--not", "--remotes=origin", "--boundary")
	cmd.Dir = c.Dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no git remote named 'origin' is configured")
	}

	output := strings.TrimSpace(string(out))

	// Empty output means HEAD is on an origin ref (no divergence) - return HEAD
	if output == "" {
		cmd = exec.Command(c.Binary, "rev-parse", "HEAD")
		cmd.Dir = c.Dir

		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("could not resolve HEAD")
		}

		return strings.TrimSpace(string(out)), nil
	}

	// First line starting with "-" is the boundary (closest merge-base)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "-") {
			return line[1:], nil
		}
	}

	// Output but no boundary means no common ancestor
	return "", fmt.Errorf("current branch has no commits in common with the 'origin' remote")
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

// patchResult holds the result of generating patch data
type patchResult struct {
	patch     []byte
	sha       string
	untracked UntrackedFilesMetadata
	lfs       LFSChangedFilesMetadata
	ok        bool
}

// generatePatchData generates patch data for working tree changes relative to the base commit on origin.
func (c *Client) generatePatchData(pathspec []string) patchResult {
	sha, err := c.GetCommit()
	if sha == "" || err != nil {
		return patchResult{}
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
		return patchResult{}
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
			return patchResult{}
		}

		if strings.Contains(string(attrs), "filter: lfs") {
			parts := strings.SplitN(string(attrs), ":", 2)
			lfsFile := strings.TrimSpace(parts[0])
			lfsChangedFiles = append(lfsChangedFiles, string(lfsFile))
		}
	}

	if len(lfsChangedFiles) > 0 {
		return patchResult{
			sha: sha,
			lfs: LFSChangedFilesMetadata{
				Files: lfsChangedFiles,
				Count: len(lfsChangedFiles),
			},
			ok: true,
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
		return patchResult{}
	}

	untrackedFiles := strings.Fields(string(untracked))

	patchArgs := []string{"diff", sha, "-p", "--binary"}
	if len(pathspec) > 0 {
		patchArgs = append(patchArgs, "--")
		patchArgs = append(patchArgs, pathspec...)
	}
	cmd = exec.Command(c.Binary, patchArgs...)
	cmd.Dir = c.Dir

	patch, err := cmd.Output()
	if err != nil {
		return patchResult{}
	}

	return patchResult{
		patch: patch,
		sha:   sha,
		untracked: UntrackedFilesMetadata{
			Files: untrackedFiles,
			Count: len(untrackedFiles),
		},
		ok: true,
	}
}

func (c *Client) GeneratePatchFile(destDir string, pathspec []string) PatchFile {
	data := c.generatePatchData(pathspec)
	if !data.ok {
		return PatchFile{}
	}

	if data.lfs.Count > 0 {
		return PatchFile{LFSChangedFiles: data.lfs}
	}

	if len(data.patch) == 0 {
		return PatchFile{}
	}

	outputPath := filepath.Join(destDir, data.sha)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return PatchFile{}
	}

	if err := os.WriteFile(outputPath, data.patch, 0644); err != nil {
		return PatchFile{}
	}

	return PatchFile{
		Written:        true,
		Path:           outputPath,
		UntrackedFiles: data.untracked,
	}
}

// AddUntrackedFilesForPatch temporarily adds untracked files with intent-to-add
// so they appear in git diff. Returns a cleanup function to undo the add.
func (c *Client) AddUntrackedFilesForPatch() (cleanup func(), err error) {
	// Get untracked files
	cmd := exec.Command(c.Binary, "ls-files", "--others", "--exclude-standard")
	cmd.Dir = c.Dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Split on newlines (not Fields) to handle filenames with spaces
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	if len(files) == 0 {
		return func() {}, nil // No untracked files, no-op cleanup
	}

	// Add with intent-to-add
	args := append([]string{"add", "-N", "--"}, files...)
	cmd = exec.Command(c.Binary, args...)
	cmd.Dir = c.Dir
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Return cleanup function
	cleanup = func() {
		args := append([]string{"reset", "HEAD", "--"}, files...)
		cmd := exec.Command(c.Binary, args...)
		cmd.Dir = c.Dir
		_ = cmd.Run() // Best effort cleanup
	}

	return cleanup, nil
}

// GeneratePatch returns patch bytes for working tree changes relative to the base commit on origin.
// Returns (nil, nil, nil) if no changes or unable to generate patch.
func (c *Client) GeneratePatch(pathspec []string) ([]byte, *LFSChangedFilesMetadata, error) {
	// Add untracked files temporarily so they appear in the diff
	cleanup, err := c.AddUntrackedFilesForPatch()
	if err != nil {
		// Non-fatal: proceed without untracked files
		cleanup = func() {}
	}
	defer cleanup()

	data := c.generatePatchData(pathspec)
	if !data.ok {
		return nil, nil, nil
	}

	if data.lfs.Count > 0 {
		return nil, &data.lfs, nil
	}

	if len(data.patch) == 0 {
		return nil, nil, nil
	}

	return data.patch, nil, nil
}

// ApplyPatch returns an exec.Cmd that applies a patch to the working directory.
// The patch bytes should be provided to the command's stdin before running.
func (c *Client) ApplyPatch(patch []byte) *exec.Cmd {
	cmd := exec.Command(c.Binary, "apply", "--allow-empty", "-")
	cmd.Dir = c.Dir
	cmd.Stdin = bytes.NewReader(patch)
	return cmd
}
