package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/mocks"
	"github.com/stretchr/testify/require"
)

type testSetup struct {
	config     cli.Config
	service    cli.Service
	mockAPI    *mocks.API
	mockSSH    *mocks.SSH
	mockGit    *mocks.Git
	mockStdout *strings.Builder
	mockStderr *strings.Builder
	tmp        string
	originalWd string
}

func setupTest(t *testing.T) *testSetup {
	setup := &testSetup{}

	var err error
	setup.tmp, err = os.MkdirTemp(os.TempDir(), "cli-service")
	require.NoError(t, err)

	setup.tmp, err = filepath.EvalSymlinks(setup.tmp)
	require.NoError(t, err)
	setup.originalWd, err = os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(setup.tmp)
	require.NoError(t, err)
	setup.mockAPI = new(mocks.API)
	setup.mockSSH = new(mocks.SSH)
	setup.mockGit = new(mocks.Git)
	setup.mockStdout = &strings.Builder{}
	setup.mockStderr = &strings.Builder{}

	setup.config = cli.Config{
		APIClient:   setup.mockAPI,
		SSHClient:   setup.mockSSH,
		GitClient:   setup.mockGit,
		Stdout:      setup.mockStdout,
		StdoutIsTTY: false,
		Stderr:      setup.mockStderr,
		StderrIsTTY: false,
	}
	setup.service, err = cli.NewService(setup.config)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir(setup.originalWd)
		require.NoError(t, err)
		err = os.RemoveAll(setup.tmp)
		require.NoError(t, err)
	})

	return setup
}
