package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"
)

type InitiateRunConfig struct {
	InitParameters map[string]string
	Json           bool
	RwxDirectory   string
	MintFilePath   string
	NoCache        bool
	TargetedTasks  []string
	Title          string
	GitBranch      string
	GitSha         string
}

func (c InitiateRunConfig) Validate() error {
	if c.MintFilePath == "" {
		return errors.New("the path to a run definition must be provided using the --file flag.")
	}

	return nil
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

	sha, err := s.GitClient.GetCommit()
	if err != nil {
		return nil, errors.Wrap(err, "unable to determine git commit")
	}
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

	resolveResult, err := ResolveCliParamsForFile(relativeRunDefinitionPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to resolve CLI init params")
	}

	if resolveResult.Rewritten {
		fmt.Fprintf(s.Stderr, "Configured CLI trigger with git init params in %q\n\n", relativeRunDefinitionPath)
	}

	for _, gitParam := range resolveResult.GitParams {
		if _, exists := cfg.InitParameters[gitParam]; exists {
			patchable = false
			break
		}
	}

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

	if patchFile.Written {
		fmt.Fprintf(s.Stderr, "Included a git patch for uncommitted changes\n")
		if patchFile.UntrackedFiles.Count == 1 {
			fmt.Fprintf(s.Stderr, "The patch did not include the following untracked file. Add it with git add to use it in the run:\n")
			fmt.Fprintf(s.Stderr, "  %s\n", patchFile.UntrackedFiles.Files[0])
		} else if patchFile.UntrackedFiles.Count > 1 {
			fmt.Fprintf(s.Stderr, "The patch did not include the following untracked files. Add them with git add to use them in the run:\n")
			limit := patchFile.UntrackedFiles.Count
			if limit > 5 {
				limit = 5
			}
			for _, file := range patchFile.UntrackedFiles.Files[:limit] {
				fmt.Fprintf(s.Stderr, "  %s\n", file)
			}
			if patchFile.UntrackedFiles.Count > 5 {
				fmt.Fprintf(s.Stderr, "  and %d more\n", patchFile.UntrackedFiles.Count-5)
			}
		}
		fmt.Fprintln(s.Stderr, "")
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
