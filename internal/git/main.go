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
