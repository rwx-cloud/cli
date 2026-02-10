package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_ListPackages(t *testing.T) {
	t.Run("returns sorted packages with correct text output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"rwx/setup-ruby":  "2.1.0",
					"mint/setup-node": "1.3.0",
				},
				LatestMinor: map[string]map[string]string{
					"mint/setup-node": {"1": "1.3.0"},
					"rwx/setup-ruby":  {"1": "1.0.5", "2": "2.1.0"},
				},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Len(t, result.Packages, 2)
		require.Equal(t, "mint/setup-node", result.Packages[0].Name)
		require.Equal(t, "1.3.0", result.Packages[0].LatestVersion)
		require.Equal(t, "rwx/setup-ruby", result.Packages[1].Name)
		require.Equal(t, "2.1.0", result.Packages[1].LatestVersion)

		output := s.mockStdout.String()
		require.Contains(t, output, "PACKAGE")
		require.Contains(t, output, "LATEST VERSION")
		require.Contains(t, output, "mint/setup-node")
		require.Contains(t, output, "1.3.0")
		require.Contains(t, output, "rwx/setup-ruby")
		require.Contains(t, output, "2.1.0")
	})

	t.Run("excludes renamed packages from output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"mint/setup-node": "1.3.0",
					"mint/old-name":   "1.0.0",
					"mint/new-name":   "1.0.0",
				},
				Renames: map[string]string{
					"mint/old-name": "mint/new-name",
				},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Len(t, result.Packages, 2)
		require.Equal(t, "mint/new-name", result.Packages[0].Name)
		require.Equal(t, "mint/setup-node", result.Packages[1].Name)
	})

	t.Run("JSON mode outputs valid JSON", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"mint/setup-node": "1.3.0",
				},
				LatestMinor: map[string]map[string]string{
					"mint/setup-node": {"1": "1.3.0"},
				},
			}, nil
		}

		_, err := s.service.ListPackages(cli.ListPackagesConfig{Json: true})
		require.NoError(t, err)

		var output cli.ListPackagesResult
		err = json.Unmarshal([]byte(s.mockStdout.String()), &output)
		require.NoError(t, err)
		require.Len(t, output.Packages, 1)
		require.Equal(t, "mint/setup-node", output.Packages[0].Name)
		require.Equal(t, "1.3.0", output.Packages[0].LatestVersion)
		require.Equal(t, map[string]string{"1": "1.3.0"}, output.Packages[0].Versions)
	})

	t.Run("empty package list shows message", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Empty(t, result.Packages)
		require.Contains(t, s.mockStdout.String(), "No packages found.")
	})

	t.Run("API error is propagated", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return nil, errors.New("network error")
		}

		_, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.Error(t, err)
		require.Contains(t, err.Error(), "network error")
	})
}
