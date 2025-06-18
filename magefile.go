//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default is the default build target.
var Default = Build

// All cleans output, builds, tests, and lints.
func All(ctx context.Context) error {
	type target func(context.Context) error

	targets := []target{
		Clean,
		Build,
		Test,
		Lint,
		LintFix,
	}

	for _, t := range targets {
		if err := t(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Build builds the Mint-CLI
func Build(ctx context.Context) error {
	args := []string{"./cmd/mint"}

	ldflags, err := getLdflags()
	if err != nil {
		return err
	}
	args = append([]string{"-ldflags", ldflags}, args...)

	if cgo_enabled := os.Getenv("CGO_ENABLED"); cgo_enabled == "0" {
		args = append([]string{"-a"}, args...)
	}

	return sh.RunV("go", append([]string{"build"}, args...)...)
}

// Clean removes any generated artifacts from the repository.
func Clean(ctx context.Context) error {
	return sh.Rm("./mint")
}

// Lint runs the linter & performs static-analysis checks.
func Lint(ctx context.Context) error {
	return sh.RunV("golangci-lint", "run", "./...")
}

// Applies lint checks and fixes any issues.
func LintFix(ctx context.Context) error {
	if err := sh.RunV("golangci-lint", "run", "--fix", "./..."); err != nil {
		return err
	}

	if err := sh.RunV("go", "mod", "tidy"); err != nil {
		return err
	}

	return nil
}

func UnitTest(ctx context.Context) error {
	// Run unit tests with standard go test
	return (makeTestTask("./internal/...", "./cmd/..."))(ctx)
}

func IntegrationTest(ctx context.Context) error {
	mg.Deps(Build)
	return (makeTestTask("./test/..."))(ctx)
}

func Test(ctx context.Context) error {
	mg.Deps(UnitTest)
	mg.Deps(IntegrationTest)
	return nil
}

func makeTestTask(args ...string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ldflags, err := getLdflags()
		if err != nil {
			return err
		}

		testArgs := []string{"test", "-ldflags", ldflags}

		// Add parallel execution by default
		testArgs = append(testArgs, "-parallel", "4")

		if report := os.Getenv("REPORT"); report != "" {
			// For JUnit reports, we might need to add a test reporter
			// For now, just run tests with verbose output
			testArgs = append(testArgs, "-v")
		}

		testArgs = append(testArgs, args...)

		return sh.RunV("go", testArgs...)
	}
}

func getLdflags() (string, error) {
	if ldflags := os.Getenv("LDFLAGS"); ldflags != "" {
		return ldflags, nil
	}

	sha, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("-X github.com/rwx-research/mint-cli/cmd/mint/config.Version=git-%v", string(sha)), nil
}
