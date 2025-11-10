package cli

import (
	"github.com/rwx-cloud/cli/internal/errors"
)

func ResolveCliParams(yamlContent string) (string, error) {
	doc, err := ParseYAMLDoc(yamlContent)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse YAML")
	}

	if !doc.hasPath("$.on") {
		return "", errors.New("no git init params found in any trigger")
	}

	return yamlContent, nil
}
