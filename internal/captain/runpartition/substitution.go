package runpartition

import (
	"github.com/rwx-cloud/cli/internal/captain/templating"
)

type Substitution interface {
	Example() string
	ValidateTemplate(compiledTemplate templating.CompiledTemplate) error
	SubstitutionLookupFor(
		_ templating.CompiledTemplate,
		testFilePaths []string,
	) (map[string]string, error)
}
