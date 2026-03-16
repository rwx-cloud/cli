package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/rwx-cloud/rwx/internal/errors"
	"github.com/rwx-cloud/rwx/internal/versions"
)

const DefaultArch = "x86_64"

var HandledError = errors.New("handled error")

// ExitCodeError signals that the process should exit with a specific code.
// Commands return this instead of calling os.Exit directly so that main()
// can flush telemetry before exiting.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

func (e *ExitCodeError) Is(target error) bool {
	_, ok := target.(*ExitCodeError)
	return ok
}

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

// recordTelemetry enqueues a telemetry event if a collector is configured.
func (s Service) recordTelemetry(event string, props map[string]any) {
	if s.TelemetryCollector == nil {
		return
	}
	s.TelemetryCollector.Record(event, props)
}

// confirmDestruction prompts the user to confirm a destructive action.
// If yes is true, confirmation is skipped. In non-TTY environments without
// yes, an error is returned instructing the user to pass --yes.
func (s Service) confirmDestruction(prompt string, yes bool) error {
	if yes {
		return nil
	}

	if !s.StderrIsTTY {
		return errors.New("use --yes to confirm in non-interactive environments")
	}

	fmt.Fprintf(s.Stderr, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(s.Stdin)
	if !scanner.Scan() {
		return errors.New("no input provided")
	}

	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return errors.New("aborted")
	}

	return nil
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
