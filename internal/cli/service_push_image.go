package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/distribution/reference"
	"github.com/rwx-cloud/cli/internal/api"
)

type PushImageOutput struct {
	PushID string `json:"push_id,omitempty"`
	RunURL string `json:"run_url,omitempty"`
	Status string `json:"status,omitempty"`
}

func (s Service) PushImage(config PushImageConfig) error {
	request := api.StartImagePushConfig{
		TaskID:      config.TaskID,
		Image:       api.StartImagePushConfigImage{},
		Credentials: api.StartImagePushConfigCredentials{},
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

	request.Credentials.Username = os.Getenv("RWX_PUSH_USERNAME")
	request.Credentials.Password = os.Getenv("RWX_PUSH_PASSWORD")

	if request.Credentials.Username == "" && request.Credentials.Password != "" {
		return fmt.Errorf("RWX_PUSH_USERNAME must be set if RWX_PUSH_PASSWORD is set")
	} else if request.Credentials.Username != "" && request.Credentials.Password == "" {
		return fmt.Errorf("RWX_PUSH_PASSWORD must be set if RWX_PUSH_USERNAME is set")
	} else if request.Credentials.Username == "" && request.Credentials.Password == "" {
		credentialsHost := request.Image.Registry
		if credentialsHost == "registry.hub.docker.com/v2" {
			credentialsHost = "index.docker.io"
		}

		credentials, err := config.DockerCLI.GetAuthConfig(credentialsHost)
		if err != nil {
			return fmt.Errorf("unable to get credentials for registry %q from docker: %w", request.Image.Registry, err)
		}
		if credentials.Username == "" || credentials.Password == "" {
			return fmt.Errorf("no credentials found for registry %q in docker config", request.Image.Registry)
		}

		request.Credentials.Username = credentials.Username
		request.Credentials.Password = credentials.Password
	}

	stopStartSpinner := func() {}
	if !config.JSON {
		stopStartSpinner = spin(
			fmt.Sprintf("Starting image push of task %q to '%s/%s' with tags: %s...", request.TaskID, request.Image.Registry, request.Image.Repository, strings.Join(request.Image.Tags, ", ")),
			s.StderrIsTTY,
			s.Stderr,
		)
	}

	result, err := s.APIClient.StartImagePush(request)
	stopStartSpinner()
	if err != nil {
		return err
	}

	if err := config.OpenURL(result.RunURL); err != nil {
		fmt.Fprintf(s.Stderr, "Warning: unable to open the run in your browser. You can manually visit the run at %q.\n", result.RunURL)
	}

	if !config.Wait {
		if config.JSON {
			output := PushImageOutput{PushID: result.PushID, RunURL: result.RunURL}
			if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
				return fmt.Errorf("unable to encode output: %w", err)
			}
			return nil
		} else {
			fmt.Fprintln(s.Stdout, "Your image is being pushed. This may take some time for large images.")
			fmt.Fprintf(s.Stdout, "To follow along, you can watch the run at %q.\n", result.RunURL)
			fmt.Fprintln(s.Stdout)
			return nil
		}
	}

	stopWaitingSpinner := func() {}
	if !config.JSON {
		stopWaitingSpinner = spin(
			"Waiting for image push to finish...",
			s.StderrIsTTY,
			s.Stderr,
		)
	}

	pollInterval := config.PollInterval
	if pollInterval == 0*time.Second {
		pollInterval = 1 * time.Millisecond
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	var finalPushResult api.ImagePushStatusResult
statusloop:
	for range ticker.C {
		result, err := s.APIClient.ImagePushStatus(result.PushID)
		if err != nil {
			stopWaitingSpinner()
			return fmt.Errorf("unable to get image push status: %w", err)
		}

		switch result.Status {
		case "succeeded", "failed":
			finalPushResult = result
			stopWaitingSpinner()
			break statusloop
		case "in_progress", "waiting":
			// continue waiting
		default:
			stopWaitingSpinner()
			return fmt.Errorf("unknown image push status: %q", result.Status)
		}
	}

	switch finalPushResult.Status {
	case "succeeded":
		{
			if config.JSON {
				output := PushImageOutput{PushID: result.PushID, RunURL: result.RunURL, Status: finalPushResult.Status}
				if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
					return fmt.Errorf("unable to encode output: %w", err)
				}
			} else {
				fmt.Fprintf(s.Stdout, "Image push succeeded! You can pull your image from '%s/%s' with tags: %s\n", request.Image.Registry, request.Image.Repository, strings.Join(request.Image.Tags, ", "))
				return nil
			}
		}
	case "failed":
		{
			if config.JSON {
				output := PushImageOutput{PushID: result.PushID, RunURL: result.RunURL, Status: finalPushResult.Status}
				if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
					return fmt.Errorf("unable to encode output: %w", err)
				}
			}
			return fmt.Errorf("image push failed, inspect the run at %q to see why", result.RunURL)
		}
	default:
		return fmt.Errorf("unknown image push status: %q", finalPushResult.Status)
	}

	return nil
}

func spin(message string, tty bool, out io.Writer) func() {
	if tty {
		indicator := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(out))
		indicator.Suffix = " " + message
		indicator.Start()
		return indicator.Stop
	} else {
		ticker := time.NewTicker(1 * time.Second)
		fmt.Fprintln(out, message)
		go func() {
			for range ticker.C {
				fmt.Fprintf(out, ".")
			}
		}()

		return func() {
			ticker.Stop()
			fmt.Fprintln(out)
		}
	}
}
