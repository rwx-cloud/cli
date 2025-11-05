package cli

import (
	"fmt"
	"maps"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/rwx-cloud/cli/internal/errors"
)

type TransmogrifyDockerfileOutput struct {
	PushID string `json:"push_id,omitempty"`
	RunURL string `json:"run_url,omitempty"`
	Status string `json:"status,omitempty"`
}

func (s Service) TransmogrifyDockerfile(config TransmogrifyDockerfileConfig) error {
	res, err := parser.Parse(config.Dockerfile)
	if err != nil {
		return fmt.Errorf("unable to parse Dockerfile: %w", err)
	}

	// TODO(kkt): meta args? what are they?
	stages, _, err := instructions.Parse(res.AST, nil)
	if err != nil {
		return fmt.Errorf("unable to interpret Dockerfile: %w", err)
	}

	// TODO(kkt): multi-stage support
	// if len(stages) != 1 {
	// 	return fmt.Errorf("only single-stage Dockerfiles are supported")
	// }

	// TODO(kkt): how do we know which architecture?
	// TODO(kkt): how do we know which config to apply?

	stage := stages[0]
	builder := NewRunDefinitionBuilder(res.EscapeToken)

	for _, cmd := range stage.Commands {
		if err := builder.Dispatch(cmd); err != nil {
			return err
		}
	}

	yaml.NewEncoder(os.Stdout).Encode(builder.tasks)

	return nil
}

type TransmogrifiedTaskDefinition struct {
	Key string            `yaml:"key"`
	Use []string          `yaml:"use,omitempty"`
	Run string            `yaml:"run"`
	Env map[string]string `yaml:"env,omitempty"`
}

func NewTransmogrifiedTaskDefinition(key string) TransmogrifiedTaskDefinition {
	return TransmogrifiedTaskDefinition{
		Key: key,
		Use: []string{},
	}
}

type RunDefinitionBuilder struct {
	workdir  string
	platform string
	shlex    *shell.Lex
	init     map[string]string
	env      map[string]string
	tasks    []TransmogrifiedTaskDefinition
	filter   []string
}

const workspace = "/var/mint-workspace"

func NewRunDefinitionBuilder(escapeToken rune) *RunDefinitionBuilder {
	return &RunDefinitionBuilder{
		workdir:  "/",
		platform: "linux",
		shlex:    shell.NewLex(escapeToken),
		init:     map[string]string{},
		env:      map[string]string{},
		tasks:    []TransmogrifiedTaskDefinition{},
		filter:   []string{},
	}
}

func (b *RunDefinitionBuilder) Dispatch(cmd instructions.Command) error {
	if c, ok := cmd.(instructions.PlatformSpecific); ok {
		err := c.CheckPlatform(b.platform)
		if err != nil {
			return fmt.Errorf("platform not supported: %w", err)
		}
	}

	// TODO(kkt): properly handle expansion (well, not exactly. replace with `init`? defaults on ENV go into init cli trigger?)

	// runConfigEnv := d.state.runConfig.Env
	// envs := shell.EnvsFromSlice(append(runConfigEnv, d.state.buildArgs.FilterAllowed(runConfigEnv)...))
	// if ex, ok := cmd.(instructions.SupportsSingleWordExpansion); ok {
	// 	err := ex.Expand(func(word string) (string, error) {
	// 		newword, _, err := b.shlex.ProcessWord(word, envs)
	// 		return newword, err
	// 	})
	// 	if err != nil {
	// 		return errdefs.InvalidParameter(err)
	// 	}
	// }

	switch c := cmd.(type) {
	case *instructions.EnvCommand:
		return b.dispatchEnv(c)
	case *instructions.MaintainerCommand:
		return b.dispatchMaintainer(c)
	case *instructions.LabelCommand:
		return b.dispatchLabel(c)
	case *instructions.AddCommand:
		return b.dispatchAdd(c)
	case *instructions.CopyCommand:
		return b.dispatchCopy(c)
	case *instructions.OnbuildCommand:
		return b.dispatchOnbuild(c)
	case *instructions.WorkdirCommand:
		return b.dispatchWorkdir(c)
	case *instructions.RunCommand:
		return b.dispatchRun(c)
	case *instructions.CmdCommand:
		return b.dispatchCmd(c)
	case *instructions.HealthCheckCommand:
		return b.dispatchHealthCheck(c)
	case *instructions.EntrypointCommand:
		return b.dispatchEntrypoint(c)
	case *instructions.ExposeCommand:
		return b.dispatchExpose(c)
	case *instructions.UserCommand:
		return b.dispatchUser(c)
	case *instructions.VolumeCommand:
		return b.dispatchVolume(c)
	case *instructions.StopSignalCommand:
		return b.dispatchStopSignal(c)
	case *instructions.ArgCommand:
		return b.dispatchArg(c)
	case *instructions.ShellCommand:
		return b.dispatchShell(c)
	}
	return errors.Errorf("unsupported command type: %v", reflect.TypeOf(cmd))
}

var unsupportedError = errors.New("instruction not supported by RWX")

func (b *RunDefinitionBuilder) dispatchEnv(cmd *instructions.EnvCommand) error {
	for _, kv := range cmd.Env {
		b.env[kv.Key] = kv.Value
	}

	return nil
}

func (b *RunDefinitionBuilder) dispatchMaintainer(_ *instructions.MaintainerCommand) error {
	return fmt.Errorf("MAINTAINER: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchLabel(_ *instructions.LabelCommand) error {
	return fmt.Errorf("LABEL: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchAdd(_ *instructions.AddCommand) error {
	return nil
}

func (b *RunDefinitionBuilder) dispatchCopy(_ *instructions.CopyCommand) error {
	return nil
}

func (b *RunDefinitionBuilder) dispatchOnbuild(_ *instructions.OnbuildCommand) error {
	return fmt.Errorf("ONBUILD: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchWorkdir(cmd *instructions.WorkdirCommand) error {
	b.workdir = cmd.Path
	return nil
}

func (b *RunDefinitionBuilder) dispatchRun(cmd *instructions.RunCommand) error {
	task := NewTransmogrifiedTaskDefinition(strconv.Itoa(len(b.tasks) + 1))
	if len(b.tasks) > 0 {
		task.Use = append(task.Use, strconv.Itoa(len(b.tasks)))
	}
	task.Env = maps.Clone(b.env)

	if b.workdir != "/" {
		task.Run += "cd " + path.Join(".", b.workdir) + "\n"
	}
	task.Run += strings.Join(cmd.CmdLine, " ")

	b.tasks = append(b.tasks, task)
	return nil
}

func (b *RunDefinitionBuilder) dispatchCmd(cmd *instructions.CmdCommand) error {
	task := NewTransmogrifiedTaskDefinition(strconv.Itoa(len(b.tasks) + 1))
	if len(b.tasks) > 0 {
		task.Use = append(task.Use, strconv.Itoa(len(b.tasks)))
	}
	task.Env = maps.Clone(b.env)
	// TODO(kkt): handle escaping
	task.Run = fmt.Sprintf("echo \"%v\" > $RWX_IMAGE/command", strings.Join(cmd.CmdLine, " "))

	b.tasks = append(b.tasks, task)
	return nil
}

func (b *RunDefinitionBuilder) dispatchHealthCheck(_ *instructions.HealthCheckCommand) error {
	return fmt.Errorf("HEALTHCHECK: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchEntrypoint(cmd *instructions.EntrypointCommand) error {
	task := NewTransmogrifiedTaskDefinition(strconv.Itoa(len(b.tasks) + 1))
	if len(b.tasks) > 0 {
		task.Use = append(task.Use, strconv.Itoa(len(b.tasks)))
	}
	task.Env = maps.Clone(b.env)
	// TODO(kkt): handle escaping
	task.Run = fmt.Sprintf("echo \"%v\" > $RWX_IMAGE/entrypoint", strings.Join(cmd.CmdLine, " "))

	b.tasks = append(b.tasks, task)
	return nil
}

func (b *RunDefinitionBuilder) dispatchExpose(_ *instructions.ExposeCommand) error {
	return fmt.Errorf("EXPOSE: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchUser(cmd *instructions.UserCommand) error {
	task := NewTransmogrifiedTaskDefinition(strconv.Itoa(len(b.tasks) + 1))
	if len(b.tasks) > 0 {
		task.Use = append(task.Use, strconv.Itoa(len(b.tasks)))
	}
	task.Env = maps.Clone(b.env)
	task.Run = fmt.Sprintf("echo \"%v\" > $RWX_IMAGE/user", cmd.User)

	b.tasks = append(b.tasks, task)
	return nil
}

func (b *RunDefinitionBuilder) dispatchVolume(_ *instructions.VolumeCommand) error {
	return fmt.Errorf("VOLUME: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchStopSignal(_ *instructions.StopSignalCommand) error {
	return fmt.Errorf("STOPSIGNAL: %w", unsupportedError)
}

func (b *RunDefinitionBuilder) dispatchArg(_ *instructions.ArgCommand) error {
	return nil
}

func (b *RunDefinitionBuilder) dispatchShell(cmd *instructions.ShellCommand) error {
	task := NewTransmogrifiedTaskDefinition(strconv.Itoa(len(b.tasks) + 1))
	if len(b.tasks) > 0 {
		task.Use = append(task.Use, strconv.Itoa(len(b.tasks)))
	}
	task.Env = maps.Clone(b.env)
	// TODO(kkt): handle escaping
	task.Run = fmt.Sprintf("echo \"%v\" > $RWX_IMAGE/shell", strings.Join(cmd.Shell, " "))

	b.tasks = append(b.tasks, task)
	return nil
}
