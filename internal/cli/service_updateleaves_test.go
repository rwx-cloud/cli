package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_UpdatingPackages(t *testing.T) {
	t.Run("when no files provided", func(t *testing.T) {
		t.Run("when no yaml files found in the default directory", func(t *testing.T) {
			s := setupTest(t)

			mintDir := s.tmp

			err := os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{},
				RwxDirectory:             mintDir,
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), fmt.Sprintf("no files provided, and no yaml files found in directory %s", mintDir))
		})

		t.Run("when yaml files are found in the specified directory", func(t *testing.T) {
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
    call: mint/setup-node 1.2.3
`), 0o644)
			require.NoError(t, err)

			nestedDir := filepath.Join(mintDir, "some", "nested", "dir")
			err = os.MkdirAll(nestedDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
			require.NoError(t, err)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: map[string]string{"mint/setup-node": "1.3.0"},
				}, nil
			}

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{},
				RwxDirectory:             mintDir,
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.3.0")

			contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.3.0")

			contents, err = os.ReadFile(filepath.Join(mintDir, "some", "nested", "dir", "tasks.yaml"))
			require.NoError(t, err)
			require.Contains(t, string(contents), "mint/setup-node 1.3.0")
		})
	})

	t.Run("with files", func(t *testing.T) {
		t.Run("when the package versions cannot be retrieved", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return nil, errors.New("cannot get package versions")
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(""), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot get package versions")
		})

		t.Run("when all packages are already up-to-date", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: map[string]string{"mint/setup-node": "1.2.3"},
				}, nil
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
			require.NoError(t, err)
			require.Equal(t, `
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`, string(contents))

			require.Contains(t, s.mockStdout.String(), "No packages to update.")
		})

		t.Run("when there are packages to update across multiple files", func(t *testing.T) {
			s := setupTest(t)

			majorPackageVersions := map[string]string{
				"mint/setup-node": "1.2.3",
				"mint/setup-ruby": "1.0.1",
				"mint/setup-go":   "1.3.5",
			}

			minorPackageVersions := map[string]map[string]string{
				"mint/setup-node": {"1": "1.2.3"},
				"mint/setup-ruby": {"0": "0.0.2", "1": "1.0.1"},
				"mint/setup-go":   {"1": "1.3.5"},
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: majorPackageVersions,
					LatestMinor: minorPackageVersions,
				}, nil
			}

			originalFooContents := `
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
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
    call: mint/setup-ruby 1.0.0
`
			err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
			require.NoError(t, err)

			t.Run("with major version updates", func(t *testing.T) {
				err := s.service.UpdatePackages(cli.UpdatePackagesConfig{
					Files:                    []string{filepath.Join(s.tmp, "foo.yaml"), filepath.Join(s.tmp, "bar.yaml")},
					ReplacementVersionPicker: cli.PickLatestMajorVersion,
				})
				require.NoError(t, err)

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 1.0.1
  - key: baz
    call: mint/setup-go 1.3.5
`, string(contents))

				contents, err = os.ReadFile(filepath.Join(s.tmp, "bar.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`, string(contents))

				require.Contains(t, s.mockStdout.String(), "Updated the following packages:")
				require.Contains(t, s.mockStdout.String(), "mint/setup-go → 1.3.5")
				require.Contains(t, s.mockStdout.String(), "mint/setup-node 1.0.1 → 1.2.3")
				require.Contains(t, s.mockStdout.String(), "mint/setup-ruby 0.0.1 → 1.0.1")
				require.Contains(t, s.mockStdout.String(), "mint/setup-ruby 1.0.0 → 1.0.1")
			})

			t.Run("with minor version updates only", func(t *testing.T) {
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
				require.NoError(t, err)

				err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
					Files:                    []string{filepath.Join(s.tmp, "foo.yaml"), filepath.Join(s.tmp, "bar.yaml")},
					ReplacementVersionPicker: cli.PickLatestMinorVersion,
				})
				require.NoError(t, err)

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 0.0.2
  - key: baz
    call: mint/setup-go 1.3.5
`, string(contents))

				contents, err = os.ReadFile(filepath.Join(s.tmp, "bar.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`, string(contents))

				require.Contains(t, s.mockStdout.String(), "Updated the following packages:")
				require.Contains(t, s.mockStdout.String(), "mint/setup-go → 1.3.5")
				require.Contains(t, s.mockStdout.String(), "mint/setup-node 1.0.1 → 1.2.3")
				require.Contains(t, s.mockStdout.String(), "mint/setup-ruby 0.0.1 → 0.0.2")
				require.Contains(t, s.mockStdout.String(), "mint/setup-ruby 1.0.0 → 1.0.1")
			})

			t.Run("when a single file is targeted", func(t *testing.T) {
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(s.tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
				require.NoError(t, err)

				err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
					Files:                    []string{filepath.Join(s.tmp, "bar.yaml")},
					ReplacementVersionPicker: cli.PickLatestMajorVersion,
				})
				require.NoError(t, err)

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
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

		t.Run("updates snippet files", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: map[string]string{
						"mint/setup-node": "1.2.3",
						"mint/setup-go":   "1.3.5",
					},
				}, nil
			}

			mintDir := filepath.Join(s.tmp, ".mint")
			err := os.MkdirAll(mintDir, 0o755)
			require.NoError(t, err)

			originalBazContents := `
# leading commment
- key: foo
  call: mint/setup-node 1.0.1
- key: bar
  call: mint/setup-go
`

			err = os.WriteFile(filepath.Join(mintDir, "_baz.yaml"), []byte(originalBazContents), 0o644)
			require.NoError(t, err)

			originalQuxContents := `
- not
- a
- list
- of
- tasks
`

			err = os.WriteFile(filepath.Join(mintDir, "_qux.yaml"), []byte(originalQuxContents), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(mintDir, "_baz.yaml"))
			require.NoError(t, err)
			require.Equal(t, `# leading commment
- key: foo
  call: mint/setup-node 1.2.3
- key: bar
  call: mint/setup-go 1.3.5
`, string(contents))

			contents, err = os.ReadFile(filepath.Join(mintDir, "_qux.yaml"))
			require.NoError(t, err)
			require.Equal(t, originalQuxContents, string(contents))
		})

		t.Run("when a package cannot be found", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: map[string]string{},
				}, nil
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
`), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
			require.NoError(t, err)
			require.Equal(t, `
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
`, string(contents))

			require.Contains(t, s.mockStderr.String(), `Unable to find the package "mint/setup-node"; skipping it.`)
		})

		t.Run("when a package reference is a later version than the latest major", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: map[string]string{"mint/setup-node": "1.0.3"},
				}, nil
			}

			err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`), 0o644)
			require.NoError(t, err)

			err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
				ReplacementVersionPicker: cli.PickLatestMajorVersion,
			})
			require.NoError(t, err)

			contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
			require.NoError(t, err)
			require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-node 1.0.3
`, string(contents))
		})

		t.Run("when a package reference is a major version behind the latest", func(t *testing.T) {
			s := setupTest(t)

			majorPackageVersions := map[string]string{"mint/setup-node": "2.0.3"}
			minorPackageVersions := map[string]map[string]string{
				"mint/setup-node": {
					"2": "2.0.3",
					"1": "1.1.1",
				},
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: majorPackageVersions,
					LatestMinor: minorPackageVersions,
				}, nil
			}

			t.Run("while referencing the latest minor version", func(t *testing.T) {
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`), 0o644)
				require.NoError(t, err)

				err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
					Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
					ReplacementVersionPicker: cli.PickLatestMinorVersion,
				})
				require.NoError(t, err)

				contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, `
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`, string(contents))

				require.Contains(t, s.mockStdout.String(), "No packages to update.")
			})

			t.Run("while not referencing the latest minor version", func(t *testing.T) {
				err := os.WriteFile(filepath.Join(s.tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.0.9
`), 0o644)
				require.NoError(t, err)

				err = s.service.UpdatePackages(cli.UpdatePackagesConfig{
					Files:                    []string{filepath.Join(s.tmp, "foo.yaml")},
					ReplacementVersionPicker: cli.PickLatestMinorVersion,
				})
				require.NoError(t, err)

				contents, err := os.ReadFile(filepath.Join(s.tmp, "foo.yaml"))
				require.NoError(t, err)
				require.Equal(t, `tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`, string(contents))

				require.Contains(t, s.mockStdout.String(), "Updated the following packages:")
				require.Contains(t, s.mockStdout.String(), "mint/setup-node 1.0.9 → 1.1.1")
			})
		})
	})
}
