package errors

import (
	"os"

	"github.com/pkg/errors"
)

var (
	ErrFileNotExists   = os.ErrNotExist
	ErrBadRequest      = errors.New("bad request")
	ErrNotFound        = errors.New("not found")
	ErrGone            = errors.New("gone")
	ErrRetry           = errors.New("retry")
	ErrSandboxNoGitDir = errors.New("no .git directory found in sandbox. Set 'preserve-git-dir: true' on your git/clone task")

	As        = errors.As
	Errorf    = errors.Errorf
	Is        = errors.Is
	New       = errors.New
	WithStack = errors.WithStack
	Wrap      = errors.Wrap
	Wrapf     = errors.Wrapf
)
