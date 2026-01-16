package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/skratchdot/open-golang/open"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/dotenv"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/rwx-cloud/cli/internal/messages"
	"github.com/rwx-cloud/cli/internal/versions"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

const DefaultArch = "x86_64"

var HandledError = errors.New("handled error")
var hasOutputVersionMessage atomic.Bool

// Service holds the main business logic of the CLI.
type Service struct {
	Config
}

func NewService(cfg Config) (Service, error) {
	if err := cfg.Validate(); err != nil {
		return Service{}, errors.Wrap(err, "validation failed")
	}

	return Service{cfg}, nil
}

// InitiateRun will connect to the Cloud API and start a new run in Mint.
func (s Service) InitiateRun(cfg InitiateRunConfig) (*api.InitiateRunResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	var rwxDirectory []RwxDirectoryEntry

	rwxDirectoryPath, err := findAndValidateRwxDirectoryPath(cfg.RwxDirectory)
	if err != nil {
		return nil, errors.Wrap(err, "unable to find .rwx directory")
	}

	runDefinitionPath, err := FindRunDefinitionFile(cfg.MintFilePath, rwxDirectoryPath)
	if err != nil {
		return nil, err
	}

	sha := s.GitClient.GetCommit()
	branch := s.GitClient.GetBranch()
	originUrl := s.GitClient.GetOriginUrl()
	patchFile := git.PatchFile{}

	// When there's no .rwx directory, create a temporary one for patches and to set run.dir
	var tempRwxDir string
	if rwxDirectoryPath == "" {
		tempRwxDir, err = os.MkdirTemp("", ".rwx-*")
		if err != nil {
			return nil, errors.Wrap(err, "unable to create temporary .rwx directory")
		}
		defer os.RemoveAll(tempRwxDir)
		rwxDirectoryPath = tempRwxDir
	}

	patchDir := filepath.Join(rwxDirectoryPath, ".patches")
	defer os.RemoveAll(patchDir)

	// Generate patches if enabled
	patchable := true
	if _, ok := os.LookupEnv("RWX_DISABLE_GIT_PATCH"); ok {
		patchable = false
	}

	// Convert to relative path for display purposes (e.g., run title)
	relativeRunDefinitionPath := relativePathFromWd(runDefinitionPath)

	if patchable {
		patchFile = s.GitClient.GeneratePatchFile(patchDir, []string{".", ":!" + relativeRunDefinitionPath})
	}

	// Load directory entries
	entries, err := rwxDirectoryEntries(rwxDirectoryPath)
	if err != nil {
		if errors.Is(err, errors.ErrFileNotExists) && tempRwxDir == "" {
			// User explicitly specified a directory that doesn't exist
			return nil, fmt.Errorf("You specified --dir %q, but %q could not be found", cfg.RwxDirectory, cfg.RwxDirectory)
		}

		return nil, errors.Wrapf(err, "unable to load directory %q", rwxDirectoryPath)
	}

	rwxDirectory = entries

	runDefinition, err := rwxDirectoryEntriesFromPaths([]string{relativeRunDefinitionPath})
	if err != nil {
		return nil, errors.Wrap(err, "unable to read provided files")
	}
	runDefinition = filterFiles(runDefinition)
	if len(runDefinition) != 1 {
		return nil, fmt.Errorf("expected exactly 1 run definition, got %d", len(runDefinition))
	}

	// reloadRunDefinitions reloads run definitions after modifying the file.
	reloadRunDefinitions := func() error {
		runDefinition, err = rwxDirectoryEntriesFromPaths([]string{relativeRunDefinitionPath})
		if err != nil {
			return errors.Wrapf(err, "unable to reload %q", relativeRunDefinitionPath)
		}
		rwxDirectoryEntries, err := rwxDirectoryEntries(rwxDirectoryPath)
		if err != nil && !errors.Is(err, errors.ErrFileNotExists) {
			return errors.Wrapf(err, "unable to reload rwx directory %q", rwxDirectoryPath)
		}

		rwxDirectory = rwxDirectoryEntries
		return nil
	}

	result, err := ResolveCliParamsForFile(runDefinition[0].OriginalPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to resolve CLI init params")
	}

	if result.Rewritten {
		fmt.Fprintf(s.Stderr, "Configured CLI trigger with git init params in %q\n\n", runDefinition[0].OriginalPath)

		if err = reloadRunDefinitions(); err != nil {
			return nil, err
		}
	}

	for _, gitParam := range result.GitParams {
		if _, exists := cfg.InitParameters[gitParam]; exists {
			patchFile = git.PatchFile{}
			break
		}
	}

	addBaseIfNeeded, err := s.insertDefaultBaseIfMissing(runDefinition)
	if err != nil {
		return nil, errors.Wrap(err, "unable to resolve base")
	}

	if len(addBaseIfNeeded.UpdatedRunFiles) > 0 {
		update := addBaseIfNeeded.UpdatedRunFiles[0]
		fmt.Fprintf(s.Stderr, "Configured %q to run on %s\n\n", update.OriginalPath, update.ResolvedBase.Image)

		if err = reloadRunDefinitions(); err != nil {
			return nil, err
		}
	}

	if len(addBaseIfNeeded.ErroredRunFiles) > 0 {
		for _, erroredFile := range addBaseIfNeeded.ErroredRunFiles {
			fmt.Fprintf(s.Stderr, "Failed to configure base for %q: %v\n", erroredFile.OriginalPath, erroredFile.Error)
		}
	}

	mintFiles := filterYAMLFilesForModification(runDefinition, func(doc *YAMLDoc) bool {
		return true
	})
	resolvedPackages, err := s.resolveOrUpdatePackagesForFiles(mintFiles, false, PickLatestMajorVersion)
	if err != nil {
		return nil, err
	}
	if len(resolvedPackages) > 0 {
		for rwxPackage, version := range resolvedPackages {
			fmt.Fprintf(s.Stderr, "Configured package %s to use version %s\n", rwxPackage, version)
		}
		fmt.Fprintln(s.Stderr, "")

		if err = reloadRunDefinitions(); err != nil {
			return nil, err
		}
	}

	i := 0
	initializationParameters := make([]api.InitializationParameter, len(cfg.InitParameters))
	for key, value := range cfg.InitParameters {
		initializationParameters[i] = api.InitializationParameter{
			Key:   key,
			Value: value,
		}
		i++
	}

	runResult, err := s.APIClient.InitiateRun(api.InitiateRunConfig{
		InitializationParameters: initializationParameters,
		TaskDefinitions:          runDefinition,
		RwxDirectory:             rwxDirectory,
		TargetedTaskKeys:         cfg.TargetedTasks,
		Title:                    cfg.Title,
		UseCache:                 !cfg.NoCache,
		Git: api.GitMetadata{
			Branch:    branch,
			Sha:       sha,
			OriginUrl: originUrl,
		},
		Patch: api.PatchMetadata{
			Sent:           patchFile.Written,
			UntrackedFiles: patchFile.UntrackedFiles.Files,
			UntrackedCount: patchFile.UntrackedFiles.Count,
			LFSFiles:       patchFile.LFSChangedFiles.Files,
			LFSCount:       patchFile.LFSChangedFiles.Count,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initiate run")
	}

	return runResult, nil
}

func (s Service) InitiateDispatch(cfg InitiateDispatchConfig) (*api.InitiateDispatchResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	dispatchResult, err := s.APIClient.InitiateDispatch(api.InitiateDispatchConfig{
		DispatchKey: cfg.DispatchKey,
		Params:      cfg.Params,
		Ref:         cfg.Ref,
		Title:       cfg.Title,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initiate dispatch")
	}

	return dispatchResult, nil
}

func (s Service) GetDispatch(cfg GetDispatchConfig) ([]GetDispatchRun, error) {
	defer s.outputLatestVersionMessage()
	dispatchResult, err := s.APIClient.GetDispatch(api.GetDispatchConfig{
		DispatchId: cfg.DispatchId,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get dispatch")
	}

	if dispatchResult.Status == "not_ready" {
		return nil, errors.ErrRetry
	}

	if dispatchResult.Status == "error" {
		if dispatchResult.Error == "" {
			return nil, errors.New("Failed to get dispatch")
		}
		return nil, errors.New(dispatchResult.Error)
	}

	if len(dispatchResult.Runs) == 0 {
		return nil, errors.New("No runs were created as a result of this dispatch")
	}

	runs := make([]GetDispatchRun, len(dispatchResult.Runs))
	for i, run := range dispatchResult.Runs {
		runs[i] = GetDispatchRun{RunId: run.RunId, RunUrl: run.RunUrl}
	}

	return runs, nil
}

func (s Service) Lint(cfg LintConfig) (*api.LintResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	rwxDirectoryPath, err := findAndValidateRwxDirectoryPath(cfg.RwxDirectory)
	if err != nil {
		return nil, errors.Wrap(err, "unable to find .rwx directory")
	}

	rwxDirEntries, err := rwxDirectoryEntries(rwxDirectoryPath)
	if err != nil {
		return nil, err
	}
	rwxDirEntries = filterYAMLFiles(rwxDirEntries)
	rwxDirEntries = removeDuplicates(rwxDirEntries, func(entry RwxDirectoryEntry) string {
		return entry.Path
	})

	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current working directory")
	}

	relativeRwxDirectoryPath, err := filepath.Rel(wd, rwxDirectoryPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get relative path for .rwx directory")
	}

	taskDefinitions := Map(rwxDirEntries, func(entry RwxDirectoryEntry) TaskDefinition {
		return TaskDefinition{
			Path:         filepath.Join(relativeRwxDirectoryPath, entry.Path),
			FileContents: entry.FileContents,
		}
	})

	targetedPaths := Map(rwxDirEntries, func(entry RwxDirectoryEntry) string {
		return filepath.Join(relativeRwxDirectoryPath, entry.Path)
	})
	nonSnippetFileNames, _ := findSnippets(targetedPaths)
	targetedPaths = nonSnippetFileNames

	lintResult, err := s.APIClient.Lint(api.LintConfig{
		TaskDefinitions: taskDefinitions,
		TargetPaths:     targetedPaths,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to lint files")
	}

	switch cfg.OutputFormat {
	case LintOutputOneLine:
		err = outputLintOneLine(s.Stdout, lintResult.Problems)
	case LintOutputMultiLine:
		err = outputLintMultiLine(s.Stdout, lintResult.Problems, len(targetedPaths))
	case LintOutputJSON:
		err = outputLintJSON(s.Stdout, lintResult.Problems)
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to output lint results")
	}

	return lintResult, nil
}

func outputLintMultiLine(w io.Writer, problems []api.LintProblem, fileCount int) error {
	for _, lf := range problems {
		fmt.Fprintln(w)

		if len(lf.StackTrace) > 0 {
			fmt.Fprint(w, "[", lf.Severity, "] ")
			fmt.Fprintln(w, messages.FormatUserMessage(lf.Message, lf.Frame, lf.StackTrace, lf.Advice))
		} else {
			if fileLoc := lf.FileLocation(); len(fileLoc) > 0 {
				fmt.Fprint(w, fileLoc, "  ")
			}
			fmt.Fprint(w, "[", lf.Severity, "]")
			fmt.Fprintln(w)

			fmt.Fprint(w, lf.Message)

			if len(lf.Advice) > 0 {
				fmt.Fprint(w, "\n", lf.Advice)
			}

			fmt.Fprintln(w)
		}
	}

	pluralizedProblems := "problems"
	if len(problems) == 1 {
		pluralizedProblems = "problem"
	}

	pluralizedFiles := "files"
	if fileCount == 1 {
		pluralizedFiles = "file"
	}

	fmt.Fprintf(w, "\nChecked %d %s and found %d %s.\n", fileCount, pluralizedFiles, len(problems), pluralizedProblems)

	return nil
}

func outputLintOneLine(w io.Writer, lintedFiles []api.LintProblem) error {
	if len(lintedFiles) == 0 {
		return nil
	}

	for _, lf := range lintedFiles {
		fmt.Fprintf(w, "%-8s", lf.Severity)

		if fileLoc := lf.FileLocation(); len(fileLoc) > 0 {
			fmt.Fprint(w, fileLoc, " - ")
		}

		fmt.Fprint(w, strings.TrimSuffix(strings.ReplaceAll(lf.Message, "\n", " "), " "))
		fmt.Fprintln(w)
	}

	return nil
}

func outputLintJSON(w io.Writer, problems []api.LintProblem) error {
	output := struct {
		Problems []api.LintProblem `json:"problems"`
	}{
		Problems: problems,
	}
	return json.NewEncoder(w).Encode(output)
}

func (s Service) Login(cfg LoginConfig) error {
	err := cfg.Validate()
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	authCodeResult, err := s.APIClient.ObtainAuthCode(api.ObtainAuthCodeConfig{
		Code: api.ObtainAuthCodeCode{
			DeviceName: cfg.DeviceName,
		},
	})
	if err != nil {
		return errors.Wrap(err, "unable to obtain an auth code")
	}

	// we print a nice message to handle the case where opening the browser fails, so we ignore this error
	cfg.OpenUrl(authCodeResult.AuthorizationUrl) //nolint:errcheck

	fmt.Fprintln(s.Stdout)
	fmt.Fprintln(s.Stdout, "To authorize this device, you'll need to login to RWX Cloud and choose an organization.")
	fmt.Fprintln(s.Stdout, "Your browser should automatically open. If it does not, you can visit this URL:")
	fmt.Fprintln(s.Stdout)
	fmt.Fprintf(s.Stdout, "\t%v\n", authCodeResult.AuthorizationUrl)
	fmt.Fprintln(s.Stdout)
	fmt.Fprintln(s.Stdout, "Once authorized, a personal access token will be generated and stored securely on this device.")
	fmt.Fprintln(s.Stdout)

	stopSpinner := Spin("Waiting for authorization...", s.StdoutIsTTY, s.Stdout)

	stop := func() {
		stopSpinner()
		s.outputLatestVersionMessage()
	}

	for {
		tokenResult, err := s.APIClient.AcquireToken(authCodeResult.TokenUrl)
		if err != nil {
			stop()
			return errors.Wrap(err, "unable to acquire the token")
		}

		switch tokenResult.State {
		case "consumed":
			stop()
			return errors.New("The code has already been used. Try again.")
		case "expired":
			stop()
			return errors.New("The code has expired. Try again.")
		case "authorized":
			stop()
			if tokenResult.Token == "" {
				return errors.New("The code has been authorized, but there is no token. You can try again, but this is likely an issue with RWX Cloud. Please reach out at support@rwx.com.")
			} else {
				if err := accesstoken.Set(cfg.AccessTokenBackend, tokenResult.Token); err == nil {
					fmt.Fprint(s.Stdout, "Authorized!\n")
					return nil
				} else {
					return fmt.Errorf("An error occurred while storing the token: %w", err)
				}
			}
		case "pending":
			time.Sleep(cfg.PollInterval)
		default:
			stop()
			return errors.New("The code is in an unexpected state. You can try again, but this is likely an issue with RWX Cloud. Please reach out at support@rwx.com.")
		}
	}
}

func (s Service) DownloadLogs(cfg DownloadLogsConfig) error {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	LogDownloadRequest, err := s.APIClient.GetLogDownloadRequest(cfg.TaskID)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return errors.New(fmt.Sprintf("Task %s not found", cfg.TaskID))
		}
		return errors.Wrap(err, "unable to fetch log archive request")
	}

	stopSpinner := Spin(
		"Downloading logs...",
		s.StderrIsTTY,
		s.Stderr,
	)

	logBytes, err := s.APIClient.DownloadLogs(LogDownloadRequest)
	stopSpinner()
	if err != nil {
		return errors.Wrap(err, "unable to download logs")
	}

	var outputPath string
	if cfg.OutputFile != "" {
		outputPath = cfg.OutputFile
	} else {
		outputPath = filepath.Join(cfg.OutputDir, LogDownloadRequest.Filename)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return errors.Wrapf(err, "unable to create output directory %s", outputDir)
	}

	if _, err := os.Stat(outputPath); err == nil {
		if !cfg.Json {
			fmt.Fprintf(s.Stdout, "Overwriting existing file at %s\n", outputPath)
		}
	}

	if err := os.WriteFile(outputPath, logBytes, 0644); err != nil {
		return errors.Wrapf(err, "unable to write log file to %s", outputPath)
	}

	var outputFiles []string
	outputFiles = append(outputFiles, outputPath)

	if cfg.AutoExtract && strings.HasSuffix(strings.ToLower(outputPath), ".zip") {
		// Create a directory named after the zip file (without .zip extension)
		zipName := filepath.Base(outputPath)
		extractDirName := strings.TrimSuffix(zipName, filepath.Ext(zipName))
		extractDir := filepath.Join(filepath.Dir(outputPath), extractDirName)

		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return errors.Wrapf(err, "unable to create extraction directory %s", extractDir)
		}

		extractedFiles, err := extractZip(outputPath, extractDir)
		if err != nil {
			return errors.Wrapf(err, "unable to extract zip archive %s", outputPath)
		}
		outputFiles = extractedFiles

		if !cfg.Json {
			fmt.Fprintf(s.Stdout, "Extracted %d file(s) from %s to %s\n", len(extractedFiles), outputPath, extractDir)
		}
	}

	if cfg.Open {
		for _, file := range outputFiles {
			if err := open.Run(file); err != nil {
				if !cfg.Json {
					fmt.Fprintf(s.Stderr, "Failed to open %s: %v\n", file, err)
				}
			}
		}
	}

	if cfg.Json {
		output := struct {
			OutputFiles []string `json:"outputFiles"`
		}{
			OutputFiles: outputFiles,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		if len(outputFiles) == 1 {
			fmt.Fprintf(s.Stdout, "Logs downloaded to %s\n", outputFiles[0])
		} else {
			fmt.Fprintf(s.Stdout, "Logs downloaded and extracted:\n")
			for _, file := range outputFiles {
				fmt.Fprintf(s.Stdout, "  %s\n", file)
			}
		}
	}
	return nil
}

func extractZip(zipPath, destDir string) ([]string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to open zip file")
	}
	defer reader.Close()

	var extractedFiles []string

	for _, file := range reader.File {
		filePath := filepath.Join(destDir, file.Name)
		if !strings.HasPrefix(filePath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return nil, errors.Wrapf(err, "unable to create directory %s", filePath)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, errors.Wrapf(err, "unable to create directory for %s", filePath)
		}

		rc, err := file.Open()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to open file %s in zip", file.Name)
		}

		outFile, err := os.Create(filePath)
		if err != nil {
			rc.Close()
			return nil, errors.Wrapf(err, "unable to create file %s", filePath)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return nil, errors.Wrapf(err, "unable to extract file %s", filePath)
		}

		if err := os.Chmod(filePath, file.FileInfo().Mode()); err != nil {
			return nil, errors.Wrapf(err, "unable to set permissions for %s", filePath)
		}

		extractedFiles = append(extractedFiles, filePath)
	}

	return extractedFiles, nil
}

func (s Service) DownloadArtifact(cfg DownloadArtifactConfig) error {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	artifactDownloadRequest, err := s.APIClient.GetArtifactDownloadRequest(cfg.TaskID, cfg.ArtifactKey)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return errors.New(fmt.Sprintf("Artifact %s for task %s not found", cfg.ArtifactKey, cfg.TaskID))
		}
		return errors.Wrap(err, "unable to fetch artifact download request")
	}

	stopSpinner := Spin(
		"Downloading artifact...",
		s.StderrIsTTY,
		s.Stderr,
	)

	artifactBytes, err := s.APIClient.DownloadArtifact(artifactDownloadRequest)
	stopSpinner()
	if err != nil {
		return errors.Wrap(err, "unable to download artifact")
	}

	// For files, always extract the single file from the tar
	// For directories, extract if AutoExtract is true
	shouldExtract := artifactDownloadRequest.Kind == "file" || (artifactDownloadRequest.Kind == "directory" && cfg.AutoExtract)

	var outputFiles []string

	if shouldExtract {
		// Extract tar to output directory
		var extractDir string
		if cfg.OutputFile != "" {
			// If output file is specified, use its directory as extraction dir
			extractDir = filepath.Dir(cfg.OutputFile)
		} else if cfg.OutputDirExplicitlySet {
			// If output-dir was explicitly set by user, extract directly into it
			extractDir = cfg.OutputDir
		} else {
			// For default Downloads folder, create a subdirectory named after the tar file
			// Strip .tar extension from filename and sanitize to prevent path traversal
			dirName := strings.TrimSuffix(artifactDownloadRequest.Filename, ".tar")
			dirName = filepath.Base(dirName) // Remove any path components for security
			extractDir = filepath.Join(cfg.OutputDir, dirName)
		}

		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return errors.Wrapf(err, "unable to create extraction directory %s", extractDir)
		}

		extractedFiles, err := extractTar(artifactBytes, extractDir)
		if err != nil {
			return errors.Wrapf(err, "unable to extract tar archive")
		}

		// For single file artifacts, if OutputFile is specified, rename the extracted file
		if artifactDownloadRequest.Kind == "file" && cfg.OutputFile != "" && len(extractedFiles) == 1 {
			newPath := cfg.OutputFile
			if err := os.Rename(extractedFiles[0], newPath); err != nil {
				return errors.Wrapf(err, "unable to rename extracted file to %s", newPath)
			}
			outputFiles = []string{newPath}
		} else {
			outputFiles = extractedFiles
		}

		if !cfg.Json && artifactDownloadRequest.Kind == "directory" {
			fmt.Fprintf(s.Stdout, "Extracted %d file(s) to %s\n", len(outputFiles), extractDir)
		}
	} else {
		// Save the raw tar file
		var outputPath string
		if cfg.OutputFile != "" {
			outputPath = cfg.OutputFile
		} else {
			outputPath = filepath.Join(cfg.OutputDir, artifactDownloadRequest.Filename)
		}

		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return errors.Wrapf(err, "unable to create output directory %s", outputDir)
		}

		if _, err := os.Stat(outputPath); err == nil {
			if !cfg.Json {
				fmt.Fprintf(s.Stdout, "Overwriting existing file at %s\n", outputPath)
			}
		}

		if err := os.WriteFile(outputPath, artifactBytes, 0644); err != nil {
			return errors.Wrapf(err, "unable to write artifact file to %s", outputPath)
		}

		outputFiles = []string{outputPath}
	}

	if cfg.Open {
		for _, file := range outputFiles {
			if err := open.Run(file); err != nil {
				if !cfg.Json {
					fmt.Fprintf(s.Stderr, "Failed to open %s: %v\n", file, err)
				}
			}
		}
	}

	if cfg.Json {
		output := struct {
			OutputFiles []string `json:"outputFiles"`
		}{
			OutputFiles: outputFiles,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		if len(outputFiles) == 1 {
			fmt.Fprintf(s.Stdout, "Artifact downloaded to %s\n", outputFiles[0])
		} else {
			fmt.Fprintf(s.Stdout, "Artifact downloaded and extracted:\n")
			for _, file := range outputFiles {
				fmt.Fprintf(s.Stdout, "  %s\n", file)
			}
		}
	}

	return nil
}

func extractTar(data []byte, destDir string) ([]string, error) {
	tarReader := tar.NewReader(bytes.NewReader(data))

	var extractedFiles []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "unable to read tar header")
		}

		filePath := filepath.Join(destDir, header.Name)
		cleanedDestDir := filepath.Clean(destDir)
		cleanedFilePath := filepath.Clean(filePath)
		// Allow the destDir itself or anything under it, but block path traversal
		if cleanedFilePath != cleanedDestDir && !strings.HasPrefix(cleanedFilePath, cleanedDestDir+string(os.PathSeparator)) {
			return nil, fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return nil, errors.Wrapf(err, "unable to create directory %s", filePath)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return nil, errors.Wrapf(err, "unable to create directory for %s", filePath)
			}

			outFile, err := os.Create(filePath)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create file %s", filePath)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return nil, errors.Wrapf(err, "unable to extract file %s", filePath)
			}
			outFile.Close()

			if err := os.Chmod(filePath, os.FileMode(header.Mode)); err != nil {
				return nil, errors.Wrapf(err, "unable to set permissions for %s", filePath)
			}

			extractedFiles = append(extractedFiles, filePath)
		}
	}

	return extractedFiles, nil
}

func (s Service) GetRunStatus(cfg GetRunStatusConfig) (*GetRunStatusResult, error) {
	defer s.outputLatestVersionMessage()

	var stopSpinner func()
	if cfg.Wait && !cfg.Json {
		stopSpinner = Spin("Waiting for run to complete...", s.StdoutIsTTY, s.Stdout)
	}

	for {
		statusResult, err := s.APIClient.RunStatus(api.RunStatusConfig{RunID: cfg.RunID})
		if err != nil {
			if stopSpinner != nil {
				stopSpinner()
			}
			return nil, errors.Wrap(err, "unable to get run status")
		}

		status := ""
		if statusResult.Status != nil {
			status = statusResult.Status.Result
		}

		if !cfg.Wait || statusResult.Polling.Completed {
			if stopSpinner != nil {
				stopSpinner()
			}
			return &GetRunStatusResult{
				RunID:        cfg.RunID,
				ResultStatus: status,
				Completed:    statusResult.Polling.Completed,
			}, nil
		}

		if statusResult.Polling.BackoffMs == nil {
			if stopSpinner != nil {
				stopSpinner()
			}
			return nil, errors.New("unable to wait for run")
		}
		time.Sleep(time.Duration(*statusResult.Polling.BackoffMs) * time.Millisecond)
	}
}

func (s Service) GetRunPrompt(runID string) (string, error) {
	return s.APIClient.GetRunPrompt(runID)
}

func (s Service) SetSecretsInVault(cfg SetSecretsInVaultConfig) error {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	secrets := []api.Secret{}
	for i := range cfg.Secrets {
		key, value, found := strings.Cut(cfg.Secrets[i], "=")
		if !found {
			return errors.New(fmt.Sprintf("Invalid secret '%s'. Secrets must be specified in the form 'KEY=value'.", cfg.Secrets[i]))
		}
		secrets = append(secrets, api.Secret{
			Name:   key,
			Secret: value,
		})
	}

	if cfg.File != "" {
		fd, err := os.Open(cfg.File)
		if err != nil {
			return errors.Wrapf(err, "error while opening %q", cfg.File)
		}
		defer fd.Close()

		fileContent, err := io.ReadAll(fd)
		if err != nil {
			return errors.Wrapf(err, "error while reading %q", cfg.File)
		}

		dotenvMap := make(map[string]string)
		err = dotenv.ParseBytes(fileContent, dotenvMap)
		if err != nil {
			return errors.Wrapf(err, "error while parsing %q", cfg.File)
		}

		for key, value := range dotenvMap {
			secrets = append(secrets, api.Secret{
				Name:   key,
				Secret: value,
			})
		}
	}

	result, err := s.APIClient.SetSecretsInVault(api.SetSecretsInVaultConfig{
		VaultName: cfg.Vault,
		Secrets:   secrets,
	})

	if err != nil {
		return errors.Wrap(err, "unable to set secrets")
	}

	if cfg.Json {
		output := struct {
			Vault      string   `json:"vault"`
			SetSecrets []string `json:"set_secrets"`
		}{
			Vault:      cfg.Vault,
			SetSecrets: result.SetSecrets,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return errors.Wrap(err, "unable to encode JSON output")
		}
	} else if result != nil && len(result.SetSecrets) > 0 {
		fmt.Fprintln(s.Stdout)
		fmt.Fprintf(s.Stdout, "Successfully set the following secrets: %s", strings.Join(result.SetSecrets, ", "))
	}

	return nil
}

func (s Service) ResolvePackages(cfg ResolvePackagesConfig) (ResolvePackagesResult, error) {
	err := cfg.Validate()
	if err != nil {
		return ResolvePackagesResult{}, errors.Wrap(err, "validation failed")
	}

	rwxDirectoryPath, err := findAndValidateRwxDirectoryPath(cfg.RwxDirectory)
	if err != nil {
		return ResolvePackagesResult{}, errors.Wrap(err, "unable to find .rwx directory")
	}

	yamlFiles, err := getFileOrDirectoryYAMLEntries(cfg.Files, rwxDirectoryPath)
	if err != nil {
		return ResolvePackagesResult{}, err
	}

	if len(yamlFiles) == 0 {
		return ResolvePackagesResult{}, fmt.Errorf("no files provided, and no yaml files found in directory %s", rwxDirectoryPath)
	}

	mintFiles := filterYAMLFilesForModification(yamlFiles, func(doc *YAMLDoc) bool {
		return true
	})

	replacements, err := s.resolveOrUpdatePackagesForFiles(mintFiles, false, cfg.LatestVersionPicker)
	if err != nil {
		return ResolvePackagesResult{}, err
	}

	if cfg.Json {
		output := struct {
			ResolvedPackages map[string]string `json:"resolved_packages"`
		}{
			ResolvedPackages: replacements,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return ResolvePackagesResult{}, errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		if len(replacements) == 0 {
			fmt.Fprintln(s.Stdout, "No packages to resolve.")
		} else {
			fmt.Fprintln(s.Stdout, "Resolved the following packages:")
			for rwxPackage, version := range replacements {
				fmt.Fprintf(s.Stdout, "\t%s → %s\n", rwxPackage, version)
			}
		}
	}

	return ResolvePackagesResult{ResolvedPackages: replacements}, nil
}

func (s Service) UpdatePackages(cfg UpdatePackagesConfig) error {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	rwxDirectoryPath, err := findAndValidateRwxDirectoryPath(cfg.RwxDirectory)
	if err != nil {
		return errors.Wrap(err, "unable to find .rwx directory")
	}

	yamlFiles, err := getFileOrDirectoryYAMLEntries(cfg.Files, rwxDirectoryPath)
	if err != nil {
		return err
	}

	if len(yamlFiles) == 0 {
		return errors.New(fmt.Sprintf("no files provided, and no yaml files found in directory %s", rwxDirectoryPath))
	}

	mintFiles := filterYAMLFilesForModification(yamlFiles, func(doc *YAMLDoc) bool {
		return true
	})

	replacements, err := s.resolveOrUpdatePackagesForFiles(mintFiles, true, cfg.ReplacementVersionPicker)
	if err != nil {
		return err
	}

	if cfg.Json {
		output := struct {
			UpdatedPackages map[string]string `json:"updated_packages"`
		}{
			UpdatedPackages: replacements,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		if len(replacements) == 0 {
			fmt.Fprintln(s.Stdout, "All packages are up-to-date.")
		} else {
			fmt.Fprintln(s.Stdout, "Updated the following packages:")
			for original, replacement := range replacements {
				fmt.Fprintf(s.Stdout, "\t%s → %s\n", original, replacement)
			}
		}
	}

	return nil
}

var rePackageVersion = regexp.MustCompile(`([a-z0-9-]+\/[a-z0-9-]+)(?:\s+(([0-9]+)\.[0-9]+\.[0-9]+))?`)

type PackageVersion struct {
	Original     string
	Name         string
	Version      string
	MajorVersion string
}

func (s Service) parsePackageVersion(str string) PackageVersion {
	match := rePackageVersion.FindStringSubmatch(str)
	if len(match) == 0 {
		return PackageVersion{}
	}

	return PackageVersion{
		Original:     match[0],
		Name:         tryGetSliceAtIndex(match, 1, ""),
		Version:      tryGetSliceAtIndex(match, 2, ""),
		MajorVersion: tryGetSliceAtIndex(match, 3, ""),
	}
}

func (s Service) resolveOrUpdatePackagesForFiles(mintFiles []*MintYAMLFile, update bool, versionPicker func(versions api.PackageVersionsResult, rwxPackage string, major string) (string, error)) (map[string]string, error) {
	packageVersions, err := s.APIClient.GetPackageVersions()
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch package versions")
	}

	docs := make(map[string]*YAMLDoc)
	replacements := make(map[string]string)

	for _, file := range mintFiles {
		hasChange := false

		var nodePath string
		if file.Doc.IsRunDefinition() {
			nodePath = "$.tasks[*].call"
		} else if file.Doc.IsListOfTasks() {
			nodePath = "$[*].call"
		} else {
			continue
		}

		err = file.Doc.ForEachNode(nodePath, func(node ast.Node) error {
			packageVersion := s.parsePackageVersion(node.String())
			if packageVersion.Name == "" {
				// Packages won't be found for eg. embedded runs, call: ${{ run.dir }}/embed.yml
				return nil
			} else if !update && packageVersion.MajorVersion != "" {
				return nil
			}

			newName := packageVersions.Renames[packageVersion.Name]
			if newName == "" {
				newName = packageVersion.Name
			}

			targetPackageVersion, err := versionPicker(*packageVersions, newName, packageVersion.MajorVersion)
			if err != nil {
				fmt.Fprintln(s.Stderr, err.Error())
				return nil
			}

			newPackage := fmt.Sprintf("%s %s", newName, targetPackageVersion)
			if newPackage == node.String() {
				return nil
			}

			if err = file.Doc.ReplaceAtPath(node.GetPath(), newPackage); err != nil {
				return err
			}

			if newName != packageVersion.Name {
				replacements[packageVersion.Original] = fmt.Sprintf("%s %s", newName, targetPackageVersion)
			} else {
				replacements[packageVersion.Original] = targetPackageVersion
			}
			hasChange = true
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "unable to replace package references")
		}

		if hasChange {
			docs[file.Entry.OriginalPath] = file.Doc
		}
	}

	for path, doc := range docs {
		if !doc.HasChanges() {
			continue
		}

		err := doc.WriteFile(path)
		if err != nil {
			return replacements, err
		}
	}

	return replacements, nil
}

func (s Service) InsertBase(cfg InsertBaseConfig) (InsertDefaultBaseResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return InsertDefaultBaseResult{}, errors.Wrap(err, "validation failed")
	}

	rwxDirectoryPath, err := findAndValidateRwxDirectoryPath(cfg.RwxDirectory)
	if err != nil {
		return InsertDefaultBaseResult{}, errors.Wrap(err, "unable to find .rwx directory")
	}

	yamlFiles, err := getFileOrDirectoryYAMLEntries(cfg.Files, rwxDirectoryPath)
	if err != nil {
		return InsertDefaultBaseResult{}, err
	}

	if len(yamlFiles) == 0 {
		return InsertDefaultBaseResult{}, fmt.Errorf("no files provided, and no yaml files found in directory %s", rwxDirectoryPath)
	}

	result, err := s.insertDefaultBaseIfMissing(yamlFiles)
	if err != nil {
		return InsertDefaultBaseResult{}, err
	}

	if cfg.Json {
		addedBases := make(map[string]string)
		for _, runFile := range result.UpdatedRunFiles {
			addedBases[relativePathFromWd(runFile.OriginalPath)] = runFile.ResolvedBase.Image
		}
		erroredBases := make(map[string]string)
		for _, runFile := range result.ErroredRunFiles {
			erroredBases[relativePathFromWd(runFile.OriginalPath)] = runFile.Error.Error()
		}
		output := struct {
			AddedBases   map[string]string `json:"added_bases"`
			ErroredBases map[string]string `json:"errored_bases,omitempty"`
		}{
			AddedBases:   addedBases,
			ErroredBases: erroredBases,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return InsertDefaultBaseResult{}, errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		if len(yamlFiles) == 0 {
			fmt.Fprintf(s.Stdout, "No run files found in %q.\n", cfg.RwxDirectory)
		} else if !result.HasChanges() {
			fmt.Fprintln(s.Stdout, "No run files were missing base.")
		} else {
			if len(result.UpdatedRunFiles) > 0 {
				fmt.Fprintln(s.Stdout, "Added base to the following run definitions:")
				for _, runFile := range result.UpdatedRunFiles {
					fmt.Fprintf(s.Stdout, "\t%s → %s\n", relativePathFromWd(runFile.OriginalPath), runFile.ResolvedBase.Image)
				}
				if len(result.ErroredRunFiles) > 0 {
					fmt.Fprintln(s.Stdout)
				}
			}

			if len(result.ErroredRunFiles) > 0 {
				fmt.Fprintln(s.Stdout, "Failed to add base to the following run definitions:")
				for _, runFile := range result.ErroredRunFiles {
					fmt.Fprintf(s.Stdout, "\t%s → %s\n", relativePathFromWd(runFile.OriginalPath), runFile.Error)
				}
			}
		}
	}

	return result, nil
}

func (s Service) insertDefaultBaseIfMissing(mintFiles []RwxDirectoryEntry) (InsertDefaultBaseResult, error) {
	runFiles, err := s.getFilesForBaseInsert(mintFiles)
	if err != nil {
		return InsertDefaultBaseResult{}, err
	}

	if len(runFiles) == 0 {
		return InsertDefaultBaseResult{}, nil
	}

	defaultBaseSpec, err := s.getDefaultBaseSpec()
	if err != nil {
		return InsertDefaultBaseResult{}, errors.Wrap(err, "unable to get default base spec")
	}

	erroredRunFiles := make([]BaseLayerRunFile, 0, len(runFiles))
	updatedRunFiles := make([]BaseLayerRunFile, 0, len(runFiles))
	for _, runFile := range runFiles {
		runFile.ResolvedBase = defaultBaseSpec

		err := s.writeRunFileWithBase(runFile)
		if err != nil {
			runFile.Error = err
			erroredRunFiles = append(erroredRunFiles, runFile)
		} else {
			updatedRunFiles = append(updatedRunFiles, runFile)
		}
	}

	return InsertDefaultBaseResult{
		ErroredRunFiles: erroredRunFiles,
		UpdatedRunFiles: updatedRunFiles,
	}, nil
}

func (s Service) getFilesForBaseInsert(entries []RwxDirectoryEntry) ([]BaseLayerRunFile, error) {
	yamlFiles := filterYAMLFilesForModification(entries, func(doc *YAMLDoc) bool {
		if !doc.HasTasks() {
			return false
		}

		// Skip files that already define a 'base'
		if doc.HasBase() {
			return false
		}

		// Skip if all tasks in this file are embedded runs
		if doc.AllTasksAreEmbeddedRuns() {
			return false
		}

		return true
	})

	runFiles := make([]BaseLayerRunFile, 0)
	for _, yamlFile := range yamlFiles {
		runFiles = append(runFiles, BaseLayerRunFile{OriginalPath: yamlFile.Entry.OriginalPath})
	}

	return runFiles, nil
}

func (s Service) getDefaultBaseSpec() (BaseSpec, error) {
	result, err := s.APIClient.GetDefaultBase()

	if err != nil {
		return BaseSpec{}, errors.Wrap(err, "unable to get default base")
	}

	return BaseSpec{
		Image:  result.Image,
		Config: result.Config,
		Arch:   result.Arch,
	}, nil
}

func (s Service) writeRunFileWithBase(runFile BaseLayerRunFile) error {
	doc, err := ParseYAMLFile(runFile.OriginalPath)
	if err != nil {
		return err
	}

	resolvedBase := runFile.ResolvedBase
	base := yaml.MapSlice{
		{Key: "image", Value: resolvedBase.Image},
		{Key: "config", Value: resolvedBase.Config},
	}

	if resolvedBase.Arch != "" && resolvedBase.Arch != DefaultArch {
		base = append(base, yaml.MapItem{Key: "arch", Value: resolvedBase.Arch})
	}

	err = doc.InsertBefore("$.tasks", map[string]any{
		"base": base,
	})
	if err != nil {
		return err
	}

	return doc.WriteFile(runFile.OriginalPath)
}

func (s Service) outputLatestVersionMessage() {
	if !versions.NewVersionAvailable() {
		return
	}

	if !hasOutputVersionMessage.CompareAndSwap(false, true) {
		return
	}

	showLatestVersion := os.Getenv("MINT_HIDE_LATEST_VERSION") == "" && os.Getenv("RWX_HIDE_LATEST_VERSION") == ""

	if !showLatestVersion {
		return
	}

	w := s.Stderr
	fmt.Fprintf(w, "A new release of rwx is available: %s → %s\n", versions.GetCliCurrentVersion(), versions.GetCliLatestVersion())

	if versions.InstalledWithHomebrew() {
		fmt.Fprintln(w, "To upgrade, run: brew upgrade rwx-cloud/tap/rwx")
	}

	fmt.Fprintln(w)
}

func PickLatestMajorVersion(versions api.PackageVersionsResult, rwxPackage string, _ string) (string, error) {
	latestVersion, ok := versions.LatestMajor[rwxPackage]
	if !ok {
		return "", fmt.Errorf("Unable to find the package %q; skipping it.", rwxPackage)
	}

	return latestVersion, nil
}

func PickLatestMinorVersion(versions api.PackageVersionsResult, rwxPackage string, major string) (string, error) {
	if major == "" {
		return PickLatestMajorVersion(versions, rwxPackage, major)
	}

	majorVersions, ok := versions.LatestMinor[rwxPackage]
	if !ok {
		return "", fmt.Errorf("Unable to find the package %q; skipping it.", rwxPackage)
	}

	latestVersion, ok := majorVersions[major]
	if !ok {
		return "", fmt.Errorf("Unable to find major version %q for package %q; skipping it.", major, rwxPackage)
	}

	return latestVersion, nil
}

func findSnippets(fileNames []string) (nonSnippetFileNames []string, snippetFileNames []string) {
	for _, fileName := range fileNames {
		if strings.HasPrefix(path.Base(fileName), "_") {
			snippetFileNames = append(snippetFileNames, fileName)
		} else {
			nonSnippetFileNames = append(nonSnippetFileNames, fileName)
		}
	}
	return nonSnippetFileNames, snippetFileNames
}

func removeDuplicates[T any, K comparable](list []T, identity func(t T) K) []T {
	seen := make(map[K]bool)
	var ts []T

	for _, t := range list {
		id := identity(t)
		if _, found := seen[id]; !found {
			seen[id] = true
			ts = append(ts, t)
		}
	}
	return ts
}

func Map[T any, R any](input []T, transformer func(T) R) []R {
	result := make([]R, len(input))
	for i, item := range input {
		result[i] = transformer(item)
	}
	return result
}

func tryGetSliceAtIndex[S ~[]E, E any](s S, index int, defaultValue E) E {
	if len(s) <= index {
		return defaultValue
	}
	return s[index]
}
