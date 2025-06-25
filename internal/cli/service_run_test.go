package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_InitiatingRun(t *testing.T) {
	t.Run("with a specific mint file and no specific directory", func(t *testing.T) {
		t.Run("with a .mint directory", func(t *testing.T) {
			t.Run("when a directory with files is found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"
				resolveBaseLayerCalled := false

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					resolveBaseLayerCalled = true
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				getPackageVersionsCalled := false
				majorPackageVersions := make(map[string]string)
				minorPackageVersions := make(map[string]map[string]string)

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					getPackageVersionsCalled = true
					return &api.PackageVersionsResult{
						LatestMajor: majorPackageVersions,
						LatestMinor: minorPackageVersions,
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				originalRwxDirFileContent := "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
				var receivedSpecifiedFileContent string
				var receivedRwxDir []api.RwxDirectoryEntry

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				mintDir := filepath.Join(s.tmp, "some", "path", "to", ".mint")
				err = os.MkdirAll(mintDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
				require.NoError(t, err)

				nestedDir := filepath.Join(mintDir, "some", "nested", "path")
				err = os.MkdirAll(nestedDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 7)
					require.Equal(t, ".", cfg.RwxDirectory[0].Path)
					require.Equal(t, "mintdir-tasks.json", cfg.RwxDirectory[1].Path)
					require.Equal(t, "mintdir-tasks.yml", cfg.RwxDirectory[2].Path)
					require.Equal(t, "some", cfg.RwxDirectory[3].Path)
					require.Equal(t, "some/nested", cfg.RwxDirectory[4].Path)
					require.Equal(t, "some/nested/path", cfg.RwxDirectory[5].Path)
					require.Equal(t, "some/nested/path/tasks.yaml", cfg.RwxDirectory[6].Path)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					receivedRwxDir = cfg.RwxDirectory
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
				require.NotNil(t, receivedRwxDir)
				require.Equal(t, "", receivedRwxDir[0].FileContents)
				require.Equal(t, "some json", receivedRwxDir[1].FileContents)
				require.Equal(t, originalRwxDirFileContent, receivedRwxDir[2].FileContents)
				require.Equal(t, "", receivedRwxDir[3].FileContents)
				require.Equal(t, "", receivedRwxDir[4].FileContents)
				require.Equal(t, "", receivedRwxDir[5].FileContents)
				require.Equal(t, "some nested yaml", receivedRwxDir[6].FileContents)

				_ = resolveBaseLayerCalled
				_ = getPackageVersionsCalled
			})

			t.Run("when an empty directory is found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				var receivedSpecifiedFileContent string

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				mintDir := filepath.Join(s.tmp, "some", "path", "to", ".mint")
				err = os.MkdirAll(mintDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 1)
					require.Equal(t, ".", cfg.RwxDirectory[0].Path)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
			})

			t.Run("when a directory is not found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"
				resolveBaseLayerCalled := false

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					resolveBaseLayerCalled = true
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				var receivedSpecifiedFileContent string

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 0)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
				require.False(t, resolveBaseLayerCalled)
			})
		})

		t.Run("with a .rwx directory", func(t *testing.T) {
			t.Run("when a directory with files is found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				originalRwxDirFileContent := "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
				var receivedSpecifiedFileContent string
				var receivedRwxDir []api.RwxDirectoryEntry

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				rwxDir := filepath.Join(s.tmp, "some", "path", "to", ".rwx")
				err = os.MkdirAll(rwxDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
				require.NoError(t, err)

				nestedDir := filepath.Join(rwxDir, "some", "nested", "path")
				err = os.MkdirAll(nestedDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 7)
					require.Equal(t, ".", cfg.RwxDirectory[0].Path)
					require.Equal(t, "mintdir-tasks.json", cfg.RwxDirectory[1].Path)
					require.Equal(t, "mintdir-tasks.yml", cfg.RwxDirectory[2].Path)
					require.Equal(t, "some", cfg.RwxDirectory[3].Path)
					require.Equal(t, "some/nested", cfg.RwxDirectory[4].Path)
					require.Equal(t, "some/nested/path", cfg.RwxDirectory[5].Path)
					require.Equal(t, "some/nested/path/tasks.yaml", cfg.RwxDirectory[6].Path)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					receivedRwxDir = cfg.RwxDirectory
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
				require.NotNil(t, receivedRwxDir)
				require.Equal(t, "", receivedRwxDir[0].FileContents)
				require.Equal(t, "some json", receivedRwxDir[1].FileContents)
				require.Equal(t, originalRwxDirFileContent, receivedRwxDir[2].FileContents)
				require.Equal(t, "", receivedRwxDir[3].FileContents)
				require.Equal(t, "", receivedRwxDir[4].FileContents)
				require.Equal(t, "", receivedRwxDir[5].FileContents)
				require.Equal(t, "some nested yaml", receivedRwxDir[6].FileContents)
			})

			t.Run("when an empty directory is found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				var receivedSpecifiedFileContent string

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				rwxDir := filepath.Join(s.tmp, "some", "path", "to", ".rwx")
				err = os.MkdirAll(rwxDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 1)
					require.Equal(t, ".", cfg.RwxDirectory[0].Path)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
			})

			t.Run("when a directory is not found", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"
				resolveBaseLayerCalled := false

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					resolveBaseLayerCalled = true
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				var receivedSpecifiedFileContent string

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 0)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
				require.False(t, resolveBaseLayerCalled)
			})

			t.Run("when the directory includes a test-suites directory inside it", func(t *testing.T) {
				s := setupTest(t)

				runConfig := cli.InitiateRunConfig{}
				baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

				s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
					return api.ResolveBaseLayerResult{
						Os:   "ubuntu 24.04",
						Tag:  "1.0",
						Arch: "x86_64",
					}, nil
				}

				s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
					return &api.PackageVersionsResult{
						LatestMajor: make(map[string]string),
						LatestMinor: make(map[string]map[string]string),
					}, nil
				}

				originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
				originalRwxDirFileContent := "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
				var receivedSpecifiedFileContent string
				var receivedRwxDir []api.RwxDirectoryEntry

				workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
				err := os.MkdirAll(workingDir, 0o755)
				require.NoError(t, err)

				err = os.Chdir(workingDir)
				require.NoError(t, err)

				rwxDir := filepath.Join(s.tmp, "some", "path", "to", ".rwx")
				err = os.MkdirAll(rwxDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
				require.NoError(t, err)

				testSuitesDir := filepath.Join(rwxDir, "test-suites")
				err = os.MkdirAll(testSuitesDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(testSuitesDir, "config.yaml"), []byte("some yaml"), 0o644)
				require.NoError(t, err)

				nestedDir := filepath.Join(rwxDir, "some", "nested", "path")
				err = os.MkdirAll(nestedDir, 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
				require.NoError(t, err)

				runConfig.MintFilePath = "mint.yml"
				runConfig.RwxDirectory = ""

				s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
					require.Len(t, cfg.TaskDefinitions, 1)
					require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
					require.Len(t, cfg.RwxDirectory, 9)
					require.Equal(t, ".", cfg.RwxDirectory[0].Path)
					require.Equal(t, "mintdir-tasks.json", cfg.RwxDirectory[1].Path)
					require.Equal(t, "mintdir-tasks.yml", cfg.RwxDirectory[2].Path)
					require.Equal(t, "some", cfg.RwxDirectory[3].Path)
					require.Equal(t, "some/nested", cfg.RwxDirectory[4].Path)
					require.Equal(t, "some/nested/path", cfg.RwxDirectory[5].Path)
					require.Equal(t, "some/nested/path/tasks.yaml", cfg.RwxDirectory[6].Path)
					require.Equal(t, "test-suites", cfg.RwxDirectory[7].Path)
					require.Equal(t, "test-suites/config.yaml", cfg.RwxDirectory[8].Path)
					require.True(t, cfg.UseCache)
					receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
					receivedRwxDir = cfg.RwxDirectory
					return &api.InitiateRunResult{
						RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
						TargetedTaskKeys: []string{},
						DefinitionPath:   ".mint/mint.yml",
					}, nil
				}

				_, err = s.service.InitiateRun(runConfig)
				require.NoError(t, err)

				require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
				require.NotNil(t, receivedRwxDir)
				require.Equal(t, 9, len(receivedRwxDir))
				require.Equal(t, ".", receivedRwxDir[0].Path)
				require.Equal(t, "mintdir-tasks.json", receivedRwxDir[1].Path)
				require.Equal(t, "mintdir-tasks.yml", receivedRwxDir[2].Path)
				require.Equal(t, "some", receivedRwxDir[3].Path)
				require.Equal(t, "some/nested", receivedRwxDir[4].Path)
				require.Equal(t, "some/nested/path", receivedRwxDir[5].Path)
				require.Equal(t, "some/nested/path/tasks.yaml", receivedRwxDir[6].Path)
				require.Equal(t, "test-suites", receivedRwxDir[7].Path)
				require.Equal(t, "test-suites/config.yaml", receivedRwxDir[8].Path)
			})
		})

		t.Run("when base is missing", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}
			baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"
			resolveBaseLayerCalled := false

			s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				resolveBaseLayerCalled = true
				return api.ResolveBaseLayerResult{
					Os:   "ubuntu 24.04",
					Tag:  "1.0",
					Arch: "x86_64",
				}, nil
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: make(map[string]string),
					LatestMinor: make(map[string]map[string]string),
				}, nil
			}

			originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n"
			var receivedSpecifiedFileContent string
			var receivedRwxDirectoryFileContent string

			mintDir := filepath.Join(s.tmp, ".mint")
			err := os.MkdirAll(mintDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "foo.yml"), []byte(originalSpecifiedFileContent), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = ".mint/foo.yml"
			runConfig.RwxDirectory = ".mint"

			s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
				require.Len(t, cfg.TaskDefinitions, 1)
				require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
				require.Len(t, cfg.RwxDirectory, 2)
				require.True(t, cfg.UseCache)
				receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
				receivedRwxDirectoryFileContent = cfg.RwxDirectory[1].FileContents

				return &api.InitiateRunResult{
					RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					TargetedTaskKeys: []string{},
					DefinitionPath:   ".mint/foo.yml",
				}, nil
			}

			_, err = s.service.InitiateRun(runConfig)
			require.NoError(t, err)

			require.True(t, resolveBaseLayerCalled)
			require.Equal(t, fmt.Sprintf("%s\n%s", baseSpec, originalSpecifiedFileContent), receivedSpecifiedFileContent)
			require.Equal(t, fmt.Sprintf("%s\n%s", baseSpec, originalSpecifiedFileContent), receivedRwxDirectoryFileContent)
			require.Contains(t, s.mockStderr.String(), "Configured \".mint/foo.yml\" to run on ubuntu 24.04\n")
		})

		t.Run("when package is missing version", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}
			baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

			s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				return api.ResolveBaseLayerResult{
					Os:   "ubuntu 24.04",
					Tag:  "1.0",
					Arch: "x86_64",
				}, nil
			}

			getPackageVersionsCalled := false
			majorPackageVersions := make(map[string]string)
			majorPackageVersions["mint/setup-node"] = "1.2.3"

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				getPackageVersionsCalled = true
				return &api.PackageVersionsResult{
					LatestMajor: majorPackageVersions,
					LatestMinor: make(map[string]map[string]string),
				}, nil
			}

			originalSpecifiedFileContent := baseSpec + "tasks:\n  - key: foo\n    call: mint/setup-node\n"
			var receivedSpecifiedFileContent string
			var receivedRwxDirectoryFileContent string

			mintDir := filepath.Join(s.tmp, ".mint")
			err := os.MkdirAll(mintDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "foo.yml"), []byte(originalSpecifiedFileContent), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = ".mint/foo.yml"
			runConfig.RwxDirectory = ".mint"

			s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
				require.Len(t, cfg.TaskDefinitions, 1)
				require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
				require.Len(t, cfg.RwxDirectory, 2)
				require.True(t, cfg.UseCache)
				receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
				receivedRwxDirectoryFileContent = cfg.RwxDirectory[1].FileContents

				return &api.InitiateRunResult{
					RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					TargetedTaskKeys: []string{},
					DefinitionPath:   ".mint/foo.yml",
				}, nil
			}

			_, err = s.service.InitiateRun(runConfig)
			require.NoError(t, err)

			require.True(t, getPackageVersionsCalled)
			require.Equal(t, baseSpec+"tasks:\n  - key: foo\n    call: mint/setup-node 1.2.3\n", receivedSpecifiedFileContent)
			require.Equal(t, baseSpec+"tasks:\n  - key: foo\n    call: mint/setup-node 1.2.3\n", receivedRwxDirectoryFileContent)
			require.Contains(t, s.mockStderr.String(), "Configured package mint/setup-node to use version 1.2.3\n")
		})
	})

	t.Run("with no specific mint file and no specific directory", func(t *testing.T) {
		s := setupTest(t)

		runConfig := cli.InitiateRunConfig{
			MintFilePath: "",
			RwxDirectory: "",
		}

		_, err := s.service.InitiateRun(runConfig)

		require.Error(t, err)
		require.Contains(t, err.Error(), "the path to a run definition must be provided")
	})

	t.Run("with a specific mint file and a specific directory", func(t *testing.T) {
		t.Run("when a directory with files is found", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}
			baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

			s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				return api.ResolveBaseLayerResult{
					Os:   "ubuntu 24.04",
					Tag:  "1.0",
					Arch: "x86_64",
				}, nil
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: make(map[string]string),
					LatestMinor: make(map[string]map[string]string),
				}, nil
			}

			originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
			originalRwxDirFileContent := "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
			var receivedSpecifiedFileContent string
			var receivedRwxDir []api.RwxDirectoryEntry

			workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
			err := os.MkdirAll(workingDir, 0o755)
			require.NoError(t, err)

			err = os.Chdir(workingDir)
			require.NoError(t, err)

			mintDir := filepath.Join(s.tmp, "other", "path", "to", ".mint")
			err = os.MkdirAll(mintDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = "mint.yml"
			runConfig.RwxDirectory = mintDir

			s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
				require.Len(t, cfg.TaskDefinitions, 1)
				require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
				require.Len(t, cfg.RwxDirectory, 3)
				require.Equal(t, ".", cfg.RwxDirectory[0].Path)
				require.Equal(t, "mintdir-tasks.json", cfg.RwxDirectory[1].Path)
				require.Equal(t, "mintdir-tasks.yml", cfg.RwxDirectory[2].Path)
				require.True(t, cfg.UseCache)
				receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
				receivedRwxDir = cfg.RwxDirectory
				return &api.InitiateRunResult{
					RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					TargetedTaskKeys: []string{},
					DefinitionPath:   ".mint/mint.yml",
				}, nil
			}

			_, err = s.service.InitiateRun(runConfig)
			require.NoError(t, err)

			require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
			require.NotNil(t, receivedRwxDir)
			require.Equal(t, "", receivedRwxDir[0].FileContents)
			require.Equal(t, "some json", receivedRwxDir[1].FileContents)
			require.Equal(t, originalRwxDirFileContent, receivedRwxDir[2].FileContents)
		})

		t.Run("when an empty directory is found", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}
			baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

			s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				return api.ResolveBaseLayerResult{
					Os:   "ubuntu 24.04",
					Tag:  "1.0",
					Arch: "x86_64",
				}, nil
			}

			s.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
				return &api.PackageVersionsResult{
					LatestMajor: make(map[string]string),
					LatestMinor: make(map[string]map[string]string),
				}, nil
			}

			originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
			var receivedSpecifiedFileContent string

			workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
			err := os.MkdirAll(workingDir, 0o755)
			require.NoError(t, err)

			err = os.Chdir(workingDir)
			require.NoError(t, err)

			mintDir := filepath.Join(s.tmp, "other", "path", "to", ".mint")
			err = os.MkdirAll(mintDir, 0o755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = "mint.yml"
			runConfig.RwxDirectory = mintDir

			s.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
				require.Len(t, cfg.TaskDefinitions, 1)
				require.Equal(t, runConfig.MintFilePath, cfg.TaskDefinitions[0].Path)
				require.Len(t, cfg.RwxDirectory, 1)
				require.Equal(t, ".", cfg.RwxDirectory[0].Path)
				require.True(t, cfg.UseCache)
				receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
				return &api.InitiateRunResult{
					RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
					TargetedTaskKeys: []string{},
					DefinitionPath:   ".mint/mint.yml",
				}, nil
			}

			_, err = s.service.InitiateRun(runConfig)
			require.NoError(t, err)

			require.Equal(t, originalSpecifiedFileContent, receivedSpecifiedFileContent)
		})

		t.Run("when the 'directory' is actually a file", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}

			workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
			err := os.MkdirAll(workingDir, 0o755)
			require.NoError(t, err)

			err = os.Chdir(workingDir)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte("yaml contents"), 0o644)
			require.NoError(t, err)

			mintDir := filepath.Join(workingDir, ".mint")
			err = os.WriteFile(mintDir, []byte("actually a file"), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = "mint.yml"
			runConfig.RwxDirectory = mintDir

			_, err = s.service.InitiateRun(runConfig)

			require.Error(t, err)
			require.Contains(t, err.Error(), "is not a directory")
		})

		t.Run("when the directory is not found", func(t *testing.T) {
			s := setupTest(t)

			runConfig := cli.InitiateRunConfig{}
			baseSpec := "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"

			originalSpecifiedFileContent := "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec

			workingDir := filepath.Join(s.tmp, "some", "path", "to", "working", "directory")
			err := os.MkdirAll(workingDir, 0o755)
			require.NoError(t, err)

			err = os.Chdir(workingDir)
			require.NoError(t, err)

			mintDir := filepath.Join(s.tmp, "other", "path", "to", ".mint")

			err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
			require.NoError(t, err)

			runConfig.MintFilePath = "mint.yml"
			runConfig.RwxDirectory = mintDir

			_, err = s.service.InitiateRun(runConfig)

			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to find .rwx directory")
		})
	})

	t.Run("with no specific mint file and a specific directory", func(t *testing.T) {
		s := setupTest(t)

		runConfig := cli.InitiateRunConfig{
			MintFilePath: "",
			RwxDirectory: "some-dir",
		}

		_, err := s.service.InitiateRun(runConfig)

		require.Error(t, err)
		require.Contains(t, err.Error(), "the path to a run definition must be provided")
	})
}
