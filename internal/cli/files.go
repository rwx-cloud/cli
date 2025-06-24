package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/fs"
)

type RwxDirectoryEntry = api.RwxDirectoryEntry
type TaskDefinition = api.TaskDefinition

type MintYAMLFile struct {
	Entry RwxDirectoryEntry
	Doc   *YAMLDoc
}

// findRwxDirectoryPath returns a configured directory, if it exists, or walks up
// from the working directory to find a .rwx directory. If the found path is not
// a directory or is not readable, an error is returned.
func findAndValidateRwxDirectoryPath(configuredDirectory string) (string, error) {
	foundPath, err := findRwxDirectoryPath(configuredDirectory)
	if err != nil {
		return "", err
	}

	if foundPath != "" {
		rwxDirInfo, err := os.Stat(foundPath)
		if err != nil {
			return foundPath, fmt.Errorf("unable to read the .rwx directory at %q", foundPath)
		}

		if !rwxDirInfo.IsDir() {
			return foundPath, fmt.Errorf(".rwx directory at %q is not a directory", foundPath)
		}
	}

	return foundPath, nil
}

// findRwxDirectoryPath returns a configured directory, if it exists, or walks up
// from the working directory to find a .rwx directory.
func findRwxDirectoryPath(configuredDirectory string) (string, error) {
	if configuredDirectory != "" {
		return configuredDirectory, nil
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to determine the working directory")
	}

	// otherwise, walk up the working directory looking at each basename
	for {
		workingDirHasRwxDir, err := fs.Exists(filepath.Join(workingDirectory, ".rwx"))
		if err != nil {
			return "", errors.Wrapf(err, "unable to determine if .rwx exists in %q", workingDirectory)
		}

		if workingDirHasRwxDir {
			return filepath.Join(workingDirectory, ".rwx"), nil
		}

		workingDirHasMintDir, err := fs.Exists(filepath.Join(workingDirectory, ".mint"))
		if err != nil {
			return "", errors.Wrapf(err, "unable to determine if .mint exists in %q", workingDirectory)
		}

		if workingDirHasMintDir {
			return filepath.Join(workingDirectory, ".mint"), nil
		}

		if workingDirectory == string(os.PathSeparator) {
			return "", nil
		}

		parentDir, _ := filepath.Split(workingDirectory)
		workingDirectory = filepath.Clean(parentDir)
	}
}

// getFileOrDirectoryYAMLEntries gets a RwxDirectoryEntry for every given YAML file, or all YAML files in rwxDir when no files are provided.
func getFileOrDirectoryYAMLEntries(files []string, rwxDir string) ([]RwxDirectoryEntry, error) {
	entries, err := getFileOrDirectoryEntries(files, rwxDir)
	if err != nil {
		return nil, err
	}
	return filterYAMLFiles(entries), nil
}

// getFileOrDirectoryEntries gets a RwxDirectoryEntry for every given file, or all files in rwxDir when no files are provided.
func getFileOrDirectoryEntries(files []string, rwxDir string) ([]RwxDirectoryEntry, error) {
	if len(files) != 0 {
		return rwxDirectoryEntriesFromPaths(files)
	} else if rwxDir != "" {
		return rwxDirectoryEntries(rwxDir)
	}
	return make([]RwxDirectoryEntry, 0), nil
}

// rwxDirectoryEntriesFromPaths loads all the files in paths relative to the current working directory.
func rwxDirectoryEntriesFromPaths(paths []string) ([]RwxDirectoryEntry, error) {
	return readRwxDirectoryEntries(paths, "")
}

// rwxDirectoryEntries loads all the files in the given dir relative to the parent of dir.
func rwxDirectoryEntries(dir string) ([]RwxDirectoryEntry, error) {
	return readRwxDirectoryEntries([]string{dir}, dir)
}

func readRwxDirectoryEntries(paths []string, relativeTo string) ([]RwxDirectoryEntry, error) {
	entries := make([]RwxDirectoryEntry, 0)
	var totalSize int

	for _, path := range paths {
		err := filepath.WalkDir(path, func(subpath string, de os.DirEntry, err error) error {
			entry, entrySize, suberr := rwxDirectoryEntry(subpath, de, relativeTo)
			if suberr != nil {
				return suberr
			}

			if entry.Path == ".rwx/test-suites" && entry.IsDir() {
				return filepath.SkipDir // Skip the test-suites directory
			}

			totalSize += entrySize
			entries = append(entries, entry)
			return nil
		})
		if err != nil {
			return nil, errors.Wrapf(err, "reading rwx directory entries at %s", path)
		}
	}

	if totalSize > 5*1024*1024 {
		return nil, fmt.Errorf("the size of the these files exceed 5MiB: %s", strings.Join(paths, ", "))
	}

	return entries, nil
}

// rwxDirectoryEntry finds the file at path and converts it to a RwxDirectoryEntry.
func rwxDirectoryEntry(path string, de os.DirEntry, makePathRelativeTo string) (RwxDirectoryEntry, int, error) {
	if de == nil {
		return RwxDirectoryEntry{}, 0, os.ErrNotExist
	}

	info, err := de.Info()
	if err != nil {
		return RwxDirectoryEntry{}, 0, err
	}

	mode := info.Mode()
	permissions := mode.Perm()

	var entryType string
	switch mode.Type() {
	case os.ModeDir:
		entryType = "dir"
	case os.ModeSymlink:
		entryType = "symlink"
	case os.ModeNamedPipe:
		entryType = "named-pipe"
	case os.ModeSocket:
		entryType = "socket"
	case os.ModeDevice:
		entryType = "device"
	case os.ModeCharDevice:
		entryType = "char-device"
	case os.ModeIrregular:
		entryType = "irregular"
	default:
		if mode.IsRegular() {
			entryType = "file"
		} else {
			entryType = "unknown"
		}
	}

	var fileContents string
	var contentLength int
	if entryType == "file" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return RwxDirectoryEntry{}, contentLength, fmt.Errorf("unable to read file %q: %w", path, err)
		}

		contentLength = len(contents)
		fileContents = string(contents)
	}

	relPath := path
	if makePathRelativeTo != "" {
		rel, err := filepath.Rel(makePathRelativeTo, path)
		if err != nil {
			return RwxDirectoryEntry{}, contentLength, fmt.Errorf("unable to determine relative path of %q: %w", path, err)
		}
		relPath = filepath.ToSlash(rel) // Mint only supports unix-style path separators
	}

	return RwxDirectoryEntry{
		Type:         entryType,
		OriginalPath: path,
		Path:         relPath,
		Permissions:  uint32(permissions),
		FileContents: fileContents,
	}, contentLength, nil
}

// filterYAMLFiles finds any *.yml and *.yaml files in the given entries.
func filterYAMLFiles(entries []RwxDirectoryEntry) []RwxDirectoryEntry {
	yamlFiles := make([]RwxDirectoryEntry, 0)

	for _, entry := range entries {
		if !isYAMLFile(entry) {
			continue
		}

		yamlFiles = append(yamlFiles, entry)
	}

	return yamlFiles
}

// filterFiles finds only files in the given entries.
func filterFiles(entries []RwxDirectoryEntry) []RwxDirectoryEntry {
	files := make([]RwxDirectoryEntry, 0)

	for _, entry := range entries {
		if !entry.IsFile() {
			continue
		}

		files = append(files, entry)
	}

	return files
}

// filterYAMLFilesForModification finds any *.yml and *.yaml files in the given entries
// and reads and parses them. Entries that cannot be modified, such as JSON files
// masquerading as YAML, will not be included.
func filterYAMLFilesForModification(entries []RwxDirectoryEntry, filter func(doc *YAMLDoc) bool) []*MintYAMLFile {
	yamlFiles := make([]*MintYAMLFile, 0)

	for _, entry := range entries {
		yamlFile := validateYAMLFileForModification(entry, filter)
		if yamlFile == nil {
			continue
		}

		yamlFiles = append(yamlFiles, yamlFile)
	}

	return yamlFiles
}

// validateYAMLFileForModification reads and parses the given file entry. If it cannot
// be modified, this method will return nil.
func validateYAMLFileForModification(entry RwxDirectoryEntry, filter func(doc *YAMLDoc) bool) *MintYAMLFile {
	if !isYAMLFile(entry) {
		return nil
	}

	content, err := os.ReadFile(entry.OriginalPath)
	if err != nil {
		return nil
	}

	// JSON is valid YAML, but we don't support modifying it
	if isJSON(content) {
		return nil
	}

	doc, err := ParseYAMLDoc(string(content))
	if err != nil {
		return nil
	}

	if !filter(doc) {
		return nil
	}

	return &MintYAMLFile{
		Entry: entry,
		Doc:   doc,
	}
}

func isJSON(content []byte) bool {
	var jsonContent any
	return len(content) > 0 && content[0] == '{' && json.Unmarshal(content, &jsonContent) == nil
}

func isYAMLFile(entry RwxDirectoryEntry) bool {
	return entry.IsFile() && (strings.HasSuffix(entry.OriginalPath, ".yml") || strings.HasSuffix(entry.OriginalPath, ".yaml"))
}

func resolveWd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Return a consistent path, which can be an issue on macOS where
	// /var is symlinked to /private/var.
	return filepath.EvalSymlinks(wd)
}

func relativePathFromWd(path string) string {
	wd, err := resolveWd()
	if err != nil {
		return path
	}

	if rel, err := filepath.Rel(wd, path); err == nil {
		return rel
	}

	return path
}
