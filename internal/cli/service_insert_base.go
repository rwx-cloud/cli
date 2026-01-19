package cli

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/goccy/go-yaml"
)

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
			AddedBases   map[string]string
			ErroredBases map[string]string `json:",omitempty"`
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
