package adb

import (
	"fmt"

	"github.com/pkg/errors"
)

// TODO(jmh): consider removing most of this sentinel error values and
// provide useful error interfaces. Explicitly document error handling
// possibilities on individual functions.

// Sentinel error values used by this package
var (
	// The connection to the server was reset in the middle of an operation. Server probably died.
	ErrConnectionReset = errors.New("connection reset")
	// Tried to perform an operation on a path that doesn't exist on the device.
	ErrFileNotExist   = errors.New("file does not exist")
	ErrNotImplemented = errors.New("not implemented")
)

type UnexpectedStatusError struct {
	want []string
	got  string
}

func (us *UnexpectedStatusError) Error() string {
	return fmt.Sprintf("want one of %v, got %s", us.want, us.got)
}

// Rename CmdError?
type ShellExitError struct {
	Command  string
	ExitCode int
}

func (s ShellExitError) Error() string {
	return fmt.Sprintf("shell %q exit code %d", s.Command, s.ExitCode)
}
