package cli

import (
	"io"

	"github.com/rwx-cloud/cli/internal/docker"
	"github.com/rwx-cloud/cli/internal/errors"
)

type Config struct {
	APIClient   APIClient
	SSHClient   SSHClient
	GitClient   GitClient
	DockerCLI   docker.Client
	Stdout      io.Writer
	StdoutIsTTY bool
	Stderr      io.Writer
	StderrIsTTY bool
}

func (c Config) Validate() error {
	if c.APIClient == nil {
		return errors.New("missing RWX client")
	}

	if c.SSHClient == nil {
		return errors.New("missing SSH client constructor")
	}

	if c.GitClient == nil {
		return errors.New("missing Git client constructor")
	}

	if c.DockerCLI == nil {
		return errors.New("missing Docker client")
	}

	if c.Stdout == nil {
		return errors.New("missing Stdout")
	}

	if c.Stderr == nil {
		return errors.New("missing Stderr")
	}

	return nil
}
