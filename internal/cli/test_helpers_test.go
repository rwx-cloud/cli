package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/mocks"
	"github.com/stretchr/testify/require"
)

// testSetup contains common test setup data
type testSetup struct {
	config     cli.Config
	service    cli.Service
	mockAPI    *mocks.API
	mockSSH    *mocks.SSH
	mockStdout *strings.Builder
	mockStderr *strings.Builder
	tmp        string
	originalWd string
}

// setupTest creates a common test environment and returns cleanup function
func setupTest(t *testing.T) *testSetup {
	setup := &testSetup{}

	// Create temp directory
	var err error
	setup.tmp, err = os.MkdirTemp(os.TempDir(), "cli-service")
	require.NoError(t, err)

	setup.tmp, err = filepath.EvalSymlinks(setup.tmp)
	require.NoError(t, err)

	// Save and change working directory
	setup.originalWd, err = os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(setup.tmp)
	require.NoError(t, err)

	// Create mocks
	setup.mockAPI = new(mocks.API)
	setup.mockSSH = new(mocks.SSH)
	setup.mockStdout = &strings.Builder{}
	setup.mockStderr = &strings.Builder{}

	setup.config = cli.Config{
		APIClient: setup.mockAPI,
		SSHClient: setup.mockSSH,
		Stdout:    setup.mockStdout,
		Stderr:    setup.mockStderr,
	}

	// Create service
	setup.service, err = cli.NewService(setup.config)
	require.NoError(t, err)

	// Register cleanup
	t.Cleanup(func() {
		err := os.Chdir(setup.originalWd)
		require.NoError(t, err)
		err = os.RemoveAll(setup.tmp)
		require.NoError(t, err)
	})

	return setup
}
