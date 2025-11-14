package main

import (
	"os"
	"strings"

	"github.com/rwx-cloud/cli/internal/errors"
)

// ParseInitParameters converts a list of `key=value` pairs to a map. It also reads any `MINT_INIT_` variables from the
// environment
func ParseInitParameters(params []string) (map[string]string, error) {
	parsedParams := make(map[string]string)

	parse := func(p string) error {
		fields := strings.Split(p, "=")
		if len(fields) < 2 {
			return errors.Errorf("unable to parse %q", p)
		}

		parsedParams[fields[0]] = strings.Join(fields[1:], "=")
		return nil
	}

	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, "MINT_INIT_") {
			continue
		}

		if err := parse(strings.TrimPrefix(envVar, "MINT_INIT_")); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, "RWX_INIT_") {
			continue
		}

		if err := parse(strings.TrimPrefix(envVar, "RWX_INIT_")); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	// Parse flag parameters after the environment as they take precedence
	for _, param := range params {
		if err := parse(param); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return parsedParams, nil
}
