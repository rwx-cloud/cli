package cli

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/skratchdot/open-golang/open"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
)

type DownloadArtifactConfig struct {
	TaskID                 string
	ArtifactKey            string
	OutputDir              string
	OutputFile             string
	OutputDirExplicitlySet bool
	Json                   bool
	AutoExtract            bool
	Open                   bool
}

func (c DownloadArtifactConfig) Validate() error {
	if c.TaskID == "" {
		return errors.New("task ID must be provided")
	}
	if c.ArtifactKey == "" {
		return errors.New("artifact key must be provided")
	}
	if c.OutputDir != "" && c.OutputFile != "" {
		return errors.New("output-dir and output-file cannot be used together")
	}
	return nil
}

type DownloadArtifactResult struct {
	OutputFiles []string
}

func (s Service) DownloadArtifact(cfg DownloadArtifactConfig) (*DownloadArtifactResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	artifactDownloadRequest, err := s.APIClient.GetArtifactDownloadRequest(cfg.TaskID, cfg.ArtifactKey)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return nil, errors.New(fmt.Sprintf("Artifact %s for task %s not found", cfg.ArtifactKey, cfg.TaskID))
		}
		return nil, errors.Wrap(err, "unable to fetch artifact download request")
	}

	stopSpinner := Spin(
		"Downloading artifact...",
		s.StderrIsTTY,
		s.Stderr,
	)

	artifactBytes, err := s.APIClient.DownloadArtifact(artifactDownloadRequest)
	stopSpinner()
	if err != nil {
		return nil, errors.Wrap(err, "unable to download artifact")
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
			return nil, errors.Wrapf(err, "unable to create extraction directory %s", extractDir)
		}

		extractedFiles, err := extractTar(artifactBytes, extractDir)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to extract tar archive")
		}

		// For single file artifacts, if OutputFile is specified, rename the extracted file
		if artifactDownloadRequest.Kind == "file" && cfg.OutputFile != "" && len(extractedFiles) == 1 {
			newPath := cfg.OutputFile
			if err := os.Rename(extractedFiles[0], newPath); err != nil {
				return nil, errors.Wrapf(err, "unable to rename extracted file to %s", newPath)
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
			return nil, errors.Wrapf(err, "unable to create output directory %s", outputDir)
		}

		if _, err := os.Stat(outputPath); err == nil {
			if !cfg.Json {
				fmt.Fprintf(s.Stdout, "Overwriting existing file at %s\n", outputPath)
			}
		}

		if err := os.WriteFile(outputPath, artifactBytes, 0644); err != nil {
			return nil, errors.Wrapf(err, "unable to write artifact file to %s", outputPath)
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

	result := &DownloadArtifactResult{OutputFiles: outputFiles}

	if cfg.Json {
		if err := json.NewEncoder(s.Stdout).Encode(result); err != nil {
			return nil, errors.Wrap(err, "unable to encode JSON output")
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

	return result, nil
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
