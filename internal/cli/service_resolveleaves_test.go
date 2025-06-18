package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_ResolvingLeaves(t *testing.T) {
	t.Run("when no files provided", func(t *testing.T) {
		t.Run("when no yaml files found in the default directory", func(t *testing.T) {
			// Setup
			s := setupTest(t)

			mintDir := s.tmp

			err := os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
			require.NoError(t, err)

			// returns an error
			_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
				RwxDirectory:        mintDir,
				LatestVersionPicker: cli.PickLatestMajorVersion,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), fmt.Sprintf("no files provided, and no yaml files found in directory %s", mintDir))
		})

		t.Run("when yaml files are found in the specified directory", func(t *testing.T) {
			// Setup
			s := setupTest(t)

			mintDir := s.tmp

			err := os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node
`), 0o644)
			require.NoError(t, err)

			nestedDir := filepath.Join(mintDir, "some", "nested", "dir")
			err = os.MkdirAll(nestedDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node
`), 0o644)
			require.NoError(t, err)

			s.mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
				return &api.LeafVersionsResult{
					LatestMajor: map[string]string{"mint/setup-node": "1.3.0"},
				}, nil
			}

			// uses the default directory
			_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
				RwxDirectory:        mintDir,
				LatestVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.2.3")

			contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.3.0")

			contents, err = os.ReadFile(filepath.Join(mintDir, "some", "nested", "dir", "tasks.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.3.0")
		})
	})

	t.Run("with files", func(t *testing.T) {
		t.Run("when the leaf versions cannot be retrieved", func(t *testing.T) {
			// Setup
			s := setupTest(t)

			s.mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
				return nil, errors.New("cannot get leaf versions")
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(""), 0o644)
			require.NoError(t, err)

			// returns an error
			_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
				RwxDirectory:        s.tmp,
				LatestVersionPicker: cli.PickLatestMajorVersion,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot get leaf versions")
		})

		t.Run("when all leaves have a version", func(t *testing.T) {
			// Setup
			s := setupTest(t)

			s.mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
				return &api.LeafVersionsResult{
					LatestMajor: map[string]string{"mint/setup-node": "1.3.0"},
				}, nil
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
			require.NoError(t, err)

			// does not change the file content
			_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
				RwxDirectory:        s.tmp,
				LatestVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
			require.NoError(t, err)
			require.Equal(t, `
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`, string(contents))

			// indicates no leaves were resolved
			require.Contains(t, s.mockStdout.String(), "No leaves to resolve.")
		})

		t.Run("when there are leaves to resolve across multiple files", func(t *testing.T) {
			// Setup
			s := setupTest(t)

			s.mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
				return &api.LeafVersionsResult{
					LatestMajor: map[string]string{
						"mint/setup-node": "1.2.3",
						"mint/setup-ruby": "1.0.1",
						"mint/setup-go":   "1.3.5",
					},
				}, nil
			}

			originalFooContents := `
tasks:
  - key: foo
    call: mint/setup-node
  - key: bar
    call: mint/setup-ruby 0.0.1
  - key: baz
    call: mint/setup-go
`
			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
			require.NoError(t, err)

			originalBarContents := `
tasks:
  - key: foo
    call: mint/setup-ruby
`
			err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
			require.NoError(t, err)

			t.Run("updates all files", func(t *testing.T) {
				_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
					RwxDirectory:        s.tmp,
					LatestVersionPicker: cli.PickLatestMajorVersion,
				})
				require.NoError(t, err)

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 0.0.1
  - key: baz
    call: mint/setup-go 1.3.5
`, string(contents))

				contents, err = os.ReadFile(filepath.Join(s.tmp, "bar.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`, string(contents))
			})

			t.Run("indicates leaves were resolved", func(t *testing.T) {
				// Reset files
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
				require.NoError(t, err)

				_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
					RwxDirectory:        s.tmp,
					LatestVersionPicker: cli.PickLatestMajorVersion,
				})

				require.NoError(t, err)
				require.Contains(t, s.mockStdout.String(), "Resolved the following leaves:")
				require.Contains(t, s.mockStdout.String(), "mint/setup-go → 1.3.5")
				require.Contains(t, s.mockStdout.String(), "mint/setup-node → 1.2.3")
				require.Contains(t, s.mockStdout.String(), "mint/setup-ruby → 1.0.1")
			})

			t.Run("when a single file is targeted", func(t *testing.T) {
				// Reset files
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
				require.NoError(t, err)

				// resolves only the targeted file
				_, err = s.service.ResolveLeaves(cli.ResolveLeavesConfig{
					RwxDirectory:        s.tmp,
					Files:               []string{filepath.Join(s.tmp, "bar.yaml")},
					LatestVersionPicker: cli.PickLatestMajorVersion,
				})
				require.NoError(t, err)

				contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, originalFooContents, string(contents))

				contents, err = os.ReadFile(filepath.Join(s.tmp, "bar.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`, string(contents))
			})
		})
	})
}
