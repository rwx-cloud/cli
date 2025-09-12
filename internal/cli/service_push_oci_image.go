package cli

import (
	"fmt"

	"github.com/distribution/reference"
	"github.com/rwx-cloud/cli/internal/api"
)

func (s Service) PushOCIImage(config PushOCIImageConfig) error {
	request := api.StartOCIImagePushConfig{
		TaskID:      config.TaskID,
		Image:       api.StartOCIImagePushConfigImage{},
		Credentials: api.StartOCIImagePushConfigCredentials{},
	}

	for _, ref := range config.References {
		registry := reference.Domain(ref)
		if registry == "docker.io" {
			registry = "registry.hub.docker.com/v2"
		}

		repository := reference.Path(ref)

		tag := "latest"
		if tagged, ok := ref.(reference.Tagged); ok {
			tag = tagged.Tag()
		}

		if request.Image.Registry == "" {
			request.Image.Registry = registry
		} else if request.Image.Registry != registry {
			return fmt.Errorf("all image references must have the same registry: %v != %v", request.Image.Registry, registry)
		}

		if request.Image.Repository == "" {
			request.Image.Repository = repository
		} else if request.Image.Repository != repository {
			return fmt.Errorf("all image references must have the same repository: %v != %v", request.Image.Repository, repository)
		}

		request.Image.Tags = append(request.Image.Tags, tag)
	}

	credentialsHost := request.Image.Registry
	if credentialsHost == "registry.hub.docker.com/v2" {
		credentialsHost = "index.docker.io"
	}

	credentials, err := config.DockerCLI.ConfigFile().GetAuthConfig(credentialsHost)
	if err != nil {
		return fmt.Errorf("unable to get credentials for registry %q from docker: %w", request.Image.Registry, err)
	}
	if credentials.Username == "" || credentials.Password == "" {
		return fmt.Errorf("no credentials found for registry %q in docker config", request.Image.Registry)
	}

	request.Credentials.Username = credentials.Username
	request.Credentials.Password = credentials.Password

	result, err := s.APIClient.StartOCIImagePush(request)
	if err != nil {
		return err
	}

	fmt.Printf("%v\n", result.PushID)
	return nil
}
