package cli

import (
	"encoding/json"
	"fmt"

	"github.com/rwx-cloud/rwx/internal/api"
	"github.com/rwx-cloud/rwx/internal/errors"
)

type CreateVaultOidcTokenConfig struct {
	Vault    string
	Name     string
	Audience string
	Provider string
	Json     bool
}

type CreateVaultOidcTokenResult struct {
	Audience         string
	Subject          string
	Expression       string
	DocumentationURL string
}

func (s Service) CreateVaultOidcToken(cfg CreateVaultOidcTokenConfig) (*CreateVaultOidcTokenResult, error) {
	if cfg.Provider == "" {
		if cfg.Name == "" {
			return nil, errors.New("--name is required (or use --provider)")
		}
		if cfg.Audience == "" {
			return nil, errors.New("--audience is required (or use --provider)")
		}
	}

	apiResult, err := s.APIClient.CreateVaultOidcToken(api.CreateVaultOidcTokenConfig{
		VaultName: cfg.Vault,
		Name:      cfg.Name,
		Audience:  cfg.Audience,
		Provider:  cfg.Provider,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to create OIDC token")
	}

	result := &CreateVaultOidcTokenResult{
		Audience:         apiResult.Audience,
		Subject:          apiResult.Subject,
		Expression:       apiResult.Expression,
		DocumentationURL: apiResult.DocumentationURL,
	}

	if cfg.Json {
		output := struct {
			Audience         string
			Subject          string
			Expression       string
			DocumentationURL string
		}{
			Audience:         result.Audience,
			Subject:          result.Subject,
			Expression:       result.Expression,
			DocumentationURL: result.DocumentationURL,
		}
		if err := json.NewEncoder(s.Stdout).Encode(output); err != nil {
			return nil, errors.Wrap(err, "unable to encode JSON output")
		}
	} else {
		name := cfg.Name
		if name == "" {
			// Extract name from the expression returned by the API
			name = cfg.Provider
		}
		fmt.Fprintf(s.Stdout, "Created OIDC token %q in vault %q.\n", name, cfg.Vault)
		fmt.Fprintf(s.Stdout, "\nAudience:   %s\n", result.Audience)
		fmt.Fprintf(s.Stdout, "Subject:    %s\n", result.Subject)
		fmt.Fprintf(s.Stdout, "Expression: %s\n", result.Expression)
		fmt.Fprintf(s.Stdout, "\nFor more information on configuring your identity provider and using your OIDC token, see: %s\n", result.DocumentationURL)
	}

	return result, nil
}
