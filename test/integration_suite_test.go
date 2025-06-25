package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type input struct {
	args []string
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

func mintCmd(t *testing.T, input input) *exec.Cmd {
	const rwxPath = "../rwx"
	_, err := os.Stat(rwxPath)
	require.NoError(t, err, "integration tests depend on a built rwx binary at %s", rwxPath)

	cmd := exec.Command(rwxPath, input.args...)

	t.Logf("Executing command: %s\n with env %s\n", cmd.String(), cmd.Env)

	return cmd
}

func runMint(t *testing.T, input input) result {
	cmd := mintCmd(t, input)
	var stdoutBuffer, stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()

	exitCode := 0

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		require.True(t, ok, "rwx exited with an error that wasn't an ExitError")
		exitCode = exitErr.ExitCode()
	}

	return result{
		stdout:   strings.TrimSuffix(stdoutBuffer.String(), "\n"),
		stderr:   strings.TrimSuffix(stderrBuffer.String(), "\n"),
		exitCode: exitCode,
	}
}

func TestMintRun(t *testing.T) {
	t.Run("errors if an init parameter is specified without a flag", func(t *testing.T) {
		input := input{
			args: []string{"run", "--access-token", "fake-for-test", "--file", "./hello-world.mint.yaml", "init=foo"},
		}

		result := runMint(t, input)

		require.Equal(t, 1, result.exitCode)
		require.Contains(t, result.stderr, "You have specified a task target with an equals sign: \"init=foo\".")
		require.Contains(t, result.stderr, "You may have meant to specify --init \"init=foo\".")
	})
}
