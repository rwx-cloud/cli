package cli_test

import (
	// "fmt"
	// "os"
	// "path/filepath"
	"testing"
	// "github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/mocks"
	// "github.com/stretchr/testify/require"
)

var _ cli.APIClient = (*mocks.API)(nil)

func TestService_InitiatingRunPatch(t *testing.T) {
	t.Run("when the run is not patchable", func(t *testing.T) {
		// it launches a run but does not patch
	})

	t.Run("when the run is patchable", func(t *testing.T) {
		t.Run("when env CI is set", func(t *testing.T) {
			// it launches a run but does not patch
		})

		t.Run("when env RWX_DISABLE_SYNC_LOCAL_CHANGES is set", func(t *testing.T) {
			// it launches a run but does not patch
		})

		t.Run("before opt-in configured", func(t *testing.T) {
			// it prompts
			// it saves an opt-in configuration file
			// it launches a run with a patch
		})

		t.Run("without opt-in configured", func(t *testing.T) {
			// it launches a run with a patch
		})
	})

	t.Run("when env CI is set", func(t *testing.T) {
		t.Run("when the run is patchable", func(t *testing.T) {
			// it launches a run but does not patch
		})

		t.Run("when the run is not patchable", func(t *testing.T) {
			// it launches a run but does not patch
		})
	})

	t.Run("when env RWX_DISABLE_SYNC_LOCAL_CHANGES is set", func(t *testing.T) {
		t.Run("when the run is patchable", func(t *testing.T) {
			// it launches a run but does not patch
		})

		t.Run("when the run is not patchable", func(t *testing.T) {
			// it launches a run but does not patch
		})
	})
}
