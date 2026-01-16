package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"
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
