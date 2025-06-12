package cli_test

import (
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"fmt"
	"strings"

	"github.com/rwx-research/mint-cli/internal/accesstoken"
	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/errors"
	"github.com/rwx-research/mint-cli/internal/messages"
	"github.com/rwx-research/mint-cli/internal/mocks"

	"golang.org/x/crypto/ssh"
)

var _ = Describe("CLI Service", func() {
	var (
		config     cli.Config
		service    cli.Service
		mockAPI    *mocks.API
		mockSSH    *mocks.SSH
		mockStdout *strings.Builder
		mockStderr *strings.Builder
		tmp        string
		originalWd string
	)

	BeforeEach(func() {
		var err error
		tmp, err = os.MkdirTemp(os.TempDir(), "cli-service")
		Expect(err).NotTo(HaveOccurred())

		tmp, err = filepath.EvalSymlinks(tmp)
		Expect(err).NotTo(HaveOccurred())

		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tmp)
		Expect(err).NotTo(HaveOccurred())

		mockAPI = new(mocks.API)
		mockSSH = new(mocks.SSH)

		mockStdout = &strings.Builder{}
		mockStderr = &strings.Builder{}

		config = cli.Config{
			APIClient: mockAPI,
			SSHClient: mockSSH,
			Stdout:    mockStdout,
			Stderr:    mockStderr,
		}
	})

	AfterEach(func() {
		var err error

		err = os.Chdir(originalWd)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(tmp)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		service, err = cli.NewService(config)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("initiating a run", func() {
		var runConfig cli.InitiateRunConfig
		var baseSpec string
		var resolveBaseLayerCalled bool
		var getLeafVersionsCalled bool
		var majorLeafVersions map[string]string
		var minorLeafVersions map[string]map[string]string
		var getLeafVersionsError error

		BeforeEach(func() {
			runConfig = cli.InitiateRunConfig{}
			baseSpec = "base:\n  os: ubuntu 24.04\n  tag: 1.0\n"
			resolveBaseLayerCalled = false

			mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				resolveBaseLayerCalled = true
				return api.ResolveBaseLayerResult{
					Os:   "ubuntu 24.04",
					Tag:  "1.0",
					Arch: "x86_64",
				}, nil
			}

			getLeafVersionsCalled = false
			majorLeafVersions = make(map[string]string)
			minorLeafVersions = make(map[string]map[string]string)
			getLeafVersionsError = nil

			mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
				getLeafVersionsCalled = true
				return &api.LeafVersionsResult{
					LatestMajor: majorLeafVersions,
					LatestMinor: minorLeafVersions,
				}, getLeafVersionsError
			}
		})

		Context("with a specific mint file and no specific directory", func() {
			Describe("with a .mint directory", func() {
				Context("when a directory with files is found", func() {
					var originalSpecifiedFileContent string
					var originalRwxDirFileContent string
					var receivedSpecifiedFileContent string
					var receivedRwxDir []api.RwxDirectoryEntry

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						originalRwxDirFileContent = "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						mintDir := filepath.Join(tmp, "some", "path", "to", ".mint")
						err = os.MkdirAll(mintDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						nestedDir := filepath.Join(mintDir, "some", "nested", "path")
						err = os.MkdirAll(nestedDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(7))
							Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
							Expect(cfg.RwxDirectory[1].Path).To(Equal(".mint/mintdir-tasks.json"))
							Expect(cfg.RwxDirectory[2].Path).To(Equal(".mint/mintdir-tasks.yml"))
							Expect(cfg.RwxDirectory[3].Path).To(Equal(".mint/some"))
							Expect(cfg.RwxDirectory[4].Path).To(Equal(".mint/some/nested"))
							Expect(cfg.RwxDirectory[5].Path).To(Equal(".mint/some/nested/path"))
							Expect(cfg.RwxDirectory[6].Path).To(Equal(".mint/some/nested/path/tasks.yaml"))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							receivedRwxDir = cfg.RwxDirectory
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
						Expect(receivedRwxDir).NotTo(BeNil())
						Expect(receivedRwxDir[0].FileContents).To(Equal(""))
						Expect(receivedRwxDir[1].FileContents).To(Equal("some json"))
						Expect(receivedRwxDir[2].FileContents).To(Equal(originalRwxDirFileContent))
						Expect(receivedRwxDir[3].FileContents).To(Equal(""))
						Expect(receivedRwxDir[4].FileContents).To(Equal(""))
						Expect(receivedRwxDir[5].FileContents).To(Equal(""))
						Expect(receivedRwxDir[6].FileContents).To(Equal("some nested yaml"))
					})
				})

				Context("when an empty directory is found", func() {
					var originalSpecifiedFileContent string
					var receivedSpecifiedFileContent string

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						mintDir := filepath.Join(tmp, "some", "path", "to", ".mint")
						err = os.MkdirAll(mintDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(1))
							Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
					})
				})

				Context("when a directory is not found", func() {
					var originalSpecifiedFileContent string
					var receivedSpecifiedFileContent string

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(0))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
					})

					It("doesn't call the API to resolve the current base layer", func() {
						Expect(resolveBaseLayerCalled).To(BeFalse())
					})
				})
			})

			Describe("with a .rwx directory", func() {
				Context("when a directory with files is found", func() {
					var originalSpecifiedFileContent string
					var originalRwxDirFileContent string
					var receivedSpecifiedFileContent string
					var receivedRwxDir []api.RwxDirectoryEntry

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						originalRwxDirFileContent = "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						rwxDir := filepath.Join(tmp, "some", "path", "to", ".rwx")
						err = os.MkdirAll(rwxDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						nestedDir := filepath.Join(rwxDir, "some", "nested", "path")
						err = os.MkdirAll(nestedDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(7))
							Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
							Expect(cfg.RwxDirectory[1].Path).To(Equal(".mint/mintdir-tasks.json"))
							Expect(cfg.RwxDirectory[2].Path).To(Equal(".mint/mintdir-tasks.yml"))
							Expect(cfg.RwxDirectory[3].Path).To(Equal(".mint/some"))
							Expect(cfg.RwxDirectory[4].Path).To(Equal(".mint/some/nested"))
							Expect(cfg.RwxDirectory[5].Path).To(Equal(".mint/some/nested/path"))
							Expect(cfg.RwxDirectory[6].Path).To(Equal(".mint/some/nested/path/tasks.yaml"))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							receivedRwxDir = cfg.RwxDirectory
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
						Expect(receivedRwxDir).NotTo(BeNil())
						Expect(receivedRwxDir[0].FileContents).To(Equal(""))
						Expect(receivedRwxDir[1].FileContents).To(Equal("some json"))
						Expect(receivedRwxDir[2].FileContents).To(Equal(originalRwxDirFileContent))
						Expect(receivedRwxDir[3].FileContents).To(Equal(""))
						Expect(receivedRwxDir[4].FileContents).To(Equal(""))
						Expect(receivedRwxDir[5].FileContents).To(Equal(""))
						Expect(receivedRwxDir[6].FileContents).To(Equal("some nested yaml"))
					})
				})

				Context("when an empty directory is found", func() {
					var originalSpecifiedFileContent string
					var receivedSpecifiedFileContent string

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						rwxDir := filepath.Join(tmp, "some", "path", "to", ".rwx")
						err = os.MkdirAll(rwxDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(1))
							Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
					})
				})

				Context("when a directory is not found", func() {
					var originalSpecifiedFileContent string
					var receivedSpecifiedFileContent string

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(0))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("sends the file contents to cloud", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
					})

					It("doesn't call the API to resolve the current base layer", func() {
						Expect(resolveBaseLayerCalled).To(BeFalse())
					})
				})

				Context("when the directory includes a test-suites directory inside it", func() {
					var originalSpecifiedFileContent string
					var originalRwxDirFileContent string
					var receivedSpecifiedFileContent string
					var receivedRwxDir []api.RwxDirectoryEntry

					BeforeEach(func() {
						originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
						originalRwxDirFileContent = "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
						receivedSpecifiedFileContent = ""

						var err error

						workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
						err = os.MkdirAll(workingDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.Chdir(workingDir)
						Expect(err).NotTo(HaveOccurred())

						rwxDir := filepath.Join(tmp, "some", "path", "to", ".rwx")
						err = os.MkdirAll(rwxDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(rwxDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						testSuitesDir := filepath.Join(rwxDir, "test-suites")
						err = os.MkdirAll(testSuitesDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(testSuitesDir, "config.yaml"), []byte("some yaml"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						nestedDir := filepath.Join(rwxDir, "some", "nested", "path")
						err = os.MkdirAll(nestedDir, 0o755)
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte("some nested yaml"), 0o644)
						Expect(err).NotTo(HaveOccurred())

						runConfig.MintFilePath = "mint.yml"
						runConfig.RwxDirectory = ""

						mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
							Expect(cfg.TaskDefinitions).To(HaveLen(1))
							Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
							Expect(cfg.RwxDirectory).To(HaveLen(9))
							Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
							Expect(cfg.RwxDirectory[1].Path).To(Equal(".mint/mintdir-tasks.json"))
							Expect(cfg.RwxDirectory[2].Path).To(Equal(".mint/mintdir-tasks.yml"))
							Expect(cfg.RwxDirectory[3].Path).To(Equal(".mint/some"))
							Expect(cfg.RwxDirectory[4].Path).To(Equal(".mint/some/nested"))
							Expect(cfg.RwxDirectory[5].Path).To(Equal(".mint/some/nested/path"))
							Expect(cfg.RwxDirectory[6].Path).To(Equal(".mint/some/nested/path/tasks.yaml"))
							Expect(cfg.RwxDirectory[7].Path).To(Equal(".mint/test-suites"))
							Expect(cfg.RwxDirectory[8].Path).To(Equal(".mint/test-suites/config.yaml"))
							Expect(cfg.UseCache).To(BeTrue())
							receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
							receivedRwxDir = cfg.RwxDirectory
							return &api.InitiateRunResult{
								RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
								TargetedTaskKeys: []string{},
								DefinitionPath:   ".mint/mint.yml",
							}, nil
						}
					})

					JustBeforeEach(func() {
						_, err := service.InitiateRun(runConfig)
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not include the test-suites directory or its children", func() {
						Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
						Expect(receivedRwxDir).NotTo(BeNil())
						Expect(len(receivedRwxDir)).To(Equal(9))
						Expect(receivedRwxDir[0].Path).To(Equal(".mint"))
						Expect(receivedRwxDir[1].Path).To(Equal(".mint/mintdir-tasks.json"))
						Expect(receivedRwxDir[2].Path).To(Equal(".mint/mintdir-tasks.yml"))
						Expect(receivedRwxDir[3].Path).To(Equal(".mint/some"))
						Expect(receivedRwxDir[4].Path).To(Equal(".mint/some/nested"))
						Expect(receivedRwxDir[5].Path).To(Equal(".mint/some/nested/path"))
						Expect(receivedRwxDir[6].Path).To(Equal(".mint/some/nested/path/tasks.yaml"))
						Expect(receivedRwxDir[7].Path).To(Equal(".mint/test-suites"))
						Expect(receivedRwxDir[8].Path).To(Equal(".mint/test-suites/config.yaml"))
					})
				})
			})

			Context("when base is missing", func() {
				var originalSpecifiedFileContent string
				var receivedSpecifiedFileContent string
				var receivedRwxDirectoryFileContent string

				BeforeEach(func() {
					originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n"

					var err error

					mintDir := filepath.Join(tmp, ".mint")
					err = os.MkdirAll(mintDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "foo.yml"), []byte(originalSpecifiedFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = ".mint/foo.yml"
					runConfig.RwxDirectory = ".mint"

					mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
						Expect(cfg.TaskDefinitions).To(HaveLen(1))
						Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
						Expect(cfg.RwxDirectory).To(HaveLen(2))
						Expect(cfg.UseCache).To(BeTrue())
						receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
						receivedRwxDirectoryFileContent = cfg.RwxDirectory[1].FileContents

						return &api.InitiateRunResult{
							RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							TargetedTaskKeys: []string{},
							DefinitionPath:   ".mint/foo.yml",
						}, nil
					}
				})

				JustBeforeEach(func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("calls the API to resolve the current base layer", func() {
					Expect(resolveBaseLayerCalled).To(BeTrue())
				})

				It("passes the updated file content to initiate run", func() {
					Expect(receivedSpecifiedFileContent).To(Equal(fmt.Sprintf("%s\n%s", baseSpec, originalSpecifiedFileContent)))
				})

				It("passes the updated file content in the mint directory artifact", func() {
					Expect(receivedRwxDirectoryFileContent).To(Equal(fmt.Sprintf("%s\n%s", baseSpec, originalSpecifiedFileContent)))
				})

				It("prints a warning", func() {
					Expect(mockStderr.String()).To(ContainSubstring("Configured \".mint/foo.yml\" to run on ubuntu 24.04\n"))
				})
			})

			Context("when leaf is missing version", func() {
				var originalSpecifiedFileContent string
				var receivedSpecifiedFileContent string
				var receivedRwxDirectoryFileContent string

				BeforeEach(func() {
					originalSpecifiedFileContent = baseSpec + "tasks:\n  - key: foo\n    call: mint/setup-node\n"

					var err error

					mintDir := filepath.Join(tmp, ".mint")
					err = os.MkdirAll(mintDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "foo.yml"), []byte(originalSpecifiedFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = ".mint/foo.yml"
					runConfig.RwxDirectory = ".mint"

					majorLeafVersions["mint/setup-node"] = "1.2.3"

					mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
						Expect(cfg.TaskDefinitions).To(HaveLen(1))
						Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
						Expect(cfg.RwxDirectory).To(HaveLen(2))
						Expect(cfg.UseCache).To(BeTrue())
						receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
						receivedRwxDirectoryFileContent = cfg.RwxDirectory[1].FileContents

						return &api.InitiateRunResult{
							RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							TargetedTaskKeys: []string{},
							DefinitionPath:   ".mint/foo.yml",
						}, nil
					}
				})

				JustBeforeEach(func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("calls the API to resolve the current leaf version", func() {
					Expect(getLeafVersionsCalled).To(BeTrue())
				})

				It("passes the updated file content to initiate run", func() {
					Expect(receivedSpecifiedFileContent).To(Equal(baseSpec + "tasks:\n  - key: foo\n    call: mint/setup-node 1.2.3\n"))
				})

				It("passes the updated file content in the mint directory artifact", func() {
					Expect(receivedRwxDirectoryFileContent).To(Equal(baseSpec + "tasks:\n  - key: foo\n    call: mint/setup-node 1.2.3\n"))
				})

				It("prints a warning", func() {
					Expect(mockStderr.String()).To(ContainSubstring("Configured leaf mint/setup-node to use version 1.2.3\n"))
				})
			})
		})

		Context("with no specific mint file and no specific directory", func() {
			BeforeEach(func() {
				runConfig.MintFilePath = ""
				runConfig.RwxDirectory = ""
			})

			It("returns an error", func() {
				_, err := service.InitiateRun(runConfig)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("the path to a mint-file must be provided"))
			})
		})

		Context("with a specific mint file and a specific directory", func() {
			Context("when a directory with files is found", func() {
				var originalSpecifiedFileContent string
				var originalRwxDirFileContent string
				var receivedSpecifiedFileContent string
				var receivedRwxDir []api.RwxDirectoryEntry

				BeforeEach(func() {
					originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
					originalRwxDirFileContent = "tasks:\n  - key: mintdir\n    run: echo 'mintdir'\n" + baseSpec
					receivedSpecifiedFileContent = ""

					var err error

					workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
					err = os.MkdirAll(workingDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.Chdir(workingDir)
					Expect(err).NotTo(HaveOccurred())

					// note this is not in the heirarchy of the mint file or the working directory
					mintDir := filepath.Join(tmp, "other", "path", "to", ".mint")
					err = os.MkdirAll(mintDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.yml"), []byte(originalRwxDirFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "mintdir-tasks.json"), []byte("some json"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = "mint.yml"
					runConfig.RwxDirectory = mintDir

					mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
						Expect(cfg.TaskDefinitions).To(HaveLen(1))
						Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
						Expect(cfg.RwxDirectory).To(HaveLen(3))
						Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
						Expect(cfg.RwxDirectory[1].Path).To(Equal(".mint/mintdir-tasks.json"))
						Expect(cfg.RwxDirectory[2].Path).To(Equal(".mint/mintdir-tasks.yml"))
						Expect(cfg.UseCache).To(BeTrue())
						receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
						receivedRwxDir = cfg.RwxDirectory
						return &api.InitiateRunResult{
							RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							TargetedTaskKeys: []string{},
							DefinitionPath:   ".mint/mint.yml",
						}, nil
					}
				})

				JustBeforeEach(func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("sends the file contents to cloud", func() {
					Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
					Expect(receivedRwxDir).NotTo(BeNil())
					Expect(receivedRwxDir[0].FileContents).To(Equal(""))
					Expect(receivedRwxDir[1].FileContents).To(Equal("some json"))
					Expect(receivedRwxDir[2].FileContents).To(Equal(originalRwxDirFileContent))
				})
			})

			Context("when an empty directory is found", func() {
				var originalSpecifiedFileContent string
				var receivedSpecifiedFileContent string

				BeforeEach(func() {
					originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec
					receivedSpecifiedFileContent = ""

					var err error

					workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
					err = os.MkdirAll(workingDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.Chdir(workingDir)
					Expect(err).NotTo(HaveOccurred())

					// note this is not in the heirarchy of the mint file or the working directory
					mintDir := filepath.Join(tmp, "other", "path", "to", ".mint")
					err = os.MkdirAll(mintDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = "mint.yml"
					runConfig.RwxDirectory = mintDir

					mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
						Expect(cfg.TaskDefinitions).To(HaveLen(1))
						Expect(cfg.TaskDefinitions[0].Path).To(Equal(runConfig.MintFilePath))
						Expect(cfg.RwxDirectory).To(HaveLen(1))
						Expect(cfg.RwxDirectory[0].Path).To(Equal(".mint"))
						Expect(cfg.UseCache).To(BeTrue())
						receivedSpecifiedFileContent = cfg.TaskDefinitions[0].FileContents
						return &api.InitiateRunResult{
							RunId:            "785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							RunURL:           "https://cloud.rwx.com/mint/rwx/runs/785ce4e8-17b9-4c8b-8869-a55e95adffe7",
							TargetedTaskKeys: []string{},
							DefinitionPath:   ".mint/mint.yml",
						}, nil
					}
				})

				JustBeforeEach(func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("sends the file contents to cloud", func() {
					Expect(receivedSpecifiedFileContent).To(Equal(originalSpecifiedFileContent))
				})
			})

			Context("when the 'directory' is actually a file", func() {
				BeforeEach(func() {
					var err error

					workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
					err = os.MkdirAll(workingDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.Chdir(workingDir)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte("yaml contents"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					mintDir := filepath.Join(workingDir, ".mint")
					err = os.WriteFile(mintDir, []byte("actually a file"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = "mint.yml"
					runConfig.RwxDirectory = mintDir
				})

				It("emits an error", func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("is not a directory"))
				})
			})

			Context("when the directory is not found", func() {
				var originalSpecifiedFileContent string

				BeforeEach(func() {
					originalSpecifiedFileContent = "tasks:\n  - key: foo\n    run: echo 'bar'\n" + baseSpec

					var err error

					workingDir := filepath.Join(tmp, "some", "path", "to", "working", "directory")
					err = os.MkdirAll(workingDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.Chdir(workingDir)
					Expect(err).NotTo(HaveOccurred())

					// note this is not in the heirarchy of the mint file or the working directory
					mintDir := filepath.Join(tmp, "other", "path", "to", ".mint")

					err = os.WriteFile(filepath.Join(workingDir, "mint.yml"), []byte(originalSpecifiedFileContent), 0o644)
					Expect(err).NotTo(HaveOccurred())

					runConfig.MintFilePath = "mint.yml"
					runConfig.RwxDirectory = mintDir
				})

				It("returns an error", func() {
					_, err := service.InitiateRun(runConfig)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to find .mint directory"))
				})
			})
		})

		Context("with no specific mint file and a specific directory", func() {
			BeforeEach(func() {
				runConfig.MintFilePath = ""
				runConfig.RwxDirectory = "some-dir"
			})

			It("returns an error", func() {
				_, err := service.InitiateRun(runConfig)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("the path to a mint-file must be provided"))
			})
		})
	})

	Describe("initiating a dispatch", func() {
		var dispatchConfig cli.InitiateDispatchConfig

		BeforeEach(func() {
			dispatchConfig = cli.InitiateDispatchConfig{}
		})

		Context("with valid dispatch parameters", func() {
			var originalParams map[string]string
			var receivedParams map[string]string

			BeforeEach(func() {
				originalParams = map[string]string{"key1": "value1", "key2": "value2"}
				receivedParams = nil

				dispatchConfig.DispatchKey = "test-dispatch-key"
				dispatchConfig.Params = originalParams
				dispatchConfig.Title = "Test Dispatch"
				dispatchConfig.Ref = "main"

				mockAPI.MockInitiateDispatch = func(cfg api.InitiateDispatchConfig) (*api.InitiateDispatchResult, error) {
					Expect(cfg.DispatchKey).To(Equal(dispatchConfig.DispatchKey))
					Expect(cfg.Params).To(Equal(originalParams))
					Expect(cfg.Title).To(Equal(dispatchConfig.Title))
					Expect(cfg.Ref).To(Equal(dispatchConfig.Ref))
					receivedParams = cfg.Params
					return &api.InitiateDispatchResult{DispatchId: "12345"}, nil
				}

				mockAPI.MockGetDispatch = func(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
					Expect(cfg.DispatchId).To(Equal("12345"))
					return &api.GetDispatchResult{
						Status: "ready",
						Runs: []api.GetDispatchRun{
							{RunId: "run-123", RunUrl: "https://example.com/run-123"},
						},
					}, nil
				}
			})

			It("calls the API and returns the dispatch ID", func() {
				dispatchResult, err := service.InitiateDispatch(dispatchConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(receivedParams).To(Equal(originalParams))
				Expect(dispatchResult.DispatchId).To(Equal("12345"))
			})
		})

		Context("with missing dispatch key", func() {
			BeforeEach(func() {
				dispatchConfig.DispatchKey = ""
			})

			It("returns a validation error", func() {
				_, err := service.InitiateDispatch(dispatchConfig)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("a dispatch key must be provided"))
			})
		})
	})

	Describe("getting a dispatch", func() {
		var dispatchConfig cli.GetDispatchConfig

		BeforeEach(func() {
			dispatchConfig = cli.GetDispatchConfig{}
		})

		Context("when the dispatch result is not ready", func() {
			BeforeEach(func() {
				dispatchConfig.DispatchId = "12345"

				mockAPI.MockGetDispatch = func(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
					return &api.GetDispatchResult{Status: "not_ready"}, nil
				}
			})

			It("returns a retry error", func() {
				_, err := service.GetDispatch(dispatchConfig)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, errors.ErrRetry)).To(BeTrue())
			})
		})

		Context("when the dispatch result contains an error", func() {
			BeforeEach(func() {
				dispatchConfig.DispatchId = "12345"

				mockAPI.MockGetDispatch = func(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
					return &api.GetDispatchResult{Status: "error", Error: "dispatch failed"}, nil
				}
			})

			It("returns the error", func() {
				_, err := service.GetDispatch(dispatchConfig)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dispatch failed"))
			})
		})

		Context("when the dispatch result succeeds", func() {
			BeforeEach(func() {
				dispatchConfig.DispatchId = "12345"

				mockAPI.MockGetDispatch = func(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
					return &api.GetDispatchResult{Status: "ready", Runs: []api.GetDispatchRun{api.GetDispatchRun{RunId: "runid", RunUrl: "runurl"}}}, nil
				}
			})

			It("returns the runs", func() {
				runs, err := service.GetDispatch(dispatchConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(runs[0].RunId).To(Equal("runid"))
				Expect(runs[0].RunUrl).To(Equal("runurl"))
			})
		})

		Context("when no runs are created", func() {
			BeforeEach(func() {
				dispatchConfig.DispatchId = "12345"

				mockAPI.MockGetDispatch = func(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
					return &api.GetDispatchResult{Status: "ready", Runs: []api.GetDispatchRun{}}, nil
				}
			})

			It("errors", func() {
				_, err := service.GetDispatch(dispatchConfig)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No runs were created as a result of this dispatch"))
			})
		})
	})

	Describe("debugging a task", func() {
		const (
			// The CLI will validate key material before connecting over SSH, hence we need some "real" keys here
			privateTestKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDiyT6ht8Z2XBEJpLR4/xmNouq5KDdn5G++cUcTH4EhzwAAAJhIWxlBSFsZ
QQAAAAtzc2gtZWQyNTUxOQAAACDiyT6ht8Z2XBEJpLR4/xmNouq5KDdn5G++cUcTH4Ehzw
AAAEC6442PQKevgYgeT0SIu9zwlnEMl6MF59ZgM+i0ByMv4eLJPqG3xnZcEQmktHj/GY2i
6rkoN2fkb75xRxMfgSHPAAAAEG1pbnQgQ0xJIHRlc3RpbmcBAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`
			publicTestKey = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLJPqG3xnZcEQmktHj/GY2i6rkoN2fkb75xRxMfgSHP mint CLI testing`
		)

		var (
			debugConfig                                                          cli.DebugTaskConfig
			agentAddress, runID                                                  string
			connectedViaSSH, fetchedConnectionInfo, interactiveSSHSessionStarted bool
		)

		BeforeEach(func() {
			agentAddress = fmt.Sprintf("%d.example.org:1234", GinkgoRandomSeed())
			connectedViaSSH = false
			fetchedConnectionInfo = false
			interactiveSSHSessionStarted = false
			runID = fmt.Sprintf("run-%d", GinkgoRandomSeed())

			debugConfig = cli.DebugTaskConfig{
				DebugKey: runID,
			}
		})

		Context("when the task is debuggable", func() {
			BeforeEach(func() {
				mockAPI.MockGetDebugConnectionInfo = func(runId string) (api.DebugConnectionInfo, error) {
					Expect(runID).To(Equal(runId))
					fetchedConnectionInfo = true
					// Note: This is returning a matching private & public key. The real API returns different ones
					return api.DebugConnectionInfo{Debuggable: true, PrivateUserKey: privateTestKey, PublicHostKey: publicTestKey, Address: agentAddress}, nil
				}

				mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
					Expect(addr).To(Equal(agentAddress))
					connectedViaSSH = true
					return nil
				}

				mockSSH.MockInteractiveSession = func() error {
					interactiveSSHSessionStarted = true
					return nil
				}
			})

			JustBeforeEach(func() {
				Expect(service.DebugTask(debugConfig)).To(Succeed())
			})

			It("fetches the connection info from the API", func() {
				Expect(fetchedConnectionInfo).To(BeTrue())
			})

			It("connects to the agent over SSH", func() {
				Expect(connectedViaSSH).To(BeTrue())
			})

			It("starts an interactive SSH session", func() {
				Expect(interactiveSSHSessionStarted).To(BeTrue())
			})
		})

		Context("when the task isn't debuggable yet", func() {
			var err error

			BeforeEach(func() {
				mockAPI.MockGetDebugConnectionInfo = func(runId string) (api.DebugConnectionInfo, error) {
					Expect(runID).To(Equal(runId))
					return api.DebugConnectionInfo{Debuggable: false}, nil
				}
			})

			JustBeforeEach(func() {
				err = service.DebugTask(debugConfig)
			})

			It("returns a 'Retry' error", func() {
				Expect(errors.Is(err, errors.ErrRetry)).To(BeTrue())
			})
		})
	})

	Describe("logging in", func() {
		var (
			tokenBackend accesstoken.Backend
		)

		BeforeEach(func() {
			var err error
			tokenBackend, err = accesstoken.NewMemoryBackend()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when unable to obtain an auth code", func() {
			BeforeEach(func() {
				mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
					Expect(oacc.Code.DeviceName).To(Equal("some-device"))
					return nil, errors.New("error in obtain auth code")
				}
			})

			It("returns an error", func() {
				err := service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
						return nil
					},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error in obtain auth code"))
			})
		})

		Context("with an auth code created", func() {
			BeforeEach(func() {
				mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
					Expect(oacc.Code.DeviceName).To(Equal("some-device"))
					return &api.ObtainAuthCodeResult{
						AuthorizationUrl: "https://cloud.local/_/auth/code?code=your-code",
						TokenUrl:         "https://cloud.local/api/auth/codes/code-uuid/token",
					}, nil
				}
			})

			Context("when polling results in authorized", func() {
				BeforeEach(func() {
					pollCounter := 0
					mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
						Expect(tokenUrl).To(Equal("https://cloud.local/api/auth/codes/code-uuid/token"))

						if pollCounter > 1 {
							pollCounter++
							return &api.AcquireTokenResult{State: "authorized", Token: "your-token"}, nil
						} else {
							pollCounter++
							return &api.AcquireTokenResult{State: "pending"}, nil
						}
					}
				})

				It("does not error", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})

					Expect(err).NotTo(HaveOccurred())
				})

				It("stores the token", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).NotTo(HaveOccurred())

					token, err := tokenBackend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("your-token"))
				})

				It("indicates success and help in case the browser does not open", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(mockStdout.String()).To(ContainSubstring("https://cloud.local/_/auth/code?code=your-code"))
					Expect(mockStdout.String()).To(ContainSubstring("Authorized!"))
				})

				It("attempts to open the authorization URL, but doesn't care if it fails", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return errors.New("couldn't open it")
						},
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(mockStdout.String()).To(ContainSubstring("https://cloud.local/_/auth/code?code=your-code"))
					Expect(mockStdout.String()).To(ContainSubstring("Authorized!"))
				})
			})

			Context("when polling results in consumed", func() {
				BeforeEach(func() {
					pollCounter := 0
					mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
						Expect(tokenUrl).To(Equal("https://cloud.local/api/auth/codes/code-uuid/token"))

						if pollCounter > 1 {
							pollCounter++
							return &api.AcquireTokenResult{State: "consumed"}, nil
						} else {
							pollCounter++
							return &api.AcquireTokenResult{State: "pending"}, nil
						}
					}
				})

				It("errors", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("already been used"))
				})

				It("does not store the token", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					token, err := tokenBackend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal(""))
				})

				It("does not indicate success, but still helps in case the browser does not open", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					Expect(mockStdout.String()).To(ContainSubstring("https://cloud.local/_/auth/code?code=your-code"))
					Expect(mockStdout.String()).NotTo(ContainSubstring("Authorized!"))
				})
			})

			Context("when polling results in expired", func() {
				BeforeEach(func() {
					pollCounter := 0
					mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
						Expect(tokenUrl).To(Equal("https://cloud.local/api/auth/codes/code-uuid/token"))

						if pollCounter > 1 {
							pollCounter++
							return &api.AcquireTokenResult{State: "expired"}, nil
						} else {
							pollCounter++
							return &api.AcquireTokenResult{State: "pending"}, nil
						}
					}
				})

				It("errors", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("has expired"))
				})

				It("does not store the token", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					token, err := tokenBackend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal(""))
				})

				It("does not indicate success, but still helps in case the browser does not open", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					Expect(mockStdout.String()).To(ContainSubstring("https://cloud.local/_/auth/code?code=your-code"))
					Expect(mockStdout.String()).NotTo(ContainSubstring("Authorized!"))
				})
			})

			Context("when polling results in something else", func() {
				BeforeEach(func() {
					pollCounter := 0
					mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
						Expect(tokenUrl).To(Equal("https://cloud.local/api/auth/codes/code-uuid/token"))

						if pollCounter > 1 {
							pollCounter++
							return &api.AcquireTokenResult{State: "unexpected-state-here-uh-oh"}, nil
						} else {
							pollCounter++
							return &api.AcquireTokenResult{State: "pending"}, nil
						}
					}
				})

				It("errors", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("is in an unexpected state"))
				})

				It("does not store the token", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					token, err := tokenBackend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal(""))
				})

				It("does not indicate success, but still helps in case the browser does not open", func() {
					err := service.Login(cli.LoginConfig{
						DeviceName:         "some-device",
						AccessTokenBackend: tokenBackend,
						OpenUrl: func(url string) error {
							Expect(url).To(Equal("https://cloud.local/_/auth/code?code=your-code"))
							return nil
						},
					})
					Expect(err).To(HaveOccurred())

					Expect(mockStdout.String()).To(ContainSubstring("https://cloud.local/_/auth/code?code=your-code"))
					Expect(mockStdout.String()).NotTo(ContainSubstring("Authorized!"))
				})
			})
		})
	})

	Describe("whoami", func() {
		Context("when outputting json", func() {
			Context("when the request fails", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						return nil, errors.New("uh oh can't figure out who you are")
					}
				})

				It("returns an error", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: true,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to determine details about the access token"))
					Expect(err.Error()).To(ContainSubstring("can't figure out who you are"))
				})
			})

			Context("when there is an email", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						email := "someone@rwx.com"
						return &api.WhoamiResult{
							TokenKind:        "personal_access_token",
							OrganizationSlug: "rwx",
							UserEmail:        &email,
						}, nil
					}
				})

				It("writes the token kind, organization, and user", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: true,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring(`"token_kind": "personal_access_token"`))
					Expect(mockStdout.String()).To(ContainSubstring(`"organization_slug": "rwx"`))
					Expect(mockStdout.String()).To(ContainSubstring(`"user_email": "someone@rwx.com"`))
				})
			})

			Context("when there is not an email", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						return &api.WhoamiResult{
							TokenKind:        "organization_access_token",
							OrganizationSlug: "rwx",
						}, nil
					}
				})

				It("writes the token kind and organization", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: true,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring(`"token_kind": "organization_access_token"`))
					Expect(mockStdout.String()).To(ContainSubstring(`"organization_slug": "rwx"`))
					Expect(mockStdout.String()).NotTo(ContainSubstring(`"user_email"`))
				})
			})
		})

		Context("when outputting plaintext", func() {
			Context("when the request fails", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						return nil, errors.New("uh oh can't figure out who you are")
					}
				})

				It("returns an error", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: false,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to determine details about the access token"))
					Expect(err.Error()).To(ContainSubstring("can't figure out who you are"))
				})
			})

			Context("when there is an email", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						email := "someone@rwx.com"
						return &api.WhoamiResult{
							TokenKind:        "personal_access_token",
							OrganizationSlug: "rwx",
							UserEmail:        &email,
						}, nil
					}
				})

				It("writes the token kind, organization, and user", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: false,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring("Token Kind: personal access token"))
					Expect(mockStdout.String()).To(ContainSubstring("Organization: rwx"))
					Expect(mockStdout.String()).To(ContainSubstring("User: someone@rwx.com"))
				})
			})

			Context("when there is not an email", func() {
				BeforeEach(func() {
					mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
						return &api.WhoamiResult{
							TokenKind:        "organization_access_token",
							OrganizationSlug: "rwx",
						}, nil
					}
				})

				It("writes the token kind and organization", func() {
					err := service.Whoami(cli.WhoamiConfig{
						Json: false,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring("Token Kind: organization access token"))
					Expect(mockStdout.String()).To(ContainSubstring("Organization: rwx"))
					Expect(mockStdout.String()).NotTo(ContainSubstring("User:"))
				})
			})
		})
	})

	Describe("setting secrets", func() {
		BeforeEach(func() {
			var err error
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when unable to set secrets", func() {
			BeforeEach(func() {
				mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
					Expect(ssivc.VaultName).To(Equal("default"))
					Expect(ssivc.Secrets[0].Name).To(Equal("ABC"))
					Expect(ssivc.Secrets[0].Secret).To(Equal("123"))
					return nil, errors.New("error setting secret")
				}
			})

			It("returns an error", func() {
				err := service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
					Vault:   "default",
					Secrets: []string{"ABC=123"},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error setting secret"))
			})
		})

		Context("with secrets set", func() {
			BeforeEach(func() {
				mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
					Expect(ssivc.VaultName).To(Equal("default"))
					Expect(ssivc.Secrets[0].Name).To(Equal("ABC"))
					Expect(ssivc.Secrets[0].Secret).To(Equal("123"))
					Expect(ssivc.Secrets[1].Name).To(Equal("DEF"))
					Expect(ssivc.Secrets[1].Secret).To(Equal("\"xyz\""))
					return &api.SetSecretsInVaultResult{
						SetSecrets: []string{"ABC", "DEF"},
					}, nil
				}
			})

			It("is successful", func() {
				err := service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
					Vault:   "default",
					Secrets: []string{"ABC=123", "DEF=\"xyz\""},
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(mockStdout.String()).To(Equal("\nSuccessfully set the following secrets: ABC, DEF"))
			})
		})

		Context("when reading secrets from a file", func() {
			var secretsFile string

			BeforeEach(func() {
				mockAPI.MockSetSecretsInVault = func(ssivc api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
					sort.Slice(ssivc.Secrets, func(i, j int) bool {
						return ssivc.Secrets[i].Name < ssivc.Secrets[j].Name
					})
					Expect(ssivc.VaultName).To(Equal("default"))
					Expect(ssivc.Secrets[0].Name).To(Equal("A"))
					Expect(ssivc.Secrets[0].Secret).To(Equal("123"))
					Expect(ssivc.Secrets[1].Name).To(Equal("B"))
					Expect(ssivc.Secrets[1].Secret).To(Equal("xyz"))
					Expect(ssivc.Secrets[2].Name).To(Equal("C"))
					Expect(ssivc.Secrets[2].Secret).To(Equal("q\\nqq"))
					Expect(ssivc.Secrets[3].Name).To(Equal("D"))
					Expect(ssivc.Secrets[3].Secret).To(Equal("a multiline\nstring\nspanning lines"))
					return &api.SetSecretsInVaultResult{
						SetSecrets: []string{"A", "B", "C", "D"},
					}, nil
				}

				var err error

				secretsFile = filepath.Join(tmp, "secrets.txt")
				err = os.WriteFile(secretsFile, []byte("A=123\nB=\"xyz\"\nC='q\\nqq'\nD=\"a multiline\nstring\nspanning lines\""), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("is successful", func() {
				err := service.SetSecretsInVault(cli.SetSecretsInVaultConfig{
					Vault:   "default",
					Secrets: []string{},
					File:    secretsFile,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(mockStdout.String()).To(Equal("\nSuccessfully set the following secrets: A, B, C, D"))
			})
		})
	})

	Describe("updating leaves", func() {
		Context("when no files provided", func() {
			Context("when no yaml files found in the default directory", func() {
				var mintDir string

				BeforeEach(func() {
					var err error

					mintDir = tmp

					err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					err := service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{},
						RwxDirectory:             mintDir,
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("no files provided, and no yaml files found in directory %s", mintDir)))
				})
			})

			Context("when yaml files are found in the specified directory", func() {
				var mintDir string

				BeforeEach(func() {
					var err error

					mintDir = tmp

					err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())

					nestedDir := filepath.Join(mintDir, "some", "nested", "dir")
					err = os.MkdirAll(nestedDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				BeforeEach(func() {
					mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
						return &api.LeafVersionsResult{
							LatestMajor: map[string]string{"mint/setup-node": "1.3.0"},
						}, nil
					}
				})

				It("uses the default directory", func() {
					var err error

					err = service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{},
						RwxDirectory:             mintDir,
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.3.0"))

					contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.3.0"))

					contents, err = os.ReadFile(filepath.Join(mintDir, "some", "nested", "dir", "tasks.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.3.0"))
				})
			})
		})

		Context("with files", func() {
			var majorLeafVersions map[string]string
			var minorLeafVersions map[string]map[string]string
			var leafError error

			BeforeEach(func() {
				majorLeafVersions = make(map[string]string)
				minorLeafVersions = make(map[string]map[string]string)
				leafError = nil

				mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
					return &api.LeafVersionsResult{
						LatestMajor: majorLeafVersions,
						LatestMinor: minorLeafVersions,
					}, leafError
				}
			})

			Context("when the leaf versions cannot be retrieved", func() {
				BeforeEach(func() {
					leafError = errors.New("cannot get leaf versions")

					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(""), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					err := service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot get leaf versions"))
				})
			})

			Context("when all leaves are already up-to-date", func() {
				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "1.2.3"

					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not change the file content", func() {
					var err error

					err = service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`))
				})

				It("indicates no leaves were updated", func() {
					err := service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring("No leaves to update."))
				})
			})

			Context("when there are leaves to update across multiple files", func() {
				var originalFooContents string
				var originalBarContents string

				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "1.2.3"
					majorLeafVersions["mint/setup-ruby"] = "1.0.1"
					majorLeafVersions["mint/setup-go"] = "1.3.5"

					minorLeafVersions["mint/setup-node"] = map[string]string{
						"1": "1.2.3",
					}
					minorLeafVersions["mint/setup-ruby"] = map[string]string{
						"0": "0.0.2",
						"1": "1.0.1",
					}
					minorLeafVersions["mint/setup-go"] = map[string]string{
						"1": "1.3.5",
					}

					var err error

					originalFooContents = `
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
  - key: bar
    call: mint/setup-ruby 0.0.1
  - key: baz
    call: mint/setup-go
`
					err = os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
					Expect(err).NotTo(HaveOccurred())

					originalBarContents = `
tasks:
  - key: foo
    call: mint/setup-ruby 1.0.0
`
					err = os.WriteFile(filepath.Join(tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("with major version updates", func() {
					It("updates all files", func() {
						var err error

						err = service.UpdateLeaves(cli.UpdateLeavesConfig{
							Files:                    []string{filepath.Join(tmp, "foo.yaml"), filepath.Join(tmp, "bar.yaml")},
							ReplacementVersionPicker: cli.PickLatestMajorVersion,
						})
						Expect(err).NotTo(HaveOccurred())

						var contents []byte

						contents, err = os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 1.0.1
  - key: baz
    call: mint/setup-go 1.3.5
`))

						contents, err = os.ReadFile(filepath.Join(tmp, "bar.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`))
					})

					It("indicates leaves were updated", func() {
						err := service.UpdateLeaves(cli.UpdateLeavesConfig{
							Files:                    []string{filepath.Join(tmp, "foo.yaml"), filepath.Join(tmp, "bar.yaml")},
							ReplacementVersionPicker: cli.PickLatestMajorVersion,
						})

						Expect(err).NotTo(HaveOccurred())
						Expect(mockStdout.String()).To(ContainSubstring("Updated the following leaves:"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-go → 1.3.5"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-node 1.0.1 → 1.2.3"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-ruby 0.0.1 → 1.0.1"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-ruby 1.0.0 → 1.0.1"))
					})
				})

				Context("with minor version updates only", func() {
					It("updates all files", func() {
						var err error

						err = service.UpdateLeaves(cli.UpdateLeavesConfig{
							Files:                    []string{filepath.Join(tmp, "foo.yaml"), filepath.Join(tmp, "bar.yaml")},
							ReplacementVersionPicker: cli.PickLatestMinorVersion,
						})
						Expect(err).NotTo(HaveOccurred())

						var contents []byte

						contents, err = os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 0.0.2
  - key: baz
    call: mint/setup-go 1.3.5
`))

						contents, err = os.ReadFile(filepath.Join(tmp, "bar.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`))
					})

					It("indicates leaves were updated", func() {
						err := service.UpdateLeaves(cli.UpdateLeavesConfig{
							Files:                    []string{filepath.Join(tmp, "foo.yaml"), filepath.Join(tmp, "bar.yaml")},
							ReplacementVersionPicker: cli.PickLatestMinorVersion,
						})

						Expect(err).NotTo(HaveOccurred())
						Expect(mockStdout.String()).To(ContainSubstring("Updated the following leaves:"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-go → 1.3.5"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-node 1.0.1 → 1.2.3"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-ruby 0.0.1 → 0.0.2"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-ruby 1.0.0 → 1.0.1"))
					})
				})

				Context("when a single file is targeted", func() {
					It("updates only the targeted file", func() {
						var err error

						err = service.UpdateLeaves(cli.UpdateLeavesConfig{
							Files:                    []string{filepath.Join(tmp, "bar.yaml")},
							ReplacementVersionPicker: cli.PickLatestMajorVersion,
						})
						Expect(err).NotTo(HaveOccurred())

						var contents []byte

						contents, err = os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(originalFooContents))

						contents, err = os.ReadFile(filepath.Join(tmp, "bar.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`))
					})
				})

				It("updates snippet files", func() {
					var err error

					mintDir := filepath.Join(tmp, ".mint")
					err = os.MkdirAll(mintDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					originalBazContents := `
# leading commment
- key: foo
  call: mint/setup-node 1.0.1
- key: bar
  call: mint/setup-go
`

					err = os.WriteFile(filepath.Join(mintDir, "_baz.yaml"), []byte(originalBazContents), 0o644)
					Expect(err).NotTo(HaveOccurred())

					originalQuxContents := `
- not
- a
- list
- of
- tasks
`

					err = os.WriteFile(filepath.Join(mintDir, "_qux.yaml"), []byte(originalQuxContents), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = service.UpdateLeaves(cli.UpdateLeavesConfig{
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(mintDir, "_baz.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`# leading commment
- key: foo
  call: mint/setup-node 1.2.3
- key: bar
  call: mint/setup-go 1.3.5
`))

					contents, err = os.ReadFile(filepath.Join(mintDir, "_qux.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(originalQuxContents))
				})
			})

			Context("when a leaf cannot be found", func() {
				BeforeEach(func() {
					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not modify the file", func() {
					var err error

					err = service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`
tasks:
  - key: foo
    call: mint/setup-node 1.0.1
`))
				})

				It("indicates a leaf could not be found", func() {
					err := service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStderr.String()).To(ContainSubstring(`Unable to find the leaf "mint/setup-node"; skipping it.`))
				})
			})

			Context("when a leaf reference is a later version than the latest major", func() {
				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "1.0.3"

					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("updates the leaf", func() {
					var err error

					err = service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-node 1.0.3
`))
				})
			})

			Context("when a leaf reference is a major version behind the latest", func() {
				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "2.0.3"
					minorLeafVersions["mint/setup-node"] = make(map[string]string)
					minorLeafVersions["mint/setup-node"]["2"] = "2.0.3"
					minorLeafVersions["mint/setup-node"]["1"] = "1.1.1"
				})

				JustBeforeEach(func() {
					Expect(service.UpdateLeaves(cli.UpdateLeavesConfig{
						Files:                    []string{filepath.Join(tmp, "foo.yaml")},
						ReplacementVersionPicker: cli.PickLatestMinorVersion,
					})).To(Succeed())
				})

				Context("while referencing the latest minor version", func() {
					BeforeEach(func() {
						err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`), 0o644)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not modify the file", func() {
						contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`
tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`))
					})

					It("indicates no leaves were updated", func() {
						Expect(mockStdout.String()).To(ContainSubstring("No leaves to update."))
					})
				})

				Context("while not referencing the latest minor version", func() {
					BeforeEach(func() {
						err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.0.9
`), 0o644)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates the file", func() {
						contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-node 1.1.1
`))
					})

					It("indicates that a leaf was updated", func() {
						Expect(mockStdout.String()).To(ContainSubstring("Updated the following leaves:"))
						Expect(mockStdout.String()).To(ContainSubstring("mint/setup-node 1.0.9 → 1.1.1"))
					})
				})
			})
		})
	})

	Describe("resolving leaves", func() {
		Context("when no files provided", func() {
			Context("when no yaml files found in the default directory", func() {
				var mintDir string

				BeforeEach(func() {
					var err error

					mintDir = tmp

					err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        mintDir,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("no files provided, and no yaml files found in directory %s", mintDir)))
				})
			})

			Context("when yaml files are found in the specified directory", func() {
				var mintDir string

				BeforeEach(func() {
					var err error

					mintDir = tmp

					err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node
`), 0o644)
					Expect(err).NotTo(HaveOccurred())

					nestedDir := filepath.Join(mintDir, "some", "nested", "dir")
					err = os.MkdirAll(nestedDir, 0o755)
					Expect(err).NotTo(HaveOccurred())

					err = os.WriteFile(filepath.Join(nestedDir, "tasks.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				BeforeEach(func() {
					mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
						return &api.LeafVersionsResult{
							LatestMajor: map[string]string{"mint/setup-node": "1.3.0"},
						}, nil
					}
				})

				It("uses the default directory", func() {
					var err error

					_, err = service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        mintDir,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.2.3"))

					contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.3.0"))

					contents, err = os.ReadFile(filepath.Join(mintDir, "some", "nested", "dir", "tasks.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(ContainSubstring("mint/setup-node 1.3.0"))
				})
			})
		})

		Context("with files", func() {
			var majorLeafVersions map[string]string
			var minorLeafVersions map[string]map[string]string
			var leafError error

			BeforeEach(func() {
				majorLeafVersions = make(map[string]string)
				minorLeafVersions = make(map[string]map[string]string)
				leafError = nil

				mockAPI.MockGetLeafVersions = func() (*api.LeafVersionsResult, error) {
					return &api.LeafVersionsResult{
						LatestMajor: majorLeafVersions,
						LatestMinor: minorLeafVersions,
					}, leafError
				}
			})

			Context("when the leaf versions cannot be retrieved", func() {
				BeforeEach(func() {
					leafError = errors.New("cannot get leaf versions")

					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(""), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        tmp,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot get leaf versions"))
				})
			})

			Context("when all leaves have a version", func() {
				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "1.3.0"

					err := os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not change the file content", func() {
					var err error

					_, err = service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        tmp,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`
tasks:
  - key: foo
    call: mint/setup-node 1.2.3
`))
				})

				It("indicates no leaves were resolved", func() {
					_, err := service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        tmp,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring("No leaves to resolve."))
				})
			})

			Context("when there are leaves to resolve across multiple files", func() {
				var originalFooContents string
				var originalBarContents string

				BeforeEach(func() {
					majorLeafVersions["mint/setup-node"] = "1.2.3"
					majorLeafVersions["mint/setup-ruby"] = "1.0.1"
					majorLeafVersions["mint/setup-go"] = "1.3.5"

					var err error

					originalFooContents = `
tasks:
  - key: foo
    call: mint/setup-node
  - key: bar
    call: mint/setup-ruby 0.0.1
  - key: baz
    call: mint/setup-go
`
					err = os.WriteFile(filepath.Join(tmp, "foo.yaml"), []byte(originalFooContents), 0o644)
					Expect(err).NotTo(HaveOccurred())

					originalBarContents = `
tasks:
  - key: foo
    call: mint/setup-ruby
`
					err = os.WriteFile(filepath.Join(tmp, "bar.yaml"), []byte(originalBarContents), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("updates all files", func() {
					var err error

					_, err = service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        tmp,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})
					Expect(err).NotTo(HaveOccurred())

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(tmp, "foo.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-node 1.2.3
  - key: bar
    call: mint/setup-ruby 0.0.1
  - key: baz
    call: mint/setup-go 1.3.5
`))

					contents, err = os.ReadFile(filepath.Join(tmp, "bar.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`))
				})

				It("indicates leaves were resolved", func() {
					_, err := service.ResolveLeaves(cli.ResolveLeavesConfig{
						RwxDirectory:        tmp,
						LatestVersionPicker: cli.PickLatestMajorVersion,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(ContainSubstring("Resolved the following leaves:"))
					Expect(mockStdout.String()).To(ContainSubstring("mint/setup-go → 1.3.5"))
					Expect(mockStdout.String()).To(ContainSubstring("mint/setup-node → 1.2.3"))
					Expect(mockStdout.String()).To(ContainSubstring("mint/setup-ruby → 1.0.1"))
				})

				Context("when a single file is targeted", func() {
					It("resolves only the targeted file", func() {
						var err error

						_, err = service.ResolveLeaves(cli.ResolveLeavesConfig{
							RwxDirectory:        tmp,
							Files:               []string{filepath.Join(tmp, "bar.yaml")},
							LatestVersionPicker: cli.PickLatestMajorVersion,
						})
						Expect(err).NotTo(HaveOccurred())

						contents, err := os.ReadFile(filepath.Join(tmp, "foo.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(originalFooContents))

						contents, err = os.ReadFile(filepath.Join(tmp, "bar.yaml"))
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal(`tasks:
  - key: foo
    call: mint/setup-ruby 1.0.1
`))
					})
				})
			})
		})
	})

	Describe("resolving base layers", func() {
		var (
			apiOs        string
			apiTag       string
			apiArch      string
			apiCallCount int
			apiError     func(callCount int) error
			workingDir   string
			mintDir      string
		)

		BeforeEach(func() {
			workingDir = filepath.Join(tmp, "subdir1/subdir2")
			err := os.MkdirAll(workingDir, 0o755)
			Expect(err).NotTo(HaveOccurred())

			mintDir = filepath.Join(tmp, "subdir1/.mint")
			err = os.MkdirAll(mintDir, 0o755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())

			apiOs = "gentoo 99"
			apiTag = "1.2"
			apiArch = "x86_64"
			apiCallCount = 0
			apiError = func(callCount int) error { return nil }

			mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				apiCallCount += 1
				if err := apiError(apiCallCount); err != nil {
					return api.ResolveBaseLayerResult{}, err
				}

				os := cfg.Os
				if os == "" {
					os = apiOs
				}
				tag := cfg.Tag
				if tag == "" {
					tag = apiTag
				}
				arch := cfg.Arch
				if arch == "" {
					arch = apiArch
				}
				return api.ResolveBaseLayerResult{
					Os:   os,
					Tag:  tag,
					Arch: arch,
				}, nil
			}
		})

		Context("when no yaml files found in the default directory", func() {
			BeforeEach(func() {
				var err error

				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := service.ResolveBase(cli.ResolveBaseConfig{})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("no files provided, and no yaml files found in directory %s", mintDir)))
			})
		})

		Context("when yaml file is actually json", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`{
"tasks": [
  { "key": "a" },
  { "key": "b" }
]
}`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("ignores the file", func() {
				_, err := service.ResolveBase(cli.ResolveBaseConfig{})

				Expect(err).NotTo(HaveOccurred())
				Expect(mockStderr.String()).To(Equal(""))
				Expect(mockStdout.String()).To(ContainSubstring("No run files were missing base"))
			})
		})

		Context("when yaml file doesn't include base", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds base to file", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{
					Arch: "quantum",
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  arch: quantum
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml → gentoo 99, tag 1.2",
				)))

				// yaml file without tasks key is unaffected
				contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`))
			})

			It("adds base to only a targeted file", func() {
				var err error

				originalQuxContents := `
tasks:
  - key: a
  - key: b
`
				err = os.WriteFile(filepath.Join(mintDir, "qux.yaml"), []byte(originalQuxContents), 0o644)
				Expect(err).NotTo(HaveOccurred())

				_, err = service.ResolveBase(cli.ResolveBaseConfig{
					Files: []string{"../.mint/bar.yaml"},
					Arch:  "quantum",
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  arch: quantum
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml → gentoo 99, tag 1.2",
				)))

				contents, err = os.ReadFile(filepath.Join(mintDir, "qux.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(originalQuxContents))
			})

			It("errors when given a file that does not exist", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{
					Files: []string{"does-not-exist.yaml"},
					Arch:  "quantum",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("reading mint directory entries at does-not-exist.yaml: file does not exist"))
			})
		})

		Context("when yaml file has a base with os but no tag or arch", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
  os: gentoo 99

tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds tag to base", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`on:
  github:
    push: {}

base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n",
					"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
				)))
			})
		})

		Context("when yaml file has a base with os and arch but no tag", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum

tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds tag to base", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum
  tag: 1.2

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n",
					"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
				)))
			})
		})

		Context("when yaml file has base after tasks with os but no tag", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds tag to base without moving base before tasks", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99
  tag: 1.2
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n",
					"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
				)))
			})
		})

		Context("with multiple yaml files", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "one.yaml"), []byte(`tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "two.yaml"), []byte(`base:
  os: gentoo 88

tasks:
  - key: c
  - key: d
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "three.yaml"), []byte(`tasks:
  - key: e
  - key: f
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates all files", func() {
				var err error

				_, err = service.ResolveBase(cli.ResolveBaseConfig{
					Os: "gentoo 99",
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "one.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`))

				contents, err = os.ReadFile(filepath.Join(mintDir, "two.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 88
  tag: 1.2

tasks:
  - key: c
  - key: d
`))

				contents, err = os.ReadFile(filepath.Join(mintDir, "three.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: e
  - key: f
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Added base to the following run definitions:\n%s\n%s\n%s\n",
					"\t../.mint/one.yaml → gentoo 99, tag 1.2",
					"\t../.mint/three.yaml → gentoo 99, tag 1.2",
					"\t../.mint/two.yaml → gentoo 88, tag 1.2",
				)))
			})

			Context("when an API request fails", func() {
				It("doesn't update any files", func() {
					contentsOne, err := os.ReadFile(filepath.Join(mintDir, "one.yaml"))
					Expect(err).NotTo(HaveOccurred())
					contentsTwo, err := os.ReadFile(filepath.Join(mintDir, "two.yaml"))
					Expect(err).NotTo(HaveOccurred())
					contentsThree, err := os.ReadFile(filepath.Join(mintDir, "three.yaml"))
					Expect(err).NotTo(HaveOccurred())

					apiError = func(callCount int) error {
						if callCount == 2 {
							return errors.New("API request failed")
						}
						return nil
					}

					_, err = service.ResolveBase(cli.ResolveBaseConfig{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("API request failed"))

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(mintDir, "one.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsOne)))

					contents, err = os.ReadFile(filepath.Join(mintDir, "two.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsTwo)))

					contents, err = os.ReadFile(filepath.Join(mintDir, "three.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsThree)))
				})
			})
		})
	})

	Describe("updating base layers", func() {
		var (
			apiOs        string
			apiTag       string
			apiArch      string
			apiCallCount int
			apiError     func(callCount int) error
			workingDir   string
			mintDir      string
		)

		BeforeEach(func() {
			workingDir = filepath.Join(tmp, "subdir1/subdir2")
			err := os.MkdirAll(workingDir, 0o755)
			Expect(err).NotTo(HaveOccurred())

			mintDir = filepath.Join(tmp, "subdir1/.mint")
			err = os.MkdirAll(mintDir, 0o755)
			Expect(err).NotTo(HaveOccurred())

			err = os.Chdir(workingDir)
			Expect(err).NotTo(HaveOccurred())

			apiOs = "gentoo 99"
			apiTag = "1.5"
			apiArch = "x86_64"
			apiCallCount = 0
			apiError = func(callCount int) error { return nil }

			mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
				apiCallCount += 1
				if err := apiError(apiCallCount); err != nil {
					return api.ResolveBaseLayerResult{}, err
				}

				os := cfg.Os
				if os == "" {
					os = apiOs
				}
				tag := cfg.Tag
				if tag == "" || string(tag[0]) == "1" {
					tag = apiTag
				} else {
					return api.ResolveBaseLayerResult{}, errors.Wrap(api.ErrNotFound, "unknown base tag")
				}
				arch := cfg.Arch
				if arch == "" {
					arch = apiArch
				}
				return api.ResolveBaseLayerResult{
					Os:   os,
					Tag:  tag,
					Arch: arch,
				}, nil
			}
		})

		Context("when no yaml files found in the default directory", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "bar.json"), []byte("some json"), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := service.UpdateBase(cli.UpdateBaseConfig{})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no files provided, and no yaml files found"))
			})
		})

		Context("when yaml file is actually json", func() {
			var mintDir string

			BeforeEach(func() {
				var err error

				mintDir = tmp

				err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`{
"tasks": [
  { "key": "a" },
  { "key": "b" }
]
}`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := service.UpdateBase(cli.UpdateBaseConfig{})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no files provided, and no yaml files found"))
			})
		})

		Context("when yaml file doesn't include a base", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds base to file", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml → tag 1.5",
				)))

				// yaml file without tasks key is unaffected
				contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`))
			})

			It("adds base to only a targeted file", func() {
				var err error

				originalQuxContents := `
tasks:
  - key: a
  - key: b
`
				err = os.WriteFile(filepath.Join(mintDir, "qux.yaml"), []byte(originalQuxContents), 0o644)
				Expect(err).NotTo(HaveOccurred())

				_, err = service.UpdateBase(cli.UpdateBaseConfig{
					Files: []string{"../.mint/bar.yaml"},
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml → tag 1.5",
				)))

				contents, err = os.ReadFile(filepath.Join(mintDir, "qux.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(originalQuxContents))
			})
		})

		Context("when yaml file includes an older base", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "foo.txt"), []byte("some txt"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "bar.yaml"), []byte(`
base:
  os: gentoo 99
  tag: 1.1

tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "baz.yaml"), []byte(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates base tag", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml tag 1.1 → tag 1.5",
				)))

				// yaml file without tasks key is unaffected
				contents, err = os.ReadFile(filepath.Join(mintDir, "baz.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`))
			})

			It("updates base for only a targeted file", func() {
				var err error

				originalQuxContents := `
tasks:
  - key: a
  - key: b
`
				err = os.WriteFile(filepath.Join(mintDir, "qux.yaml"), []byte(originalQuxContents), 0o644)
				Expect(err).NotTo(HaveOccurred())

				_, err = service.UpdateBase(cli.UpdateBaseConfig{
					Files: []string{"../.mint/bar.yaml"},
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "bar.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/bar.yaml tag 1.1 → tag 1.5",
				)))

				contents, err = os.ReadFile(filepath.Join(mintDir, "qux.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(originalQuxContents))
			})
		})

		Context("when yaml file includes base tag newer than the server knows about", func() {
			var originalContents string

			BeforeEach(func() {
				var err error

				originalContents = `
base:
  os: gentoo 99
  tag: 2.0

tasks:
  - key: a
  - key: b
`

				err = os.WriteFile(filepath.Join(workingDir, "ci.yaml"), []byte(originalContents), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors without changing file", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{
					Files: []string{"ci.yaml"},
				})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(workingDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(originalContents))

				Expect(mockStderr.String()).To(Equal("Unknown base tag 2 for run definitions: ci.yaml\n"))
				Expect(mockStdout.String()).To(Equal("No run bases to update.\n"))
			})
		})

		Context("when yaml file has a base with os and arch but no tag", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum

tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds tag to base", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/ci.yaml → tag 1.5",
				)))
			})
		})

		Context("when yaml file has base after tasks with os but no tag", func() {
			BeforeEach(func() {
				err := os.WriteFile(filepath.Join(mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds tag to base without moving base before tasks", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "ci.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99
  tag: 1.5
`))

				Expect(mockStdout.String()).To(Equal(fmt.Sprintf(
					"Updated base for the following run definitions:\n%s\n",
					"\t../.mint/ci.yaml → tag 1.5",
				)))
			})
		})

		Context("with multiple yaml files", func() {
			BeforeEach(func() {
				var err error

				err = os.WriteFile(filepath.Join(mintDir, "one.yaml"), []byte(`base:
  os: gentoo 99
  tag: 1.1

tasks:
  - key: a
  - key: b
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "two.yaml"), []byte(`base:
  os: gentoo 88

tasks:
  - key: c
  - key: d
`), 0o644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(mintDir, "three.yaml"), []byte(`tasks:
  - key: e
  - key: f
`), 0o644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates all files", func() {
				var err error

				_, err = service.UpdateBase(cli.UpdateBaseConfig{})
				Expect(err).NotTo(HaveOccurred())

				var contents []byte

				contents, err = os.ReadFile(filepath.Join(mintDir, "one.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: a
  - key: b
`))

				contents, err = os.ReadFile(filepath.Join(mintDir, "two.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 88
  tag: 1.5

tasks:
  - key: c
  - key: d
`))

				contents, err = os.ReadFile(filepath.Join(mintDir, "three.yaml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(`base:
  os: gentoo 99
  tag: 1.5

tasks:
  - key: e
  - key: f
`))

				Expect(mockStdout.String()).To(ContainSubstring("Updated base for the following run definitions:"))
				Expect(mockStdout.String()).To(ContainSubstring("\t../.mint/one.yaml tag 1.1 → tag 1.5"))
				Expect(mockStdout.String()).To(ContainSubstring("\t../.mint/two.yaml → tag 1.5"))
				Expect(mockStdout.String()).To(ContainSubstring("\t../.mint/three.yaml → tag 1.5"))
			})

			Context("when an API request fails", func() {
				It("doesn't update any files", func() {
					contentsOne, err := os.ReadFile(filepath.Join(mintDir, "one.yaml"))
					Expect(err).NotTo(HaveOccurred())
					contentsTwo, err := os.ReadFile(filepath.Join(mintDir, "two.yaml"))
					Expect(err).NotTo(HaveOccurred())
					contentsThree, err := os.ReadFile(filepath.Join(mintDir, "three.yaml"))
					Expect(err).NotTo(HaveOccurred())

					apiError = func(callCount int) error {
						if callCount == 2 {
							return errors.New("API request failed")
						}
						return nil
					}

					_, err = service.UpdateBase(cli.UpdateBaseConfig{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("API request failed"))

					var contents []byte

					contents, err = os.ReadFile(filepath.Join(mintDir, "one.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsOne)))

					contents, err = os.ReadFile(filepath.Join(mintDir, "two.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsTwo)))

					contents, err = os.ReadFile(filepath.Join(mintDir, "three.yaml"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal(string(contentsThree)))
				})
			})
		})
	})

	Describe("linting", func() {
		var truncatedDiff bool
		var lintConfig cli.LintConfig

		BeforeEach(func() {
			truncatedDiff = format.TruncatedDiff
			format.TruncatedDiff = false

			Expect(os.MkdirAll(filepath.Join(tmp, "some/path/to/.mint"), 0o755)).NotTo(HaveOccurred())
			Expect(os.Chdir(filepath.Join(tmp, "some/path/to"))).NotTo(HaveOccurred())

			lintConfig = cli.LintConfig{OutputFormat: cli.LintOutputNone}
		})

		AfterEach(func() {
			format.TruncatedDiff = truncatedDiff
		})

		Context("with multiple errors", func() {
			BeforeEach(func() {
				Expect(os.WriteFile("mint1.yml", []byte("mint1 contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)).NotTo(HaveOccurred())

				lintConfig.MintFilePaths = []string{"mint1.yml", ".mint/base.yml"}

				mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
					Expect(cfg.TaskDefinitions).To(HaveLen(2))
					return &api.LintResult{
						Problems: []api.LintProblem{
							{Severity: "error", Message: "message 1\nmessage 1a", FileName: "mint1.yml", Line: api.NewNullInt(11), Column: api.NewNullInt(22), Advice: "advice 1\nadvice 1a"},
							{Severity: "error", Message: "message 2\nmessage 2a", FileName: "mint1.yml", Line: api.NewNullInt(15), Column: api.NewNullInt(4)},
							{Severity: "warning", Message: "message 3", FileName: "mint1.yml", Line: api.NewNullInt(2), Column: api.NewNullInt(6), Advice: "advice 3\nadvice 3a"},
							{Severity: "warning", Message: "message 4", FileName: "mint1.yml", Line: api.NullInt{IsNull: true}, Column: api.NullInt{IsNull: true}},
						},
					}, nil
				}
			})

			Context("using oneline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputOneLine
				})

				It("lists only files", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(`error   mint1.yml:11:22 - message 1 message 1a
error   mint1.yml:15:4 - message 2 message 2a
warning mint1.yml:2:6 - message 3
warning mint1.yml - message 4
`))
				})
			})

			Context("using multiline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputMultiLine
				})

				It("lists all the data from the problem", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(`
mint1.yml:11:22  [error]
message 1
message 1a
advice 1
advice 1a

mint1.yml:15:4  [error]
message 2
message 2a

mint1.yml:2:6  [warning]
message 3
advice 3
advice 3a

mint1.yml  [warning]
message 4

Checked 2 files and found 4 problems.
`))
				})
			})

			Context("using none output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputNone
				})

				It("doesn't output", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(""))
				})
			})
		})

		Context("with multiple errors including stack traces", func() {
			BeforeEach(func() {
				Expect(os.WriteFile("mint1.yml", []byte("mint1 contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)).NotTo(HaveOccurred())

				lintConfig.MintFilePaths = []string{"mint1.yml", ".mint/base.yml"}

				mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
					Expect(cfg.TaskDefinitions).To(HaveLen(2))
					return &api.LintResult{
						Problems: []api.LintProblem{
							{
								Severity: "error",
								Message:  "message 1\nmessage 1a",
								StackTrace: []messages.StackEntry{
									{
										FileName: "mint1.yml",
										Line:     11,
										Column:   22,
									},
								},
								Frame:  "  4 |     run: echo hi\n> 5 |     bad: true\n    |     ^\n  6 |     env:\n  7 |       A:",
								Advice: "advice 1\nadvice 1a",
							},
							{
								Severity: "error",
								Message:  "message 2\nmessage 2a",
								StackTrace: []messages.StackEntry{
									{
										FileName: "mint1.yml",
										Line:     22,
										Column:   11,
										Name:     "*alias",
									},
									{
										FileName: "mint1.yml",
										Line:     5,
										Column:   22,
									},
								},
							},
							{
								Severity: "warning",
								Message:  "message 3",
								StackTrace: []messages.StackEntry{
									{
										FileName: "mint1.yml",
										Line:     2,
										Column:   6,
									},
								},
								Advice: "advice 3\nadvice 3a",
							},
							{
								Severity: "warning",
								Message:  "message 4",
								StackTrace: []messages.StackEntry{
									{
										FileName: "mint1.yml",
										Line:     7,
										Column:   9,
									},
								},
							},
						},
					}, nil
				}
			})

			Context("using oneline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputOneLine
				})

				It("lists only files", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(`error   mint1.yml:11:22 - message 1 message 1a
error   mint1.yml:5:22 - message 2 message 2a
warning mint1.yml:2:6 - message 3
warning mint1.yml:7:9 - message 4
`))
				})
			})

			Context("using multiline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputMultiLine
				})

				It("lists all the data from the problem", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(`
[error] message 1
message 1a
  4 |     run: echo hi
> 5 |     bad: true
    |     ^
  6 |     env:
  7 |       A:
  at mint1.yml:11:22
advice 1
advice 1a

[error] message 2
message 2a
  at mint1.yml:5:22
  at *alias (mint1.yml:22:11)

[warning] message 3
  at mint1.yml:2:6
advice 3
advice 3a

[warning] message 4
  at mint1.yml:7:9

Checked 2 files and found 4 problems.
`))
				})
			})

			Context("using none output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputNone
				})

				It("doesn't output", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(""))
				})
			})
		})

		Context("with no errors", func() {
			BeforeEach(func() {
				Expect(os.WriteFile("mint1.yml", []byte("mint1 contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)).NotTo(HaveOccurred())

				lintConfig.MintFilePaths = []string{"mint1.yml", ".mint/base.yml"}

				mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
					Expect(cfg.TaskDefinitions).To(HaveLen(2))
					return &api.LintResult{}, nil
				}
			})

			Context("using oneline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputOneLine
				})

				It("doesn't output", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(""))
				})
			})

			Context("using multiline output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputMultiLine
				})

				It("outputs check counts", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal("\nChecked 2 files and found 0 problems.\n"))
				})
			})

			Context("using none output", func() {
				BeforeEach(func() {
					lintConfig.OutputFormat = cli.LintOutputNone
				})

				It("doesn't output", func() {
					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockStdout.String()).To(Equal(""))
				})
			})
		})

		Context("with snippets", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(".mint/base1.yml", []byte(".mint/base1.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base2.yml", []byte(".mint/base2.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/_snippet1.yml", []byte(".mint/_snippet1.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/_snippet2.yml", []byte(".mint/_snippet2.yml contents"), 0o644)).NotTo(HaveOccurred())

				lintConfig.OutputFormat = cli.LintOutputOneLine
			})

			Context("without targeting", func() {
				It("doesn't target the snippets", func() {
					mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
						runDefinitionPaths := make([]string, len(cfg.TaskDefinitions))
						for i, runDefinitionPath := range cfg.TaskDefinitions {
							runDefinitionPaths[i] = runDefinitionPath.Path
						}
						Expect(runDefinitionPaths).To(ConsistOf([]string{".mint/base1.yml", ".mint/base2.yml", ".mint/_snippet1.yml", ".mint/_snippet2.yml"}))
						Expect(cfg.TargetPaths).To(ConsistOf([]string{".mint/base1.yml", ".mint/base2.yml"}))
						return &api.LintResult{}, nil
					}

					_, err := service.Lint(lintConfig)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with targeting", func() {
				It("doesn't allow you target the snippets", func() {
					lintConfig.MintFilePaths = []string{".mint/_snippet1.yml", ".mint/_snippet2.yml"}

					_, err := service.Lint(lintConfig)
					Expect(err).To(MatchError("You cannot target snippets for linting, but you targeted the following snippets: .mint/_snippet1.yml, .mint/_snippet2.yml\n\nTo lint snippets, include them from a Mint run definition and lint the run definition."))
				})
			})
		})

		Context("when specific files are not targeted", func() {
			var lintedDefinitions []api.TaskDefinition

			BeforeEach(func() {
				Expect(os.WriteFile("mint1.yml", []byte("mint1 contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.MkdirAll(".mint/some/nested/dir", 0o755)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/some/nested/dir/tasks.yml", []byte(".mint/some/nested/dir/tasks.yml contents"), 0o644)).NotTo(HaveOccurred())
				Expect(os.WriteFile(".mint/some/nested/dir/tasks.json", []byte(".mint/some/nested/dir/tasks.json contents"), 0o644)).NotTo(HaveOccurred())

				mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
					lintedDefinitions = cfg.TaskDefinitions
					return &api.LintResult{Problems: []api.LintProblem{}}, nil
				}
			})

			It("targets yaml files in the .mint dir recursively", func() {
				_, err := service.Lint(lintConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(lintedDefinitions).To(HaveLen(2))
				Expect(lintedDefinitions[0].Path).To(Equal(".mint/base.yml"))
				Expect(lintedDefinitions[1].Path).To(Equal(".mint/some/nested/dir/tasks.yml"))
			})
		})
	})
})
