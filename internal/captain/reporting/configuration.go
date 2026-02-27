package reporting

import "github.com/rwx-cloud/cli/internal/captain/providers"

type Configuration struct {
	CloudEnabled          bool
	CloudHost             string
	CloudOrganizationSlug string
	SuiteID               string
	RetryCommandTemplate  string
	Provider              providers.Provider
}
