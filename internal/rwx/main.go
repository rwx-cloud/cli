package rwx

import (
	"fmt"
	"os"
	"path/filepath"
)

func IsRWX() bool {
	return os.Getenv("RWX") == "true"
}

func WriteValue(key string, value string) error {
	values := os.Getenv("RWX_VALUES")
	if err := os.WriteFile(filepath.Join(values, key), []byte(value), 0644); err != nil {
		return fmt.Errorf("unable to write RWX value %q: %w", key, err)
	}
	return nil
}
