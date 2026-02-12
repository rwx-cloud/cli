package cli

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/versions"
)

const DefaultArch = "x86_64"

var HandledError = errors.New("handled error")
var hasOutputVersionMessage atomic.Bool

// Service holds the main business logic of the CLI.
type Service struct {
	Config
}

func NewService(cfg Config) (Service, error) {
	if err := cfg.Validate(); err != nil {
		return Service{}, errors.Wrap(err, "validation failed")
	}

	svc := Service{cfg}
	svc.outputLatestVersionMessage()
	return svc, nil
}

func (s Service) outputLatestVersionMessage() {
	versions.LoadLatestVersionFromFile(s.VersionsBackend)

	if !versions.NewVersionAvailable() {
		return
	}

	if !hasOutputVersionMessage.CompareAndSwap(false, true) {
		return
	}

	showLatestVersion := os.Getenv("MINT_HIDE_LATEST_VERSION") == "" && os.Getenv("RWX_HIDE_LATEST_VERSION") == ""

	if !showLatestVersion {
		return
	}

	w := s.Stderr
	fmt.Fprintf(w, "A new release of rwx is available: %s → %s\n", versions.GetCliCurrentVersion(), versions.GetCliLatestVersion())

	if versions.InstalledWithHomebrew() {
		fmt.Fprintln(w, "To upgrade, run: brew upgrade rwx-cloud/tap/rwx")
	}

	fmt.Fprintln(w)
}

func Map[T any, R any](input []T, transformer func(T) R) []R {
	result := make([]R, len(input))
	for i, item := range input {
		result[i] = transformer(item)
	}
	return result
}

func tryGetSliceAtIndex[S ~[]E, E any](s S, index int, defaultValue E) E {
	if len(s) <= index {
		return defaultValue
	}
	return s[index]
}
