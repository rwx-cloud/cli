package parsing

import (
	"encoding/json"
	"io"

	"github.com/rwx-cloud/cli/internal/captain/errors"
	v1 "github.com/rwx-cloud/cli/internal/captain/testingschema/v1"
)

type RWXParser struct{}

func (p RWXParser) Parse(data io.Reader) (*v1.TestResults, error) {
	var testResults v1.TestResults

	if err := json.NewDecoder(data).Decode(&testResults); err != nil {
		return nil, errors.NewInputError("Unable to parse test results as JSON: %s", err)
	}

	return &testResults, nil
}
