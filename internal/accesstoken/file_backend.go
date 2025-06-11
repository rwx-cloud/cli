package accesstoken

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/rwx-research/mint-cli/internal/errors"
)

type FileBackend struct {
	PrimaryDirectory    string
	FallbackDirectories []string
}

func NewFileBackend(dirs []string) (*FileBackend, error) {
	if len(dirs) < 1 {
		return nil, fmt.Errorf("at least one directory must be provided")
	}

	primaryDirectory, err := expandTilde(dirs[0])
	if err != nil {
		return nil, err
	}

	fallbackDirectories := make([]string, len(dirs)-1)
	for i, dir := range dirs[1:] {
		fallbackDir, err := expandTilde(dir)
		if err != nil {
			return nil, err
		}
		fallbackDirectories[i] = fallbackDir
	}

	return &FileBackend{
		PrimaryDirectory:    primaryDirectory,
		FallbackDirectories: fallbackDirectories,
	}, nil
}

func (f FileBackend) Get() (string, error) {
	token, err := f.getFrom(f.PrimaryDirectory)

	if err != nil && errors.Is(err, fs.ErrNotExist) {
		for _, dir := range f.FallbackDirectories {
			token, err = f.getFrom(dir)

			if err != nil && errors.Is(err, fs.ErrNotExist) {
				continue
			}

			if err != nil {
				return token, err
			}

			if err := f.Set(token); err != nil {
				return "", errors.Wrapf(err, "unable to migrate access token from %q to %q", dir, f.PrimaryDirectory)
			}

			return token, nil
		}

		// if no fallbacks were used, indicate no token was found
		return "", nil
	}

	return token, err
}

func (f FileBackend) getFrom(dir string) (string, error) {
	path := filepath.Join(dir, "accesstoken")
	fd, err := os.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "unable to open %q", path)
	}
	defer fd.Close()

	contents, err := io.ReadAll(fd)
	if err != nil {
		return "", errors.Wrapf(err, "error reading %q", path)
	}

	return strings.TrimSpace(string(contents)), nil
}

func (f FileBackend) Set(value string) error {
	err := os.MkdirAll(f.PrimaryDirectory, os.ModePerm)
	if err != nil {
		return errors.Wrapf(err, "unable to create %q", f.PrimaryDirectory)
	}

	path := filepath.Join(f.PrimaryDirectory, "accesstoken")
	fd, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "unable to create %q", path)
	}
	defer fd.Close()

	_, err = io.WriteString(fd, value)
	if err != nil {
		return errors.Wrapf(err, "unable to write token to %q", path)
	}

	return nil
}

var tildeSlash = fmt.Sprintf("~%v", string(os.PathSeparator))

func expandTilde(dir string) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(dir, tildeSlash) {
		return filepath.Join(user.HomeDir, strings.TrimPrefix(dir, tildeSlash)), nil
	} else if dir == "~" {
		return user.HomeDir, nil
	} else {
		return dir, nil
	}
}
