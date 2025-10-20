package git

import (
	"os/exec"
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
