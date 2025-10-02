package dockercli

import (
	"fmt"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/flags"
)

type AuthConfigurator interface {
	GetAuthConfig(string) (types.AuthConfig, error)
}

type realDockerCLI struct {
	*command.DockerCli
}

func (r realDockerCLI) GetAuthConfig(registry string) (types.AuthConfig, error) {
	return r.ConfigFile().GetAuthConfig(registry)
}

func New() (AuthConfigurator, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("unable to make a new docker CLI: %w", err)
	}

	if err := cli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, fmt.Errorf("unable to initialize the docker CLI: %w", err)
	}

	return realDockerCLI{cli}, nil
}
