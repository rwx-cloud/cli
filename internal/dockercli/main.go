package dockercli

import (
	"fmt"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/flags"
)

type DockerConfigurator interface {
	ConfigFile() *configfile.ConfigFile
}

func New() (DockerConfigurator, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("unable to make a new docker CLI: %w", err)
	}

	if err := cli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, fmt.Errorf("unable to initialize the docker CLI: %w", err)
	}

	return cli, nil
}
