package cli_test

import (
	"encoding/json"
	"strings"
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
					"ruby/install":   "1.1.0",
					"nodejs/install": "1.3.0",
				},
				LatestMinor: map[string]map[string]string{
					"nodejs/install": {"1": "1.3.0"},
					"ruby/install":   {"1": "1.1.0"},
				},
				Packages: map[string]api.ApiPackageInfo{
					"nodejs/install": {Description: "Install Node.js"},
					"ruby/install":   {Description: "Install Ruby"},
				},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Len(t, result.Packages, 2)
		require.Equal(t, "nodejs/install", result.Packages[0].Name)
		require.Equal(t, "1.3.0", result.Packages[0].LatestVersion)
		require.Equal(t, "Install Node.js", result.Packages[0].Description)
		require.Equal(t, "ruby/install", result.Packages[1].Name)
		require.Equal(t, "1.1.0", result.Packages[1].LatestVersion)
		require.Equal(t, "Install Ruby", result.Packages[1].Description)

		output := s.mockStdout.String()
		require.Contains(t, output, "PACKAGE")
		require.Contains(t, output, "LATEST")
		require.Contains(t, output, "DESCRIPTION")
		require.Contains(t, output, "nodejs/install")
		require.Contains(t, output, "1.3.0")
		require.Contains(t, output, "Install Node.js")
		require.Contains(t, output, "ruby/install")
		require.Contains(t, output, "1.1.0")
		require.Contains(t, output, "Install Ruby")
	})

	t.Run("excludes renamed packages from output", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"nodejs/install": "1.3.0",
					"mint/git-clone": "1.0.0",
					"git/clone":      "1.0.0",
				},
				Renames: map[string]string{
					"mint/git-clone": "git/clone",
				},
				Packages: map[string]api.ApiPackageInfo{
					"nodejs/install": {Description: "Install Node.js"},
					"git/clone":      {Description: "Clone a Git repository"},
				},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Len(t, result.Packages, 2)
		require.Equal(t, "git/clone", result.Packages[0].Name)
		require.Equal(t, "nodejs/install", result.Packages[1].Name)
	})

	t.Run("wraps long descriptions in text output", func(t *testing.T) {
		s := setupTest(t)

		longDesc := "This is a very long description that exceeds the column width limit for display"

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"nodejs/install": "1.3.0",
				},
				Packages: map[string]api.ApiPackageInfo{
					"nodejs/install": {Description: longDesc},
				},
			}, nil
		}

		result, err := s.service.ListPackages(cli.ListPackagesConfig{Json: false})
		require.NoError(t, err)
		require.Equal(t, longDesc, result.Packages[0].Description)

		output := s.mockStdout.String()
		// Full description appears across wrapped lines
		require.Contains(t, output, "This is a very long description that exceeds the column")
		require.Contains(t, output, "width limit for display")
		// No single line exceeds 80 chars
		for _, line := range strings.Split(output, "\n") {
			require.LessOrEqual(t, len(line), 80)
		}
	})

	t.Run("JSON mode outputs valid JSON", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{
				LatestMajor: map[string]string{
					"nodejs/install": "1.3.0",
				},
				LatestMinor: map[string]map[string]string{
					"nodejs/install": {"1": "1.3.0"},
				},
				Packages: map[string]api.ApiPackageInfo{
					"nodejs/install": {Description: "Install Node.js"},
				},
			}, nil
		}

		_, err := s.service.ListPackages(cli.ListPackagesConfig{Json: true})
		require.NoError(t, err)

		var output cli.ListPackagesResult
		err = json.Unmarshal([]byte(s.mockStdout.String()), &output)
		require.NoError(t, err)
		require.Len(t, output.Packages, 1)
		require.Equal(t, "nodejs/install", output.Packages[0].Name)
		require.Equal(t, "1.3.0", output.Packages[0].LatestVersion)
		require.Equal(t, map[string]string{"1": "1.3.0"}, output.Packages[0].Versions)
		require.Equal(t, "Install Node.js", output.Packages[0].Description)
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

func TestService_ShowPackage(t *testing.T) {
	t.Run("shows table view by default", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageDocumentation = func(name string) (*api.PackageDocumentationResult, error) {
			require.Equal(t, "git/clone", name)
			return &api.PackageDocumentationResult{
				Name:            "git/clone",
				Version:         "1.2.0",
				Description:     "Clone a Git repository",
				Readme:          "# git/clone\n\nClones a Git repository.\n",
				SourceCodeUrl:   "https://github.com/rwx-research/mint-leaves",
				IssueTrackerUrl: "https://github.com/rwx-research/mint-leaves/issues",
				Parameters: []api.PackageDocumentationParameter{
					{Name: "repository", Required: true, Description: "The repository to clone"},
				},
			}, nil
		}

		result, err := s.service.ShowPackage(cli.ShowPackageConfig{PackageName: "git/clone"})
		require.NoError(t, err)
		require.Equal(t, "git/clone", result.Name)
		require.Equal(t, "1.2.0", result.Version)
		require.Equal(t, "Clone a Git repository", result.Description)
		require.Len(t, result.Parameters, 1)

		output := s.mockStdout.String()
		require.Contains(t, output, "Name:         git/clone")
		require.Contains(t, output, "Version:      1.2.0")
		require.Contains(t, output, "Description:  Clone a Git repository")
		require.Contains(t, output, "Source Code:  https://github.com/rwx-research/mint-leaves")
		require.Contains(t, output, "Issues:       https://github.com/rwx-research/mint-leaves/issues")
		require.Contains(t, output, "PARAMETER")
		require.Contains(t, output, "repository")
		require.Contains(t, output, "true")
		require.NotContains(t, output, "# git/clone")
	})

	t.Run("wraps long parameter descriptions", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageDocumentation = func(name string) (*api.PackageDocumentationResult, error) {
			return &api.PackageDocumentationResult{
				Name:        "git/clone",
				Version:     "1.2.0",
				Description: "Clone a Git repository",
				Parameters: []api.PackageDocumentationParameter{
					{Name: "ref", Required: true, Description: "Whether or not to preserve the .git directory. Set to true if you want to perform git operations."},
				},
			}, nil
		}

		_, err := s.service.ShowPackage(cli.ShowPackageConfig{PackageName: "git/clone"})
		require.NoError(t, err)

		output := s.mockStdout.String()
		// The full description should appear across multiple lines
		require.Contains(t, output, "Whether or not to preserve the .git")
		require.Contains(t, output, "directory.")
		// But not as a single long line
		for _, line := range strings.Split(output, "\n") {
			require.LessOrEqual(t, len(line), 80)
		}
	})

	t.Run("--readme flag renders readme", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageDocumentation = func(name string) (*api.PackageDocumentationResult, error) {
			return &api.PackageDocumentationResult{
				Name:        "git/clone",
				Version:     "1.2.0",
				Description: "Clone a Git repository",
				Readme:      "# git/clone\n\nClones a Git repository.\n",
				Parameters: []api.PackageDocumentationParameter{
					{Name: "repository", Required: true, Description: "The repository to clone"},
				},
			}, nil
		}

		_, err := s.service.ShowPackage(cli.ShowPackageConfig{PackageName: "git/clone", Readme: true})
		require.NoError(t, err)

		output := s.mockStdout.String()
		require.Contains(t, output, "# git/clone")
		require.Contains(t, output, "Clones a Git repository.")
		require.NotContains(t, output, "PARAMETER")
	})

	t.Run("returns error for not found package", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageDocumentation = func(name string) (*api.PackageDocumentationResult, error) {
			return nil, api.ErrNotFound
		}

		_, err := s.service.ShowPackage(cli.ShowPackageConfig{PackageName: "nonexistent/pkg"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to fetch documentation for package")
	})

	t.Run("JSON mode outputs structured result", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetPackageDocumentation = func(name string) (*api.PackageDocumentationResult, error) {
			return &api.PackageDocumentationResult{
				Name:            "git/clone",
				Version:         "1.2.0",
				Description:     "Clone a Git repository",
				Readme:          "# git/clone\n\nClones a Git repository.\n",
				SourceCodeUrl:   "https://github.com/rwx-research/mint-leaves",
				IssueTrackerUrl: "https://github.com/rwx-research/mint-leaves/issues",
				Parameters: []api.PackageDocumentationParameter{
					{Name: "repository", Required: true, Description: "The repository to clone"},
				},
			}, nil
		}

		_, err := s.service.ShowPackage(cli.ShowPackageConfig{PackageName: "git/clone", Json: true})
		require.NoError(t, err)

		var output cli.ShowPackageResult
		err = json.Unmarshal([]byte(s.mockStdout.String()), &output)
		require.NoError(t, err)
		require.Equal(t, "git/clone", output.Name)
		require.Equal(t, "1.2.0", output.Version)
		require.Equal(t, "Clone a Git repository", output.Description)
		require.Equal(t, "https://github.com/rwx-research/mint-leaves", output.SourceCodeUrl)
		require.Len(t, output.Parameters, 1)
		require.NotContains(t, s.mockStdout.String(), "# git/clone")
	})
}
