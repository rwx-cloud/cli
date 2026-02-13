package lsp

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rwx-cloud/cli/cmd/rwx/config"
	"github.com/rwx-cloud/cli/internal/errors"
)

func Serve() (int, error) {
	nodePath, err := findNode()
	if err != nil {
		return 0, err
	}

	serverJS, err := ensureBundle()
	if err != nil {
		return 0, err
	}

	return runServer(nodePath, serverJS)
}

func findNode() (string, error) {
	path, err := exec.LookPath("node")
	if err != nil {
		return "", errors.New("node is required but was not found on PATH. Install Node.js from https://nodejs.org")
	}
	return path, nil
}

func bundleHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(bundle, "bundle", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to compute bundle hash")
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func ensureBundle() (string, error) {
	hash, err := bundleHash()
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "unable to determine home directory")
	}

	cacheDir := filepath.Join(homeDir, ".config", "rwx", "lsp-server", config.Version+"-"+hash)
	markerFile := filepath.Join(cacheDir, ".extracted")
	serverJS := filepath.Join(cacheDir, "bundle", "server.js")

	currentName := filepath.Base(cacheDir)
	parentDir := filepath.Dir(cacheDir)

	if _, err := os.Stat(markerFile); err == nil {
		return serverJS, nil
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return "", errors.Wrap(err, "unable to clean cache directory")
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", errors.Wrap(err, "unable to create cache directory")
	}

	err = fs.WalkDir(bundle, "bundle", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		targetPath := filepath.Join(cacheDir, path)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, data, 0o644)
	})
	if err != nil {
		_ = os.RemoveAll(cacheDir)
		return "", errors.Wrap(err, "unable to extract language server bundle")
	}

	if err := os.WriteFile(markerFile, nil, 0o644); err != nil {
		_ = os.RemoveAll(cacheDir)
		return "", errors.Wrap(err, "unable to write extraction marker")
	}

	cleanStaleBundles(parentDir, currentName)
	return serverJS, nil
}

func cleanStaleBundles(parentDir string, currentName string) {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Name() != currentName {
			_ = os.RemoveAll(filepath.Join(parentDir, entry.Name()))
		}
	}
}

func runServer(nodePath string, serverJS string) (int, error) {
	cmd := exec.Command(nodePath, serverJS, "--stdio")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return 0, errors.Wrap(err, "unable to start language server")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			_ = cmd.Process.Signal(sig)
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, errors.Wrap(err, "language server exited unexpectedly")
	}

	return 0, nil
}
