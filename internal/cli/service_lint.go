package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/messages"
)

type LintOutputFormat int

const (
	LintOutputNone LintOutputFormat = iota
	LintOutputOneLine
	LintOutputMultiLine
	LintOutputJSON
)

type LintConfig struct {
	RwxDirectory string
	OutputFormat LintOutputFormat
}

func (c LintConfig) Validate() error {
	return nil
}

func NewLintConfig(rwxDir string, formatString string) (LintConfig, error) {
	var format LintOutputFormat

	switch formatString {
	case "none":
		format = LintOutputNone
	case "oneline":
		format = LintOutputOneLine
	case "text", "multiline":
		format = LintOutputMultiLine
	case "json":
		format = LintOutputJSON
	default:
		return LintConfig{}, errors.New("unknown output format, expected one of: none, oneline, multiline, json, text")
	}

	return LintConfig{
		RwxDirectory: rwxDir,
		OutputFormat: format,
	}, nil
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
		Problems []api.LintProblem
	}{
		Problems: problems,
	}
	return json.NewEncoder(w).Encode(output)
}
