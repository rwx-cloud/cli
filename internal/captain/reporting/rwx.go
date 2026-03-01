package reporting

import (
	"encoding/json"

	"github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/fs"
	v1 "github.com/rwx-cloud/cli/internal/captain/testingschema/v1"
)

func WriteJSONSummary(file fs.File, testResults v1.TestResults, _ Configuration) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(testResults); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
