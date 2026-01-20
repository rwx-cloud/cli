package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
)

type WhoamiConfig struct {
	Json bool
}

func (c WhoamiConfig) Validate() error {
	return nil
}

func (s Service) Whoami(cfg WhoamiConfig) (*api.WhoamiResult, error) {
	result, err := s.APIClient.Whoami()
	s.outputLatestVersionMessage()
	if err != nil {
		return nil, errors.Wrap(err, "unable to determine details about the access token")
	}

	if cfg.Json {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "unable to JSON encode the result")
		}

		fmt.Fprint(s.Stdout, string(encoded))
	} else {
		fmt.Fprintf(s.Stdout, "Token Kind: %v\n", strings.ReplaceAll(result.TokenKind, "_", " "))
		fmt.Fprintf(s.Stdout, "Organization: %v\n", result.OrganizationSlug)
		if result.UserEmail != nil {
			fmt.Fprintf(s.Stdout, "User: %v\n", *result.UserEmail)
		}
	}

	return result, nil
}
