package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/providers"
)

// testConfig is the internal representation of the captain configuration.
type testConfig struct {
	captaincli.ConfigFile

	ProvidersEnv providers.Env

	Secrets struct {
		APIToken string
	}
}

type testContextKey string

var testConfigKey = testContextKey("captainConfig")

func getTestConfig(cmd *cobra.Command) (testConfig, error) {
	val := cmd.Context().Value(testConfigKey)
	if val == nil {
		return testConfig{}, captainerrors.NewInternalError(
			"Tried to fetch config from the command but it wasn't set. This should never happen!")
	}

	cfg, ok := val.(testConfig)
	if !ok {
		return testConfig{}, captainerrors.NewInternalError(
			"Tried to fetch config from the command but it was of the wrong type. This should never happen!")
	}

	return cfg, nil
}

func setTestConfigContext(cmd *cobra.Command, cfg testConfig) error {
	if _, err := getTestConfig(cmd); err == nil {
		return captainerrors.NewInternalError("Tried to set config on the command but it was already set. This should never happen!")
	}

	ctx := context.WithValue(cmd.Context(), testConfigKey, cfg)
	cmd.SetContext(ctx)
	return nil
}

var captainDirectory = filepath.Join(".rwx", "test")

const (
	captainConfigName   = "config"
	flakesFileName      = "flakes.yaml"
	quarantinesFileName = "quarantines.yaml"
	timingsFileName     = "timings.yaml"
)

var captainConfigExtensions = []string{"yaml", "yml"}

func findInParentDir(fileName string) (string, error) {
	var match string
	var walk func(string, string) error

	walk = func(base, root string) error {
		if base == root {
			return captainerrors.WithStack(os.ErrNotExist)
		}

		match = filepath.Join(base, fileName)

		info, err := os.Stat(match)
		if err != nil && !captainerrors.Is(err, os.ErrNotExist) {
			return captainerrors.WithStack(err)
		}

		if info != nil {
			return nil
		}

		return walk(filepath.Dir(base), root)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return "", captainerrors.WithStack(err)
	}

	volumeName := filepath.VolumeName(pwd)
	if volumeName == "" {
		volumeName = string(os.PathSeparator)
	}

	if err := walk(pwd, volumeName); err != nil {
		return "", captainerrors.WithStack(err)
	}

	return match, nil
}

func initTestConfig(cmd *cobra.Command, cliArgs testCliArgs) (cfg testConfig, err error) {
	if cliArgs.rootCliArgs.configFilePath == "" {
		possibleConfigFilePaths := make([]string, 0, 2)
		errs := make([]error, 0, 2)

		for _, extension := range captainConfigExtensions {
			configFilePath, err := findInParentDir(
				filepath.Join(captainDirectory, fmt.Sprintf("%s.%s", captainConfigName, extension)),
			)

			if err == nil {
				possibleConfigFilePaths = append(possibleConfigFilePaths, configFilePath)
			} else {
				errs = append(errs, err)
			}
		}

		if len(possibleConfigFilePaths) > 1 {
			return cfg, captainerrors.NewConfigurationError(
				"Unable to identify configuration file",
				fmt.Sprintf(
					"rwx test found multiple configuration files in your environment: %s\n",
					strings.Join(possibleConfigFilePaths, ", "),
				),
				"Please make sure only one config file is present in your environment or explicitly specify "+
					"one using the '--config-file' flag.",
			)
		}

		if len(possibleConfigFilePaths) == 0 {
			for _, err := range errs {
				if err != nil && !captainerrors.Is(err, os.ErrNotExist) {
					return cfg, captainerrors.NewConfigurationError(
						"Unable to read configuration file",
						fmt.Sprintf(
							"The following system error occurred while searching for a config file: %s",
							err.Error(),
						),
						"Please make sure that rwx test has the correct permissions to access the config file, "+
							"or explicitly specify one using the '--config-file' flag.",
					)
				}
			}
		} else {
			cliArgs.rootCliArgs.configFilePath = possibleConfigFilePaths[0]
		}
	}

	if cliArgs.rootCliArgs.configFilePath != "" {
		fd, err := os.Open(cliArgs.rootCliArgs.configFilePath)
		if err != nil {
			if !captainerrors.Is(err, os.ErrNotExist) {
				return cfg, captainerrors.Wrap(err, fmt.Sprintf("unable to open config file %q", cliArgs.rootCliArgs.configFilePath))
			}
		} else {
			defer fd.Close()
			decoder := yaml.NewDecoder(fd)
			decoder.KnownFields(true)
			if err = decoder.Decode(&cfg.ConfigFile); err != nil {
				typeError := new(yaml.TypeError)
				if captainerrors.As(err, &typeError) {
					err = captainerrors.NewConfigurationError(
						"Parsing Error",
						strings.Join(typeError.Errors, "\n"),
						"Please refer to the documentation at https://www.rwx.com/docs/captain/cli-configuration/config-yaml for the"+
							" correct config file syntax.",
					)
				}

				return cfg, captainerrors.Wrap(err, "unable to parse config file")
			}
		}
	}

	for name, value := range cfg.Flags {
		if err := cmd.Flags().Set(name, fmt.Sprintf("%v", value)); err != nil {
			return cfg, captainerrors.Wrap(err, fmt.Sprintf("unable to set flag %q", name))
		}
	}

	// Resolve the access token through the file backend so `rwx login` tokens work
	accessTokenBackend, err := initAccessTokenBackend()
	if err != nil {
		return cfg, captainerrors.Wrap(err, "unable to initialize access token backend")
	}
	token, err := accesstoken.Get(accessTokenBackend, AccessToken)
	if err != nil {
		return cfg, captainerrors.Wrap(err, "unable to resolve access token")
	}
	cfg.Secrets.APIToken = token

	if _, ok := cfg.TestSuites[cliArgs.rootCliArgs.suiteID]; !ok {
		if cfg.TestSuites == nil {
			cfg.TestSuites = make(map[string]captaincli.SuiteConfig)
		}

		cfg.TestSuites[cliArgs.rootCliArgs.suiteID] = captaincli.SuiteConfig{}
	}

	cfg = bindTestRootFlags(cfg, cliArgs.rootCliArgs)
	cfg = bindTestFrameworkFlags(cfg, cliArgs.frameworkParams, cliArgs.rootCliArgs.suiteID)
	cfg = bindTestRunFlags(cfg, cliArgs, cmd)

	if err = setTestConfigContext(cmd, cfg); err != nil {
		return cfg, captainerrors.WithStack(err)
	}

	return cfg, nil
}
