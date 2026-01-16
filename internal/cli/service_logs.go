package cli

import (
	"archive/zip"
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
