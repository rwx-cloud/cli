package mocks

import (
	"io"

	"github.com/rwx-cloud/cli/internal/errors"

	"golang.org/x/crypto/ssh"
)

type SSH struct {
	MockConnect                          func(addr string, cfg ssh.ClientConfig) error
	MockConnectWithKey                   func(addr string, cfg ssh.ClientConfig, privateKeyPEM string) error
	MockInteractiveSession               func() error
	MockExecuteCommand                   func(command string) (int, error)
	MockExecuteCommandWithStdin          func(command string, stdin io.Reader) (int, error)
	MockExecuteCommandWithOutput         func(command string) (int, string, error)
	MockExecuteCommandWithStdinAndStdout func(command string, stdin io.Reader, stdout io.Writer) (int, error)
	MockGetConnectionDetails             func() (address string, user string, privateKeyPEM string)
}

func (s *SSH) Close() error {
	return nil
}

func (s *SSH) Connect(addr string, cfg ssh.ClientConfig) error {
	if s.MockConnect != nil {
		return s.MockConnect(addr, cfg)
	}

	return errors.New("MockConnect was not configured")
}

func (s *SSH) ConnectWithKey(addr string, cfg ssh.ClientConfig, privateKeyPEM string) error {
	if s.MockConnectWithKey != nil {
		return s.MockConnectWithKey(addr, cfg, privateKeyPEM)
	}

	return errors.New("MockConnectWithKey was not configured")
}

func (s *SSH) InteractiveSession() error {
	if s.MockInteractiveSession != nil {
		return s.MockInteractiveSession()
	}

	return errors.New("MockInteractiveSession was not configured")
}

func (s *SSH) ExecuteCommand(command string) (int, error) {
	if s.MockExecuteCommand != nil {
		return s.MockExecuteCommand(command)
	}

	return -1, errors.New("MockExecuteCommand was not configured")
}

func (s *SSH) ExecuteCommandWithStdin(command string, stdin io.Reader) (int, error) {
	if s.MockExecuteCommandWithStdin != nil {
		return s.MockExecuteCommandWithStdin(command, stdin)
	}

	return -1, errors.New("MockExecuteCommandWithStdin was not configured")
}

func (s *SSH) ExecuteCommandWithOutput(command string) (int, string, error) {
	if s.MockExecuteCommandWithOutput != nil {
		return s.MockExecuteCommandWithOutput(command)
	}

	return -1, "", errors.New("MockExecuteCommandWithOutput was not configured")
}

func (s *SSH) ExecuteCommandWithStdinAndStdout(command string, stdin io.Reader, stdout io.Writer) (int, error) {
	if s.MockExecuteCommandWithStdinAndStdout != nil {
		return s.MockExecuteCommandWithStdinAndStdout(command, stdin, stdout)
	}

	return -1, errors.New("MockExecuteCommandWithStdinAndStdout was not configured")
}

func (s *SSH) GetConnectionDetails() (address string, user string, privateKeyPEM string) {
	if s.MockGetConnectionDetails != nil {
		return s.MockGetConnectionDetails()
	}

	return "", "", ""
}
