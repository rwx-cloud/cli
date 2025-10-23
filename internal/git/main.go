package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetBranch(dir string, gitBinary ...string) string {

	binary := "git"
	if len(gitBinary) > 0 && gitBinary[0] != "" {
		binary = gitBinary[0]
	}

	cmd := exec.Command(binary, "branch", "--show-current")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(out))
	return branch
}

func GetCommit(dir string, gitBinary ...string) string {
	binary := "git"
	if len(gitBinary) > 0 && gitBinary[0] != "" {
		binary = gitBinary[0]
	}

	// Map known commits to their remote ref
	cmd := exec.Command(binary, "for-each-ref", "--format=%(objectname) %(refname)", "refs/remotes")
	cmd.Dir = dir

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
	cmd = exec.Command(binary, "rev-list", "HEAD")
	cmd.Dir = dir
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

func GeneratePatchFile(sourceDir string, destDir string, gitBinary ...string) PatchFile {

	binary := "git"
	if len(gitBinary) > 0 && gitBinary[0] != "" {
		binary = gitBinary[0]
	}
	sha := GetCommit(sourceDir)
	if sha == "" {
		// We can't determine a patch
		return PatchFile{}
	}

	cmd := exec.Command(binary, "diff", sha, "--name-only")
	cmd.Dir = sourceDir

	files, err := cmd.Output()
	if err != nil {
		// We can't determine a patch
		return PatchFile{}
	}

	for _, file := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		cmd := exec.Command(binary, "check-attr", "filter", "--", file)
		cmd.Dir = sourceDir

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

	cmd = exec.Command(binary, "diff", sha, "-p", "--binary")
	cmd.Dir = sourceDir

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

	cmd = exec.Command(binary, "ls-files", "--others", "--exclude-standard")
	cmd.Dir = sourceDir

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
